package ui

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/pkg/listext"
	"github.com/iota-uz/iota-sdk/pkg/shellext"
)

// Middleware installs the runtime on every request: the native list column
// engine, the navigation projection, the shell activation markup and the
// islands host (slots and list cells). It
// reads the snapshot lazily, so a request that renders none of them costs
// nothing.
func (r *Runtime) Middleware(overlays listext.OverlayJoiner) mux.MiddlewareFunc {
	install := r.Installer(overlays)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(install(req.Context())))
		})
	}
}

// Installer returns the function that adds the runtime to a request context;
// Middleware applies it to every request.
func (r *Runtime) Installer(overlays listext.OverlayJoiner) func(context.Context) context.Context {
	var engine *listext.Engine
	if overlays != nil {
		engine = listext.NewEngine(r.Columns(), overlays)
	}
	nav := r.Navigation()
	return func(ctx context.Context) context.Context {
		ctx = shellext.WithNavigation(ctx, nav)
		ctx = shellext.WithMarkup(ctx, r.Markup)
		ctx = shellext.WithIslands(ctx, r.newRequestIslands())
		if engine != nil {
			ctx = listext.WithEngine(ctx, engine)
		}
		return ctx
	}
}
