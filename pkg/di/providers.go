package di

import (
	"context"
	"fmt"
	"reflect"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

// Provider is an interface that can provide a value for a given type
type Provider interface {
	// Ok checks if this provider can handle the requested type
	Ok(t reflect.Type) bool

	// Provide returns the value for the given type from the context
	// Should only be called if Ok returns true
	Provide(t reflect.Type, ctx context.Context) (reflect.Value, error)
}

// Define provider types for each built-in provider
type pageContextProvider struct{}
type localizerProvider struct{}
type userProvider struct{}
type appProvider struct{}
type serviceProvider struct{}
type loggerProvider struct{}
type localeProvider struct{}

func (p *pageContextProvider) Ok(t reflect.Type) bool {
	pageCtxType := reflect.TypeOf((*types.PageContext)(nil))
	return t == pageCtxType
}

func (p *pageContextProvider) Provide(t reflect.Type, ctx context.Context) (reflect.Value, error) {
	return reflect.ValueOf(composables.UsePageCtx(ctx)), nil
}

func (p *localizerProvider) Ok(t reflect.Type) bool {
	localizerType := reflect.TypeOf((*i18n.Localizer)(nil))
	return t == localizerType
}

func (p *localizerProvider) Provide(t reflect.Type, ctx context.Context) (reflect.Value, error) {
	localizer, ok := intl.UseLocalizer(ctx)
	if !ok {
		return reflect.Value{}, fmt.Errorf("localizer not found in context")
	}
	return reflect.ValueOf(localizer), nil
}

func (p *userProvider) Ok(t reflect.Type) bool {
	userType := reflect.TypeOf((*user.User)(nil)).Elem()
	return t == userType || (t.Kind() == reflect.Interface && userType.Implements(t))
}

func (p *userProvider) Provide(t reflect.Type, ctx context.Context) (reflect.Value, error) {
	u, err := composables.UseUser(ctx)
	if err != nil {
		return reflect.Value{}, fmt.Errorf("user not found in context")
	}
	return reflect.ValueOf(u), nil
}

func (p *appProvider) Ok(t reflect.Type) bool {
	appType := reflect.TypeOf((*application.Application)(nil)).Elem()
	return t.Implements(appType)
}

func (p *appProvider) Provide(t reflect.Type, ctx context.Context) (reflect.Value, error) {
	app, err := composables.UseApp(ctx)
	if err != nil {
		return reflect.Value{}, err
	}
	return reflect.ValueOf(app), nil
}

func (p *serviceProvider) Ok(t reflect.Type) bool {
	// Basic check: must be a pointer type for services
	return t.Kind() == reflect.Ptr
}

func (p *serviceProvider) Provide(t reflect.Type, ctx context.Context) (reflect.Value, error) {
	app, err := composables.UseApp(ctx)
	if err != nil {
		return reflect.Value{}, err
	}

	if service, exists := app.Services()[t.Elem()]; exists {
		return reflect.ValueOf(service), nil
	}

	return reflect.Value{}, fmt.Errorf("service not found for type: %v", t)
}

func (p *loggerProvider) Ok(t reflect.Type) bool {
	return t == reflect.TypeOf((*logrus.Entry)(nil))
}

func (p *loggerProvider) Provide(t reflect.Type, ctx context.Context) (reflect.Value, error) {
	return reflect.ValueOf(composables.UseLogger(ctx)), nil
}

func (p *localeProvider) Ok(t reflect.Type) bool {
	return t == reflect.TypeOf(language.Tag{})
}

func (p *localeProvider) Provide(t reflect.Type, ctx context.Context) (reflect.Value, error) {
	l, ok := intl.UseLocale(ctx)
	if !ok {
		return reflect.Value{}, fmt.Errorf("locale not found in context")
	}
	return reflect.ValueOf(l), nil
}

// BuiltinProviders returns the list of built-in providers
func BuiltinProviders() []Provider {
	return []Provider{
		&loggerProvider{},
		&pageContextProvider{},
		&localizerProvider{},
		&userProvider{},
		&appProvider{},
		&serviceProvider{},
		&localeProvider{},
	}
}
