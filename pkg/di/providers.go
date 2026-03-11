package di

import (
	"context"
	"fmt"
	"reflect"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
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
type serviceProvider struct{}
type loggerProvider struct{}
type localeProvider struct{}
type valueProvider struct {
	value reflect.Value
}

type serviceContainer interface {
	Services() map[reflect.Type]interface{}
}

func (p *pageContextProvider) Ok(t reflect.Type) bool {
	pageCtxType := reflect.TypeOf((*types.PageContext)(nil)).Elem()
	return t.Kind() == reflect.Interface && t.Implements(pageCtxType)
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

func (p *serviceProvider) Ok(t reflect.Type) bool {
	// Basic check: must be a pointer type for services
	return t.Kind() == reflect.Ptr
}

func (p *serviceProvider) Provide(t reflect.Type, ctx context.Context) (reflect.Value, error) {
	rawApp := ctx.Value(constants.AppKey)
	if rawApp == nil {
		return reflect.Value{}, fmt.Errorf("app not found in context")
	}
	app, ok := rawApp.(serviceContainer)
	if !ok {
		return reflect.Value{}, fmt.Errorf("app does not expose services")
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

func ValueProvider(value interface{}) Provider {
	if value == nil {
		panic("value is required")
	}
	return &valueProvider{value: reflect.ValueOf(value)}
}

func (p *valueProvider) Ok(t reflect.Type) bool {
	valueType := p.value.Type()
	if valueType.AssignableTo(t) {
		return true
	}
	return t.Kind() == reflect.Interface && valueType.Implements(t)
}

func (p *valueProvider) Provide(t reflect.Type, _ context.Context) (reflect.Value, error) {
	if !p.Ok(t) {
		return reflect.Value{}, fmt.Errorf("value provider does not support type: %v", t)
	}
	return p.value, nil
}

// BuiltinProviders returns the list of built-in providers
func BuiltinProviders() []Provider {
	return []Provider{
		&loggerProvider{},
		&pageContextProvider{},
		&localizerProvider{},
		&userProvider{},
		&serviceProvider{},
		&localeProvider{},
	}
}
