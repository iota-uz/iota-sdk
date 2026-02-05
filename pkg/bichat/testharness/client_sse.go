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
	"sync"
	"time"

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
	doneDrain    time.Duration
}

func NewSSEClient(cfg Config) *SSEClient {
	return &SSEClient{
		httpClient:   http.DefaultClient,
		endpointURL:  strings.TrimRight(cfg.ServerURL, "/") + cfg.StreamEndpointPath,
		cookieName:   cfg.CookieName,
		sessionToken: cfg.SessionToken,
		doneDrain:    time.Duration(cfg.StreamDoneDrainMillis) * time.Millisecond,
	}
}

func (c *SSEClient) WithHTTPClient(client *http.Client) *SSEClient {
	if client != nil {
		c.httpClient = client
	}
	return c
}

type StreamSinks struct {
	Raw  io.Writer // raw SSE bytes
	JSON io.Writer // JSONL of decoded payloads
}

type StreamResult struct {
	StreamedContent string
	Usage           *services.TokenUsage
	Citations       []domain.Citation
	ErrorPayload    *httpdto.StreamChunkPayload
	SawDone         bool
}

func (c *SSEClient) StreamMessage(ctx context.Context, sessionID uuid.UUID, content string, sinks StreamSinks) (*StreamResult, error) {
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

	// Detect redirect-to-login patterns early (common when auth cookie missing/invalid).
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return nil, &NotAuthenticatedRedirectError{
			StatusCode:  resp.StatusCode,
			Location:    resp.Header.Get("Location"),
			ContentType: resp.Header.Get("Content-Type"),
			BodySnippet: readSnippet(resp.Body, 512),
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPStatusError{
			EndpointURL: c.endpointURL,
			StatusCode:  resp.StatusCode,
			ContentType: resp.Header.Get("Content-Type"),
			BodySnippet: readSnippet(resp.Body, 512),
		}
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		return nil, &UnexpectedContentTypeError{
			EndpointURL:  c.endpointURL,
			StatusCode:   resp.StatusCode,
			ContentType:  ct,
			BodySnippet:  readSnippet(resp.Body, 512),
			ExpectedHint: "text/event-stream",
		}
	}

	result := &StreamResult{
		Citations: make([]domain.Citation, 0),
	}

	bodyReader := resp.Body
	if sinks.Raw != nil {
		bodyReader = io.NopCloser(io.TeeReader(resp.Body, sinks.Raw))
	}

	// Decode in a goroutine so we can stop after "done" + a drain period if the server keeps the connection open.
	payloadCh := make(chan httpdto.StreamChunkPayload, 64)
	errCh := make(chan error, 1)
	var once sync.Once

	go func() {
		err := decodeSSE(bodyReader, func(payload httpdto.StreamChunkPayload) error {
			payloadCh <- payload
			return nil
		})
		once.Do(func() {
			errCh <- err
			close(payloadCh)
		})
	}()

	var doneTimer *time.Timer
	stopDoneTimer := func() {
		if doneTimer != nil {
			doneTimer.Stop()
			doneTimer = nil
		}
	}

	for {
		var doneCh <-chan time.Time
		if doneTimer != nil {
			doneCh = doneTimer.C
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("sse context done: %w", ctx.Err())
		case err := <-errCh:
			stopDoneTimer()
			if err != nil {
				return nil, err
			}
			return result, nil
		case payload, ok := <-payloadCh:
			if !ok {
				stopDoneTimer()
				return result, nil
			}
			if sinks.JSON != nil {
				if b, err := json.Marshal(payload); err == nil {
					_, _ = sinks.JSON.Write(append(b, '\n'))
				}
			}

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
				result.SawDone = true
				if doneTimer == nil && c.doneDrain > 0 {
					doneTimer = time.NewTimer(c.doneDrain)
				}
			}
		case <-doneCh:
			// Drain window expired; close body to stop reading.
			_ = resp.Body.Close()
			stopDoneTimer()
			return result, nil
		}
	}
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
