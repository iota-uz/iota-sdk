package listext

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

type engineKey struct{}

// WithEngine returns ctx carrying the engine native lists consult.
func WithEngine(ctx context.Context, e *Engine) context.Context {
	return context.WithValue(ctx, engineKey{}, e)
}

// FromContext returns the engine installed by Middleware, or nil when the
// application has no extension runtime (lists then show no extra columns).
func FromContext(ctx context.Context) *Engine {
	e, _ := ctx.Value(engineKey{}).(*Engine)
	return e
}

// Middleware installs the engine on every request.
func Middleware(e *Engine) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(WithEngine(r.Context(), e)))
		})
	}
}

// SubjectFromContext builds the invoking subject from the request user: the
// permissions it holds right now, so access is evaluated per request.
func SubjectFromContext(ctx context.Context) (spi.Subject, error) {
	u, err := composables.UseUser(ctx)
	if err != nil {
		return spi.Subject{}, err
	}
	locale := ""
	if tag, ok := intl.UseLocale(ctx); ok {
		locale = tag.String()
	}
	return spi.Subject{
		TenantID:    u.TenantID(),
		UserID:      int64(u.ID()),
		Kind:        spi.SubjectInitiator,
		Permissions: composables.EffectivePermissionNames(u),
		Locale:      locale,
	}, nil
}

// Compile compiles the plan for a native list running in the request
// transaction. It returns a nil plan (and no error) when the application has
// no engine, so callers need no branch.
func Compile(ctx context.Context, in Input) (*Plan, error) {
	e := FromContext(ctx)
	if e == nil {
		return nil, nil
	}
	sub, err := SubjectFromContext(ctx)
	if err != nil {
		return nil, err
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	return e.Compile(ctx, tx, sub, in)
}

// LangFromContext is the primary language of the request locale ("en" when
// unknown), used to pick column labels.
func LangFromContext(ctx context.Context) string {
	if tag, ok := intl.UseLocale(ctx); ok {
		base, _ := tag.Base()
		return base.String()
	}
	return "en"
}
