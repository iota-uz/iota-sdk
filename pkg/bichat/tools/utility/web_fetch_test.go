package utility

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/storage"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type captureStorage struct {
	saveFn func(ctx context.Context, filename string, content io.Reader, metadata storage.FileMetadata) (string, error)
}

func (s *captureStorage) Save(ctx context.Context, filename string, content io.Reader, metadata storage.FileMetadata) (string, error) {
	if s.saveFn != nil {
		return s.saveFn(ctx, filename, content, metadata)
	}
	return "https://files.example.com/" + filename, nil
}

func (s *captureStorage) Get(ctx context.Context, url string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (s *captureStorage) Delete(ctx context.Context, url string) error {
	return nil
}

func (s *captureStorage) Exists(ctx context.Context, url string) (bool, error) {
	return true, nil
}

func newHTTPClient(rt roundTripFunc) *http.Client {
	return &http.Client{Transport: rt}
}

func structuredWebFetch(t *testing.T, tool agents.Tool, input string) (*types.ToolResult, error) {
	t.Helper()
	st, ok := tool.(agents.StructuredTool)
	require.True(t, ok, "tool should implement StructuredTool")
	return st.CallStructured(context.Background(), input)
}

func TestWebFetchTool_Parameters(t *testing.T) {
	t.Parallel()

	tool := NewWebFetchTool()
	params := tool.Parameters()
	require.NotNil(t, params)
	assert.Equal(t, "object", params["type"])

	props, ok := params["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, props, "url")
	assert.Contains(t, props, "save_to_artifacts")
	assert.Contains(t, props, "filename")
}

func TestWebFetchTool_ImageURLSuccess_NoSave(t *testing.T) {
	t.Parallel()

	client := newHTTPClient(func(req *http.Request) (*http.Response, error) {
		assert.Equal(t, "https://example.com/image.png", req.URL.String())
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/png"}},
			Body:       io.NopCloser(strings.NewReader("png-bytes")),
		}, nil
	})

	result, err := structuredWebFetch(t, NewWebFetchTool(WithWebFetchHTTPClient(client)), `{"url":"https://example.com/image.png"}`)
	require.NoError(t, err)
	require.Equal(t, types.CodecJSON, result.CodecID)
	assert.Empty(t, result.Artifacts)

	payload, ok := result.Payload.(types.JSONPayload)
	require.True(t, ok)
	out, ok := payload.Output.(webFetchOutput)
	require.True(t, ok)
	assert.Equal(t, "https://example.com/image.png", out.SourceURL)
	assert.Equal(t, "image/png", out.ContentType)
	assert.True(t, out.Injectable)
	assert.Equal(t, "input_image", out.InjectionType)
	assert.Equal(t, "https://example.com/image.png", out.InjectionURL)
	assert.False(t, out.Saved)
}

func TestWebFetchTool_PDFURLSuccess_NoSave(t *testing.T) {
	t.Parallel()

	client := newHTTPClient(func(req *http.Request) (*http.Response, error) {
		assert.Equal(t, "https://example.com/report", req.URL.String())
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/pdf"}},
			Body:       io.NopCloser(strings.NewReader("%PDF-1.4\n...")),
		}, nil
	})

	result, err := structuredWebFetch(t, NewWebFetchTool(WithWebFetchHTTPClient(client)), `{"url":"https://example.com/report"}`)
	require.NoError(t, err)
	require.Equal(t, types.CodecJSON, result.CodecID)

	payload, ok := result.Payload.(types.JSONPayload)
	require.True(t, ok)
	out, ok := payload.Output.(webFetchOutput)
	require.True(t, ok)
	assert.Equal(t, "application/pdf", out.ContentType)
	assert.Equal(t, "input_file", out.InjectionType)
	assert.Equal(t, "report.pdf", out.Filename)
}

func TestWebFetchTool_UnsupportedMIME(t *testing.T) {
	t.Parallel()

	client := newHTTPClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
			Body:       io.NopCloser(strings.NewReader("plain text")),
		}, nil
	})

	result, err := structuredWebFetch(t, NewWebFetchTool(WithWebFetchHTTPClient(client)), `{"url":"https://example.com/doc.txt"}`)
	require.NoError(t, err)
	require.Equal(t, types.CodecToolError, result.CodecID)

	payload, ok := result.Payload.(types.ToolErrorPayload)
	require.True(t, ok)
	assert.Equal(t, "INVALID_REQUEST", payload.Code)
	assert.Contains(t, payload.Message, "unsupported content type")
}

func TestWebFetchTool_BlockedPrivateURL(t *testing.T) {
	t.Parallel()

	result, err := structuredWebFetch(t, NewWebFetchTool(), `{"url":"http://127.0.0.1/internal"}`)
	require.NoError(t, err)
	require.Equal(t, types.CodecToolError, result.CodecID)

	payload, ok := result.Payload.(types.ToolErrorPayload)
	require.True(t, ok)
	assert.Equal(t, "POLICY_VIOLATION", payload.Code)
}

func TestWebFetchTool_SaveToArtifacts_PersistsAndEmitsArtifact(t *testing.T) {
	t.Parallel()

	client := newHTTPClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/pdf"}},
			Body:       io.NopCloser(strings.NewReader("%PDF-1.4\nsaved")),
		}, nil
	})

	var capturedFilename string
	var capturedType string
	var capturedSize int64
	var capturedBody string
	fs := &captureStorage{
		saveFn: func(ctx context.Context, filename string, content io.Reader, metadata storage.FileMetadata) (string, error) {
			data, _ := io.ReadAll(content)
			capturedFilename = filename
			capturedType = metadata.ContentType
			capturedSize = metadata.Size
			capturedBody = string(data)
			return "https://files.example.com/saved-report.pdf", nil
		},
	}

	result, err := structuredWebFetch(t,
		NewWebFetchTool(
			WithWebFetchHTTPClient(client),
			WithWebFetchStorage(fs),
		),
		`{"url":"https://example.com/report.pdf","save_to_artifacts":true}`,
	)
	require.NoError(t, err)
	require.Equal(t, types.CodecJSON, result.CodecID)
	require.Len(t, result.Artifacts, 1)
	assert.Equal(t, "export", result.Artifacts[0].Type)
	assert.Equal(t, "https://files.example.com/saved-report.pdf", result.Artifacts[0].URL)

	assert.Equal(t, "report.pdf", capturedFilename)
	assert.Equal(t, "application/pdf", capturedType)
	assert.Equal(t, int64(len(capturedBody)), capturedSize)

	payload, ok := result.Payload.(types.JSONPayload)
	require.True(t, ok)
	out, ok := payload.Output.(webFetchOutput)
	require.True(t, ok)
	assert.True(t, out.Saved)
	assert.Equal(t, "https://files.example.com/saved-report.pdf", out.SavedURL)
	assert.Equal(t, "https://files.example.com/saved-report.pdf", out.InjectionURL)
}

func TestWebFetchTool_SaveWithoutStorage_ReturnsError(t *testing.T) {
	t.Parallel()

	client := newHTTPClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/png"}},
			Body:       io.NopCloser(strings.NewReader("png-data")),
		}, nil
	})

	result, err := structuredWebFetch(t,
		NewWebFetchTool(WithWebFetchHTTPClient(client)),
		`{"url":"https://example.com/image.png","save_to_artifacts":true}`,
	)
	require.NoError(t, err)
	require.Equal(t, types.CodecToolError, result.CodecID)

	payload, ok := result.Payload.(types.ToolErrorPayload)
	require.True(t, ok)
	assert.Equal(t, "SERVICE_UNAVAILABLE", payload.Code)
	assert.Contains(t, payload.Message, "storage is not configured")
}

func TestWebFetchTool_OversizedResponseBlocked(t *testing.T) {
	t.Parallel()

	client := newHTTPClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/pdf"}},
			Body:       io.NopCloser(strings.NewReader("1234567890")),
		}, nil
	})

	result, err := structuredWebFetch(t,
		NewWebFetchTool(
			WithWebFetchHTTPClient(client),
			WithWebFetchMaxDownloadBytes(5),
		),
		`{"url":"https://example.com/report.pdf"}`,
	)
	require.NoError(t, err)
	require.Equal(t, types.CodecToolError, result.CodecID)

	payload, ok := result.Payload.(types.ToolErrorPayload)
	require.True(t, ok)
	assert.Equal(t, "DATA_TOO_LARGE", payload.Code)
}
