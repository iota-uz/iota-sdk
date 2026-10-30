package shell

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	RunsPath         = "/v1/runs"
	maxResponseBytes = 128 << 20
)

// defaultExecuteTimeout bounds a call without a deadline; it covers the default
// worker queue draining at maximum run time plus the transfer. A larger
// SHELL_WORKER_QUEUE_SIZE needs a caller deadline instead (see worker/README.md).
var defaultExecuteTimeout = 2 * time.Minute

// HTTPExecutor calls the shell worker internal API.
type HTTPExecutor struct {
	url    string
	token  string
	client *http.Client
}

// NewHTTPExecutor targets the worker at baseURL and authenticates with token.
// A nil client uses http.DefaultClient; deadlines come from the call context,
// and a context without one is bounded by defaultExecuteTimeout.
func NewHTTPExecutor(baseURL, token string, client *http.Client) *HTTPExecutor {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPExecutor{url: strings.TrimRight(baseURL, "/") + RunsPath, token: token, client: client}
}

func (e *HTTPExecutor) Execute(ctx context.Context, req ExecRequest) (*ExecResult, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultExecuteTimeout)
		defer cancel()
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("shell: encode request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("shell: build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+e.token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &Error{Code: CodeDependencyUnavailable, Message: "shell worker is unreachable", Err: err}
	}
	defer func() { _ = resp.Body.Close() }()
	reader := io.LimitReader(resp.Body, maxResponseBytes)

	if resp.StatusCode == http.StatusOK {
		var res ExecResult
		if err := json.NewDecoder(reader).Decode(&res); err != nil {
			return nil, &Error{Code: CodeDependencyUnavailable, Message: "shell worker returned an unreadable result", Err: err}
		}
		return &res, nil
	}

	var werr Error
	if err := json.NewDecoder(reader).Decode(&werr); err != nil || werr.Code == "" || resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode == http.StatusUnauthorized {
		return nil, &Error{Code: CodeDependencyUnavailable, Message: fmt.Sprintf("shell worker responded with status %d", resp.StatusCode)}
	}
	return nil, &werr
}
