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
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
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

type HITLQuestionOption struct {
	ID          string `json:"id,omitempty"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
}

type HITLQuestion struct {
	ID      string               `json:"id,omitempty"`
	Text    string               `json:"text,omitempty"`
	Type    string               `json:"type,omitempty"`
	Options []HITLQuestionOption `json:"options,omitempty"`
}

type HITLInterrupt struct {
	CheckpointID       string         `json:"checkpoint_id,omitempty"`
	AgentName          string         `json:"agent_name,omitempty"`
	ProviderResponseID string         `json:"provider_response_id,omitempty"`
	Questions          []HITLQuestion `json:"questions,omitempty"`
}

type StreamResult struct {
	StreamedContent string
	Usage           *types.DebugUsage
	Citations       []domain.Citation
	ToolCalls       []ToolCall
	Interrupt       *HITLInterrupt
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
		ToolCalls: make([]ToolCall, 0),
	}
	toolCalls := make(map[string]ToolCall)
	toolOrder := make([]string, 0)

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
				if resultHasSSEPayload(result) {
					return result, err
				}
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
			if payload.Tool != nil {
				recordSSEToolCall(toolCalls, &toolOrder, payload.Tool)
				result.ToolCalls = orderedSSEToolCalls(toolCalls, toolOrder)
			}
			if payload.Interrupt != nil {
				result.Interrupt = mapSSEInterrupt(payload.Interrupt)
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

func resultHasSSEPayload(result *StreamResult) bool {
	if result == nil {
		return false
	}
	return result.SawDone ||
		strings.TrimSpace(result.StreamedContent) != "" ||
		len(result.ToolCalls) > 0 ||
		result.Interrupt != nil ||
		result.Usage != nil ||
		result.ErrorPayload != nil
}

func mapSSEInterrupt(in *httpdto.InterruptEventPayload) *HITLInterrupt {
	if in == nil {
		return nil
	}
	out := &HITLInterrupt{
		CheckpointID:       strings.TrimSpace(in.CheckpointID),
		AgentName:          strings.TrimSpace(in.AgentName),
		ProviderResponseID: strings.TrimSpace(in.ProviderResponseID),
		Questions:          make([]HITLQuestion, 0, len(in.Questions)),
	}
	for _, q := range in.Questions {
		qOut := HITLQuestion{
			ID:      strings.TrimSpace(q.ID),
			Text:    strings.TrimSpace(q.Text),
			Type:    strings.TrimSpace(q.Type),
			Options: make([]HITLQuestionOption, 0, len(q.Options)),
		}
		for _, opt := range q.Options {
			qOut.Options = append(qOut.Options, HITLQuestionOption{
				ID:          strings.TrimSpace(opt.ID),
				Label:       strings.TrimSpace(opt.Label),
				Description: strings.TrimSpace(opt.Description),
			})
		}
		out.Questions = append(out.Questions, qOut)
	}
	return out
}

func recordSSEToolCall(toolCalls map[string]ToolCall, toolOrder *[]string, tool *httpdto.ToolEventPayload) {
	if tool == nil {
		return
	}

	key := strings.TrimSpace(tool.CallID)
	if key == "" {
		key = fmt.Sprintf("__unnamed_tool_%d", len(*toolOrder))
	}

	call, exists := toolCalls[key]
	if !exists {
		call = ToolCall{
			ID: key,
		}
		*toolOrder = append(*toolOrder, key)
	}

	if name := strings.TrimSpace(tool.Name); name != "" {
		call.Name = name
	}
	if args := strings.TrimSpace(tool.Arguments); args != "" {
		call.Arguments = args
	}
	if result := strings.TrimSpace(tool.Result); result != "" {
		call.Result = result
	}
	if err := strings.TrimSpace(tool.Error); err != "" {
		call.Error = err
	}
	if tool.DurationMs > 0 {
		call.DurationMS = tool.DurationMs
	}

	toolCalls[key] = call
}

func orderedSSEToolCalls(toolCalls map[string]ToolCall, toolOrder []string) []ToolCall {
	if len(toolOrder) == 0 {
		return nil
	}
	result := make([]ToolCall, 0, len(toolOrder))
	for _, key := range toolOrder {
		call, ok := toolCalls[key]
		if !ok {
			continue
		}
		result = append(result, call)
	}
	return result
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
