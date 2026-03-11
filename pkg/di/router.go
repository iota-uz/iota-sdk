// Package di provides this package.
package di

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/constants"
)

// Invoke creates a generic DI function that can be used for any function type
func Invoke(ctx context.Context, fn interface{}, customProviders ...Provider) ([]reflect.Value, error) {
	allProviders := append([]Provider{}, customProviders...)
	allProviders = append(allProviders, BuiltinProviders()...)
	return InvokeWithProviders(ctx, fn, allProviders...)
}

// InvokeWithProviders creates a generic DI function that resolves dependencies using only the given providers.
func InvokeWithProviders(ctx context.Context, fn interface{}, providers ...Provider) ([]reflect.Value, error) {
	diContext := NewDIContextWithProviders(providers...)
	return diContext.Invoke(ctx, fn)
}

// H creates a dependency injection HTTP handler
func H(handler interface{}, customProviders ...Provider) http.HandlerFunc {
	allProviders := append([]Provider{}, customProviders...)
	allProviders = append(allProviders, BuiltinProviders()...)
	diContext := NewDIContextWithProviders(allProviders...)
	return createHandlerFunc(diContext, handler)
}

// DIContext holds context for dependency injection
type DIContext struct {
	allProviders     []Provider
	matchedProviders map[reflect.Type]Provider
	mu               sync.RWMutex
}

// NewDIContext creates a new DIContext with custom providers
func NewDIContext(customProviders ...Provider) *DIContext {
	allProviders := append([]Provider{}, customProviders...)
	allProviders = append(allProviders, BuiltinProviders()...)
	return NewDIContextWithProviders(allProviders...)
}

// NewDIContextWithProviders creates a new DIContext that uses exactly the given providers.
func NewDIContextWithProviders(providers ...Provider) *DIContext {
	return &DIContext{
		allProviders:     append([]Provider{}, providers...),
		matchedProviders: make(map[reflect.Type]Provider),
	}
}

func (d *DIContext) resolveProvider(argType reflect.Type) (Provider, error) {
	d.mu.RLock()
	provider, ok := d.matchedProviders[argType]
	d.mu.RUnlock()
	if ok {
		return provider, nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Re-check after upgrading to write lock.
	if provider, ok = d.matchedProviders[argType]; ok {
		return provider, nil
	}

	for _, p := range d.allProviders {
		if p.Ok(argType) {
			d.matchedProviders[argType] = p
			return p, nil
		}
	}

	return nil, fmt.Errorf("no provider found for type: %v", argType)
}

// provideValue resolves a single dependency value for a given type
func (d *DIContext) provideValue(argType reflect.Type, ctx context.Context) (reflect.Value, error) {
	if argType == reflect.TypeOf((*context.Context)(nil)).Elem() {
		return reflect.ValueOf(ctx), nil
	}
	provider, err := d.resolveProvider(argType)
	if err != nil {
		if value, ok := provideAppValue(argType, ctx); ok {
			return value, nil
		}
		return reflect.Value{}, err
	}
	return provider.Provide(argType, ctx)
}

func provideAppValue(argType reflect.Type, ctx context.Context) (reflect.Value, bool) {
	rawApp := ctx.Value(constants.AppKey)
	if rawApp == nil {
		return reflect.Value{}, false
	}
	value := reflect.ValueOf(rawApp)
	valueType := value.Type()
	if valueType.AssignableTo(argType) {
		return value, true
	}
	if argType.Kind() == reflect.Interface && valueType.Implements(argType) {
		return value, true
	}
	return reflect.Value{}, false
}

// Invoke resolves dependencies and calls the provided function with the given context
func (d *DIContext) Invoke(ctx context.Context, fn interface{}) ([]reflect.Value, error) {
	fnValue := reflect.ValueOf(fn)
	typeOf := fnValue.Type()
	numArgs := typeOf.NumIn()

	argTypes := make([]reflect.Type, numArgs)
	for i := 0; i < numArgs; i++ {
		argTypes[i] = typeOf.In(i)
	}

	args := make([]reflect.Value, numArgs)

	// Resolve each argument
	for i, argType := range argTypes {
		value, err := d.provideValue(argType, ctx)
		if err != nil {
			return nil, err
		}
		args[i] = value
	}

	return fnValue.Call(args), nil
}

// createHandlerFunc generates an HTTP handler function that uses dependency injection
func createHandlerFunc(diContext *DIContext, handler interface{}) http.HandlerFunc {
	fnValue := reflect.ValueOf(handler)
	typeOf := fnValue.Type()
	numArgs := typeOf.NumIn()

	argTypes := make([]reflect.Type, numArgs)
	for i := 0; i < numArgs; i++ {
		argTypes[i] = typeOf.In(i)
	}

	// Precompute if we have direct HTTP injections
	hasHTTPRequestArg := make([]bool, numArgs)
	hasHTTPWriterArg := make([]bool, numArgs)
	writerInterface := reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()
	resolvedProviders := make([]Provider, numArgs)

	for i, argType := range argTypes {
		// Check for direct *http.Request injection
		if argType == reflect.TypeOf((*http.Request)(nil)) {
			hasHTTPRequestArg[i] = true
			continue
		}

		// Check for direct http.ResponseWriter injection
		if argType == writerInterface || (argType.Kind() == reflect.Interface && writerInterface.Implements(argType)) {
			hasHTTPWriterArg[i] = true
			continue
		}

		provider, err := diContext.resolveProvider(argType)
		if err != nil {
			continue
		}
		resolvedProviders[i] = provider
	}

	return func(w http.ResponseWriter, r *http.Request) {
		args := make([]reflect.Value, numArgs)

		// Handle direct HTTP injections
		for i := 0; i < numArgs; i++ {
			if hasHTTPRequestArg[i] {
				args[i] = reflect.ValueOf(r)
				continue
			}

			if hasHTTPWriterArg[i] {
				args[i] = reflect.ValueOf(w)
				continue
			}
		}

		// For remaining args, use diContext to resolve them
		for i := 0; i < numArgs; i++ {
			if !hasHTTPRequestArg[i] && !hasHTTPWriterArg[i] {
				var (
					value reflect.Value
					err   error
				)
				if resolvedProviders[i] != nil {
					value, err = resolvedProviders[i].Provide(argTypes[i], r.Context())
				} else {
					value, err = diContext.provideValue(argTypes[i], r.Context())
				}
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				args[i] = value
			}
		}

		fnValue.Call(args)
	}
}
