package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportToPDFTool_CallStructured_EmitsArtifact(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/forms/chromium/convert/html" {
			http.NotFound(w, r)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if len(body) == 0 {
			http.Error(w, "empty body", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("%PDF-1.7 test"))
	}))
	defer server.Close()

	tool := NewExportToPDFTool(server.URL, nil).(*ExportToPDFTool)
	result, err := tool.CallStructured(context.Background(), `{"html":"<html><body>Hello</body></html>","filename":"summary.pdf"}`)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Artifacts, 1)

	artifact := result.Artifacts[0]
	assert.Equal(t, "export", artifact.Type)
	assert.Equal(t, "summary.pdf", artifact.Name)
	assert.Equal(t, "application/pdf", artifact.MimeType)
	assert.Equal(t, "/exports/summary.pdf", artifact.URL)
	assert.Positive(t, artifact.SizeBytes)

	out, err := tool.Call(context.Background(), `{"html":"<html><body>Hello</body></html>","filename":"summary.pdf"}`)
	require.NoError(t, err)
	var payload pdfExportOutput
	require.NoError(t, json.Unmarshal([]byte(out), &payload))
	assert.Equal(t, "summary.pdf", payload.Filename)
}

func TestExportToPDFTool_CallStructured_ValidationError(t *testing.T) {
	t.Parallel()

	tool := NewExportToPDFTool("http://example.com", nil).(*ExportToPDFTool)
	result, err := tool.CallStructured(context.Background(), `{"filename":"x.pdf"}`)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, types.CodecToolError, result.CodecID)
	assert.Empty(t, result.Artifacts)
}
