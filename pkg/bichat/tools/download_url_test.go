package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/stretchr/testify/assert"
)

func TestBuildDownloadURL(t *testing.T) {
	t.Parallel()

	ctxWithRequest := withRequestContext(t, "http://internal.local/stream", map[string]string{
		"X-Forwarded-Host":  "app.example.com",
		"X-Forwarded-Proto": "https",
	})

	tests := []struct {
		name     string
		ctx      context.Context
		baseURL  string
		filename string
		want     string
	}{
		{
			name:     "absolute base URL",
			ctx:      context.Background(),
			baseURL:  "https://cdn.example.com/exports",
			filename: "report.xlsx",
			want:     "https://cdn.example.com/exports/report.xlsx",
		},
		{
			name:     "relative base URL with request context",
			ctx:      ctxWithRequest,
			baseURL:  "/exports",
			filename: "report.xlsx",
			want:     "https://app.example.com/exports/report.xlsx",
		},
		{
			name:     "empty base URL with request context",
			ctx:      ctxWithRequest,
			baseURL:  "",
			filename: "report.xlsx",
			want:     "https://app.example.com/report.xlsx",
		},
		{
			name:     "empty base URL without request context",
			ctx:      context.Background(),
			baseURL:  "",
			filename: "report.xlsx",
			want:     "/report.xlsx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := buildDownloadURL(tt.ctx, tt.baseURL, tt.filename)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveDownloadURL(t *testing.T) {
	t.Parallel()

	ctxWithRequest := withRequestContext(t, "http://internal.local/stream", map[string]string{
		"X-Forwarded-Host":  "analytics.example.com",
		"X-Forwarded-Proto": "https",
	})

	assert.Equal(t, "https://analytics.example.com/exports/file.pdf", resolveDownloadURL(ctxWithRequest, "/exports/file.pdf"))
	assert.Equal(t, "https://cdn.example.com/file.pdf", resolveDownloadURL(ctxWithRequest, "https://cdn.example.com/file.pdf"))
	assert.Equal(t, "/exports/file.pdf", resolveDownloadURL(context.Background(), "/exports/file.pdf"))
}

func withRequestContext(t *testing.T, requestURL string, headers map[string]string) context.Context {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, requestURL, nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return composables.WithParams(context.Background(), &composables.Params{
		Request: req,
	})
}
