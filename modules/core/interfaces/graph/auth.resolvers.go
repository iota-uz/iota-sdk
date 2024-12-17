package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.
// Code generated by github.com/99designs/gqlgen version v0.17.57

import (
	"context"
	"fmt"
	"github.com/iota-agency/iota-sdk/modules/core/interfaces/graph/mappers"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"github.com/iota-agency/iota-sdk/pkg/services"
	"net/http"

	model "github.com/iota-agency/iota-sdk/modules/core/interfaces/graph/gqlmodels"
)

// Authenticate is the resolver for the authenticate field.
func (r *mutationResolver) Authenticate(ctx context.Context, email string, password string) (*model.Session, error) {
	writer, ok := composables.UseWriter(ctx)
	if !ok {
		return nil, fmt.Errorf("request params not found")
	}
	authService := r.app.Service(services.AuthService{}).(*services.AuthService)
	_, session, err := authService.Authenticate(ctx, email, password)
	if err != nil {
		return nil, err
	}
	conf := configuration.Use()
	cookie := &http.Cookie{
		Path:     conf.SidCookieKey,
		Value:    session.Token,
		Expires:  session.ExpiresAt,
		HttpOnly: false,
		SameSite: http.SameSiteDefaultMode,
		Secure:   false,
		Domain:   conf.Domain,
	}
	http.SetCookie(writer, cookie)
	return mappers.SessionToGraphModel(session), nil
}

// GoogleAuthenticate is the resolver for the googleAuthenticate field.
func (r *mutationResolver) GoogleAuthenticate(ctx context.Context) (string, error) {
	//return r.app.AuthService.GoogleAuthenticate()
	panic(fmt.Errorf("not implemented: GoogleAuthenticate - googleAuthenticate"))
}

// DeleteSession is the resolver for the deleteSession field.
func (r *mutationResolver) DeleteSession(ctx context.Context, token string) (bool, error) {
	panic(fmt.Errorf("not implemented: DeleteSession - deleteSession"))
}

// SessionDeleted is the resolver for the sessionDeleted field.
func (r *subscriptionResolver) SessionDeleted(ctx context.Context) (<-chan int64, error) {
	panic(fmt.Errorf("not implemented: SessionDeleted - sessionDeleted"))
}