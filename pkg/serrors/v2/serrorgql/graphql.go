// Package serrorgql projects GraphQL execution errors without changing transport policy.
package serrorgql

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/iota-uz/go-i18n/v2/i18n"
	serrors "github.com/iota-uz/iota-sdk/pkg/serrors/v2"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// Presenter is installed through graphql.Handler.SetErrorPresenter. Protocol
// errors remain owned by the transport, retaining their original extensions.
func Presenter(localizer func(context.Context) *i18n.Localizer) graphql.ErrorPresenterFunc {
	return func(ctx context.Context, err error) *gqlerror.Error {
		base := graphql.DefaultErrorPresenter(ctx, err)
		if base.Extensions["code"] == "GRAPHQL_PARSE_FAILED" || base.Extensions["code"] == "GRAPHQL_VALIDATION_FAILED" {
			return base
		}
		var l *i18n.Localizer
		if localizer != nil {
			l = localizer(ctx)
		}
		p := serrors.Public(err, l)
		code := p.Code
		if serrors.HasCode(err, serrors.Unauthenticated) {
			code = "UNAUTHORIZED"
		}
		extensions := map[string]any{"code": code}
		if p.Reason != "" {
			extensions["reason"] = p.Reason
		}
		if len(p.Fields) > 0 {
			extensions["fields"] = p.Fields
		}
		return &gqlerror.Error{Path: base.Path, Locations: base.Locations, Message: p.Message, Extensions: extensions, Err: err}
	}
}
