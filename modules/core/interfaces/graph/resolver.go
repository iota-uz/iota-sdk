package graph

import (
	"github.com/iota-uz/iota-sdk/modules/core/interfaces/graph/authorizers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	app               application.Application
	userService       *services.UserService
	uploadService     *services.UploadService
	uploadsAuthorizer types.UploadsAuthorizer
	usersAuthorizer   types.UsersAuthorizer
}

// ResolverOption is a functional option for configuring the Resolver.
type ResolverOption func(*Resolver)

// WithUploadsAuthorizer sets a custom UploadsAuthorizer.
// If not provided, DefaultUploadsAuthorizer will be used.
func WithUploadsAuthorizer(authorizer types.UploadsAuthorizer) ResolverOption {
	return func(r *Resolver) {
		r.uploadsAuthorizer = authorizer
	}
}

// WithUsersAuthorizer sets a custom UsersAuthorizer.
// If not provided, DefaultUsersAuthorizer will be used.
func WithUsersAuthorizer(authorizer types.UsersAuthorizer) ResolverOption {
	return func(r *Resolver) {
		r.usersAuthorizer = authorizer
	}
}

// NewResolver creates a new GraphQL resolver with dependency injection.
// Optional authorizers can be provided via ResolverOption functions to override
// default authorization behavior.
//
// # Usage Examples
//
// Default configuration (uses SDK authorizers):
//
//	resolver := NewResolver(app)
//
// Custom authorizers:
//
//	resolver := NewResolver(app,
//	    WithUploadsAuthorizer(customUploadsAuthorizer),
//	    WithUsersAuthorizer(customUsersAuthorizer),
//	)
func NewResolver(app application.Application, opts ...ResolverOption) *Resolver {
	userService := app.Service(services.UserService{}).(*services.UserService)

	r := &Resolver{
		app:               app,
		userService:       userService,
		uploadService:     app.Service(services.UploadService{}).(*services.UploadService),
		uploadsAuthorizer: authorizers.NewDefaultUploadsAuthorizer(),
		usersAuthorizer:   authorizers.NewDefaultUsersAuthorizer(userService),
	}

	// Apply options to override defaults
	for _, opt := range opts {
		opt(r)
	}

	return r
}
