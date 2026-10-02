// Package serrorgql projects GraphQL execution errors without changing transport policy.
package serrorgql

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/iota-uz/go-i18n/v2/i18n"
	serrors "github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/serrors/serrorlog"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// PublicCarrier declares a specialized safe GraphQL contract.
type PublicCarrier interface {
	GraphQLCode() string
	GraphQLExtensions() map[string]serrors.Value
}

// Presenter applies to execution errors and retains protocol extensions.
func Presenter(localizer func(context.Context) *i18n.Localizer) graphql.ErrorPresenterFunc {
	return func(ctx context.Context, err error) *gqlerror.Error {
		base := graphql.DefaultErrorPresenter(ctx, err)
		_, direct := err.(*gqlerror.Error)
		executing := graphql.HasOperationContext(ctx) && graphql.GetOperationContext(ctx).Operation != nil
		if direct && !executing && (base.Extensions["code"] == "GRAPHQL_PARSE_FAILED" || base.Extensions["code"] == "GRAPHQL_VALIDATION_FAILED") {
			return base
		}
		serrorlog.Log(ctx, err, "GraphQL request failed")
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
		if carrier, ok := serrors.FindPublicCarrier[PublicCarrier](err); ok {
			if declared := carrier.GraphQLCode(); declared != "" {
				extensions["code"] = declared
			}
			for key, value := range carrier.GraphQLExtensions() {
				if key != "code" && key != "reason" && key != "fields" {
					extensions[key] = serrors.PublicValue(value, l)
				}
			}
		}
		if p.Reason != "" {
			extensions["reason"] = p.Reason
		}
		if len(p.Fields) > 0 {
			extensions["fields"] = p.Fields
		}
		return &gqlerror.Error{Path: base.Path, Locations: base.Locations, Message: p.Message, Extensions: extensions, Err: err}
	}
}
