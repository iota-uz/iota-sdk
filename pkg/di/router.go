package di

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sync"
)

// Invoke creates a generic DI function that can be used for any function type
func Invoke(ctx context.Context, fn interface{}, customProviders ...Provider) ([]reflect.Value, error) {
	diContext := NewDIContext(customProviders...)
	return diContext.Invoke(ctx, fn)
}

// H creates a dependency injection HTTP handler
func H(handler interface{}, customProviders ...Provider) http.HandlerFunc {
	diContext := NewDIContext(customProviders...)
	return createHandlerFunc(diContext, handler)
}

// DIContext holds context for dependency injection
type DIContext struct {
	customProviders  []Provider
	allProviders     []Provider
	matchedProviders map[reflect.Type]Provider
	mu               sync.RWMutex
}

// NewDIContext creates a new DIContext with custom providers
func NewDIContext(customProviders ...Provider) *DIContext {
	allProviders := append([]Provider{}, customProviders...)
	allProviders = append(allProviders, BuiltinProviders()...)

	return &DIContext{
		customProviders:  customProviders,
		allProviders:     allProviders,
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
	provider, err := d.resolveProvider(argType)
	if err != nil {
		return reflect.Value{}, err
	}
	return provider.Provide(argType, ctx)
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
	hasHttpRequestArg := make([]bool, numArgs)
	hasHttpWriterArg := make([]bool, numArgs)
	writerInterface := reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()
	resolvedProviders := make([]Provider, numArgs)

	for i, argType := range argTypes {
		// Check for direct *http.Request injection
		if argType == reflect.TypeOf((*http.Request)(nil)) {
			hasHttpRequestArg[i] = true
			continue
		}

		// Check for direct http.ResponseWriter injection
		if argType == writerInterface || (argType.Kind() == reflect.Interface && writerInterface.Implements(argType)) {
			hasHttpWriterArg[i] = true
			continue
		}

		provider, err := diContext.resolveProvider(argType)
		if err != nil {
			// Return a handler that will return an error for this specific type
			errorMsg := err.Error()
			return func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, errorMsg, http.StatusInternalServerError)
			}
		}
		resolvedProviders[i] = provider
	}

	return func(w http.ResponseWriter, r *http.Request) {
		args := make([]reflect.Value, numArgs)

		// Handle direct HTTP injections
		for i := 0; i < numArgs; i++ {
			if hasHttpRequestArg[i] {
				args[i] = reflect.ValueOf(r)
				continue
			}

			if hasHttpWriterArg[i] {
				args[i] = reflect.ValueOf(w)
				continue
			}
		}

		// For remaining args, use diContext to resolve them
		for i := 0; i < numArgs; i++ {
			if !hasHttpRequestArg[i] && !hasHttpWriterArg[i] {
				value, err := resolvedProviders[i].Provide(argTypes[i], r.Context())
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
