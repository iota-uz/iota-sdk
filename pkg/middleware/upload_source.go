package middleware

import (
	"net/http"

	"github.com/iota-uz/iota-sdk/pkg/composables"
)

// UploadSourceProvider determines the source for uploads based on request context.
// Child projects implement this to customize source assignment.
type UploadSourceProvider interface {
	// GetUploadSource returns the source string for the current request.
	GetUploadSource(r *http.Request) string
}

// DefaultUploadSourceProvider returns empty source (no filtering).
type DefaultUploadSourceProvider struct{}

func (d *DefaultUploadSourceProvider) GetUploadSource(r *http.Request) string {
	return ""
}

// UploadSourceConfig holds the configuration for upload source middleware.
type UploadSourceConfig struct {
	Provider      UploadSourceProvider
	AccessChecker composables.UploadSourceAccessChecker
}

// WithUploadSource creates middleware that sets the upload source in context.
// Uses the configured provider to determine source.
func WithUploadSource(config *UploadSourceConfig) func(http.Handler) http.Handler {
	if config == nil {
		config = &UploadSourceConfig{}
	}

	provider := config.Provider
	if provider == nil {
		provider = &DefaultUploadSourceProvider{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			source := provider.GetUploadSource(r)
			ctx := composables.WithUploadSource(r.Context(), source)
			if config.AccessChecker != nil {
				ctx = composables.WithUploadAccessChecker(ctx, config.AccessChecker)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
