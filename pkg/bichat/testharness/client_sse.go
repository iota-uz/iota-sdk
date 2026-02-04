package testharness

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/httpdto"
)

type SSEClient struct {
	httpClient   *http.Client
	endpointURL  string
	cookieName   string
	sessionToken string
}

func NewSSEClient(cfg Config) *SSEClient {
	return &SSEClient{
		httpClient:   http.DefaultClient,
		endpointURL:  strings.TrimRight(cfg.ServerURL, "/") + cfg.StreamEndpointPath,
		cookieName:   cfg.CookieName,
		sessionToken: cfg.SessionToken,
	}
}

func (c *SSEClient) WithHTTPClient(client *http.Client) *SSEClient {
	if client != nil {
		c.httpClient = client
	}
	return c
}

type StreamResult struct {
	StreamedContent string
	Usage           *services.TokenUsage
	Citations       []domain.Citation
	ErrorPayload    *httpdto.StreamChunkPayload
}

var errSSEDone = errors.New("sse done")

func (c *SSEClient) StreamMessage(ctx context.Context, sessionID uuid.UUID, content string) (*StreamResult, error) {
	if c.httpClient == nil {
		return nil, errors.New("http client is nil")
	}

	reqBody := map[string]any{
		"sessionId":   sessionID.String(),
		"content":     content,
		"attachments": []any{},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(reqBody); err != nil {
		return nil, fmt.Errorf("encode sse request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpointURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("create sse request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if c.cookieName != "" && c.sessionToken != "" {
		req.Header.Set("Cookie", fmt.Sprintf("%s=%s", c.cookieName, c.sessionToken))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sse request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("sse http status %d", resp.StatusCode)
	}

	result := &StreamResult{
		Citations: make([]domain.Citation, 0),
	}

	decodeErr := decodeSSE(resp.Body, func(payload httpdto.StreamChunkPayload) error {
		if payload.Content != "" && payload.Type == "content" {
			result.StreamedContent += payload.Content
		}
		if payload.Citation != nil {
			result.Citations = append(result.Citations, *payload.Citation)
		}
		if payload.Usage != nil {
			result.Usage = payload.Usage
		}
		if payload.Type == "error" || payload.Error != "" {
			cp := payload
			result.ErrorPayload = &cp
		}
		if payload.Type == "done" {
			return errSSEDone
		}
		return nil
	})
	if decodeErr != nil && !errors.Is(decodeErr, errSSEDone) {
		return nil, decodeErr
	}

	return result, nil
}

func decodeSSE(r io.Reader, onPayload func(httpdto.StreamChunkPayload) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	dataLines := make([]string, 0, 4)

	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		data := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]

		var payload httpdto.StreamChunkPayload
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			return fmt.Errorf("decode sse payload: %w", err)
		}
		return onPayload(payload)
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}

		if strings.HasPrefix(line, "data:") {
			v := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			dataLines = append(dataLines, v)
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read sse stream: %w", err)
	}

	return flush()
}
