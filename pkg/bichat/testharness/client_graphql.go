package testharness

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type NotAuthenticatedRedirectError struct {
	StatusCode  int
	Location    string
	ContentType string
	BodySnippet string
}

func (e *NotAuthenticatedRedirectError) Error() string {
	loc := e.Location
	if loc == "" {
		loc = "<missing>"
	}
	ct := e.ContentType
	if ct == "" {
		ct = "<missing>"
	}
	return fmt.Sprintf("not authenticated (redirect): status=%d location=%s content_type=%s", e.StatusCode, loc, ct)
}

type UnexpectedContentTypeError struct {
	EndpointURL  string
	ContentType  string
	BodySnippet  string
	StatusCode   int
	ExpectedHint string
}

func (e *UnexpectedContentTypeError) Error() string {
	ct := e.ContentType
	if ct == "" {
		ct = "<missing>"
	}
	return fmt.Sprintf("unexpected content-type: status=%d content_type=%s expected=%s", e.StatusCode, ct, e.ExpectedHint)
}

type HTTPStatusError struct {
	EndpointURL  string
	StatusCode   int
	ContentType  string
	BodySnippet  string
	ExpectedHint string
}

func (e *HTTPStatusError) Error() string {
	ct := e.ContentType
	if ct == "" {
		ct = "<missing>"
	}
	return fmt.Sprintf("http status %d content_type=%s", e.StatusCode, ct)
}

type GraphQLClient struct {
	httpClient   *http.Client
	endpointURL  string
	cookieName   string
	sessionToken string
}

func NewGraphQLClient(cfg Config) *GraphQLClient {
	return &GraphQLClient{
		httpClient:   http.DefaultClient,
		endpointURL:  strings.TrimRight(cfg.ServerURL, "/") + cfg.GraphQLEndpointPath,
		cookieName:   cfg.CookieName,
		sessionToken: cfg.SessionToken,
	}
}

func (c *GraphQLClient) WithHTTPClient(client *http.Client) *GraphQLClient {
	if client != nil {
		c.httpClient = client
	}
	return c
}

type graphQLError struct {
	Message string `json:"message"`
}

type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphQLError  `json:"errors"`
}

func (c *GraphQLClient) Do(ctx context.Context, query string, variables map[string]any, out any) error {
	if c.httpClient == nil {
		return errors.New("http client is nil")
	}

	body := map[string]any{
		"query": query,
	}
	if variables != nil {
		body["variables"] = variables
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return fmt.Errorf("encode graphql request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpointURL, &buf)
	if err != nil {
		return fmt.Errorf("create graphql request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cookieName != "" && c.sessionToken != "" {
		req.Header.Set("Cookie", fmt.Sprintf("%s=%s", c.cookieName, c.sessionToken))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("graphql request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Detect redirect-to-login patterns early (common when auth cookie missing/invalid).
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		snippet := readSnippet(resp.Body, 512)
		return &NotAuthenticatedRedirectError{
			StatusCode:  resp.StatusCode,
			Location:    resp.Header.Get("Location"),
			ContentType: resp.Header.Get("Content-Type"),
			BodySnippet: snippet,
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPStatusError{
			EndpointURL: c.endpointURL,
			StatusCode:  resp.StatusCode,
			ContentType: resp.Header.Get("Content-Type"),
			BodySnippet: readSnippet(resp.Body, 512),
		}
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") && !strings.Contains(ct, "application/graphql-response+json") {
		return &UnexpectedContentTypeError{
			EndpointURL:  c.endpointURL,
			StatusCode:   resp.StatusCode,
			ContentType:  ct,
			BodySnippet:  readSnippet(resp.Body, 512),
			ExpectedHint: "application/json",
		}
	}

	var gqlResp graphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return fmt.Errorf("decode graphql response: %w", err)
	}
	if len(gqlResp.Errors) > 0 {
		msgs := make([]string, 0, len(gqlResp.Errors))
		for _, e := range gqlResp.Errors {
			if e.Message != "" {
				msgs = append(msgs, e.Message)
			}
		}
		if len(msgs) == 0 {
			return errors.New("graphql returned errors")
		}
		return fmt.Errorf("graphql returned errors: %s", strings.Join(msgs, "; "))
	}
	if out == nil {
		return nil
	}
	if len(gqlResp.Data) == 0 {
		return errors.New("graphql response missing data")
	}
	if err := json.Unmarshal(gqlResp.Data, out); err != nil {
		return fmt.Errorf("decode graphql data: %w", err)
	}
	return nil
}

func readSnippet(r io.Reader, limit int64) string {
	if r == nil {
		return ""
	}
	b, _ := io.ReadAll(io.LimitReader(r, limit))
	s := strings.TrimSpace(string(b))
	if s == "" {
		return ""
	}
	return s
}

const createSessionMutation = `
mutation($title: String) {
  createSession(title: $title) {
    id
  }
}`

func (c *GraphQLClient) CreateSession(ctx context.Context, title string) (uuid.UUID, error) {
	var titleVar any = title
	if strings.TrimSpace(title) == "" {
		titleVar = nil
	}
	vars := map[string]any{"title": titleVar}

	var out struct {
		CreateSession struct {
			ID string `json:"id"`
		} `json:"createSession"`
	}

	if err := c.Do(ctx, createSessionMutation, vars, &out); err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(out.CreateSession.ID)
}

type ToolCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"toolCalls"`
}

const messagesQuery = `
query($sessionId: UUID!, $limit: Int, $offset: Int) {
  messages(sessionId: $sessionId, limit: $limit, offset: $offset) {
    role
    content
    toolCalls { name arguments }
  }
}`

func (c *GraphQLClient) Messages(ctx context.Context, sessionID uuid.UUID, limit, offset int) ([]Message, error) {
	var out struct {
		Messages []Message `json:"messages"`
	}
	vars := map[string]any{
		"sessionId": sessionID.String(),
		"limit":     limit,
		"offset":    offset,
	}
	if err := c.Do(ctx, messagesQuery, vars, &out); err != nil {
		return nil, err
	}
	return out.Messages, nil
}
