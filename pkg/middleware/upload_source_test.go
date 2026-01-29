package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

func TestWithUploadSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		config         *middleware.UploadSourceConfig
		expectedSource string
	}{
		{
			name:           "uses default provider when nil config",
			config:         nil,
			expectedSource: "general",
		},
		{
			name: "uses default provider when nil provider",
			config: &middleware.UploadSourceConfig{
				Provider: nil,
			},
			expectedSource: "general",
		},
		{
			name: "uses custom provider",
			config: &middleware.UploadSourceConfig{
				Provider: &mockSourceProvider{source: "custom"},
			},
			expectedSource: "custom",
		},
		{
			name: "defaults to general when provider returns empty",
			config: &middleware.UploadSourceConfig{
				Provider: &mockSourceProvider{source: ""},
			},
			expectedSource: "general",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := middleware.WithUploadSource(tt.config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				source := composables.UseUploadSource(r.Context())
				if source != tt.expectedSource {
					t.Errorf("expected source %q, got %q", tt.expectedSource, source)
				}
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
		})
	}
}

func TestWithUploadSource_AccessChecker(t *testing.T) {
	t.Parallel()

	checker := &mockAccessChecker{}
	config := &middleware.UploadSourceConfig{
		Provider:      &middleware.DefaultUploadSourceProvider{},
		AccessChecker: checker,
	}

	handler := middleware.WithUploadSource(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		retrievedChecker := composables.UseUploadAccessChecker(r.Context())
		if retrievedChecker != checker {
			t.Error("expected access checker to be set in context")
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
}

// mockSourceProvider is a mock implementation for testing
type mockSourceProvider struct {
	source string
}

func (m *mockSourceProvider) GetUploadSource(r *http.Request) string {
	return m.source
}

// mockAccessChecker is a mock implementation for testing
type mockAccessChecker struct{}

func (m *mockAccessChecker) CanAccessSource(r *http.Request, source string) error {
	return nil
}

func (m *mockAccessChecker) CanUploadToSource(r *http.Request, source string) error {
	return nil
}
