package scaffold

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

// Provider is an interface that can provide a value for a given type
type Provider interface {
	// Ok checks if this provider can handle the requested type
	Ok(t reflect.Type) bool
	
	// Provide returns the value for the given type
	// Should only be called if Ok returns true
	Provide(t reflect.Type, w http.ResponseWriter, r *http.Request) (reflect.Value, error)
}

// Define provider types for each built-in provider
type pageContextProvider struct{}
type httpWriterProvider struct{}
type httpRequestProvider struct{}
type localizerProvider struct{}
type userProvider struct{}
type appProvider struct{}
type serviceProvider struct{}

func (p *pageContextProvider) Ok(t reflect.Type) bool {
	pageCtxType := reflect.TypeOf((*types.PageContext)(nil))
	return t == pageCtxType
}

func (p *pageContextProvider) Provide(t reflect.Type, w http.ResponseWriter, r *http.Request) (reflect.Value, error) {
	return reflect.ValueOf(composables.UsePageCtx(r.Context())), nil
}

func (p *httpWriterProvider) Ok(t reflect.Type) bool {
	writerType := reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()
	return t.Implements(writerType)
}

func (p *httpWriterProvider) Provide(t reflect.Type, w http.ResponseWriter, r *http.Request) (reflect.Value, error) {
	return reflect.ValueOf(w), nil
}

func (p *httpRequestProvider) Ok(t reflect.Type) bool {
	requestType := reflect.TypeOf((*http.Request)(nil))
	return t == requestType
}

func (p *httpRequestProvider) Provide(t reflect.Type, w http.ResponseWriter, r *http.Request) (reflect.Value, error) {
	return reflect.ValueOf(r), nil
}

func (p *localizerProvider) Ok(t reflect.Type) bool {
	localizerType := reflect.TypeOf((*i18n.Localizer)(nil))
	return t == localizerType
}

func (p *localizerProvider) Provide(t reflect.Type, w http.ResponseWriter, r *http.Request) (reflect.Value, error) {
	localizer, ok := composables.UseLocalizer(r.Context())
	if !ok {
		return reflect.Value{}, fmt.Errorf("localizer not found in request context")
	}
	return reflect.ValueOf(localizer), nil
}

func (p *userProvider) Ok(t reflect.Type) bool {
	userType := reflect.TypeOf((*user.User)(nil)).Elem()
	return t.Implements(userType)
}

func (p *userProvider) Provide(t reflect.Type, w http.ResponseWriter, r *http.Request) (reflect.Value, error) {
	u, err := composables.UseUser(r.Context())
	if err != nil {
		return reflect.Value{}, fmt.Errorf("user not found in request context")
	}
	return reflect.ValueOf(u), nil
}

func (p *appProvider) Ok(t reflect.Type) bool {
	appType := reflect.TypeOf((*application.Application)(nil)).Elem()
	return t.Implements(appType)
}

func (p *appProvider) Provide(t reflect.Type, w http.ResponseWriter, r *http.Request) (reflect.Value, error) {
	app, err := composables.UseApp(r.Context())
	if err != nil {
		return reflect.Value{}, err
	}
	return reflect.ValueOf(app), nil
}

func (p *serviceProvider) Ok(t reflect.Type) bool {
	// Basic check: must be a pointer type for services
	return t.Kind() == reflect.Ptr
}

func (p *serviceProvider) Provide(t reflect.Type, w http.ResponseWriter, r *http.Request) (reflect.Value, error) {
	app, err := composables.UseApp(r.Context())
	if err != nil {
		return reflect.Value{}, err
	}

	services := app.Services()
	if service, exists := services[t.Elem()]; exists {
		return reflect.ValueOf(service), nil
	}

	return reflect.Value{}, fmt.Errorf("service not found for type: %v", t)
}

// BuiltinProviders returns the list of built-in providers
func BuiltinProviders() []Provider {
	return []Provider{
		&pageContextProvider{},
		&httpWriterProvider{},
		&httpRequestProvider{},
		&localizerProvider{},
		&userProvider{},
		&appProvider{},
		&serviceProvider{},
	}
}