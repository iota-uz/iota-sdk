package composables

import (
	"context"
	"net/http"

	"github.com/iota-uz/iota-sdk/pkg/constants"
)

// UploadSourceAccessChecker interface for checking source access.
type UploadSourceAccessChecker interface {
	CanAccessSource(r *http.Request, source string) error
	CanUploadToSource(r *http.Request, source string) error
}

// UseUploadSource returns the upload source from context.
// Returns empty string if not set.
func UseUploadSource(ctx context.Context) string {
	source, _ := ctx.Value(constants.UploadSourceKey).(string)
	return source
}

// WithUploadSource sets the upload source in context.
func WithUploadSource(ctx context.Context, source string) context.Context {
	return context.WithValue(ctx, constants.UploadSourceKey, source)
}

// UseUploadAccessChecker returns the access checker from context.
// Returns nil if not set.
func UseUploadAccessChecker(ctx context.Context) UploadSourceAccessChecker {
	checker, _ := ctx.Value(constants.UploadAccessCheckerKey).(UploadSourceAccessChecker)
	return checker
}

// WithUploadAccessChecker sets the access checker in context.
func WithUploadAccessChecker(ctx context.Context, checker UploadSourceAccessChecker) context.Context {
	return context.WithValue(ctx, constants.UploadAccessCheckerKey, checker)
}

// CheckUploadSourceAccess checks if the current context allows access to the given source.
// Returns nil if no checker is configured (allows all by default).
func CheckUploadSourceAccess(ctx context.Context, source string, r *http.Request) error {
	checker := UseUploadAccessChecker(ctx)
	if checker == nil {
		return nil
	}
	return checker.CanAccessSource(r, source)
}

// CheckUploadToSource checks if the current context allows uploading to the given source.
// Returns nil if no checker is configured (allows all by default).
func CheckUploadToSource(ctx context.Context, source string, r *http.Request) error {
	checker := UseUploadAccessChecker(ctx)
	if checker == nil {
		return nil
	}
	return checker.CanUploadToSource(r, source)
}
