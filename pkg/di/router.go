package di

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
)

// Invoke creates a generic DI function that can be used for any function type
func Invoke(ctx context.Context, fn interface{}, customProviders ...Provider) ([]reflect.Value, error) {
	diContext := &DIContext{
		value:           reflect.ValueOf(fn),
		customProviders: customProviders,
	}
	return diContext.Invoke(ctx)
}

// H creates a dependency injection HTTP handler
func H(handler interface{}, customProviders ...Provider) http.HandlerFunc {
	diHandler := &DIHandler{
		value:           reflect.ValueOf(handler),
		customProviders: customProviders,
	}
	return diHandler.Handler()
}

// DIContext holds context for dependency injection
type DIContext struct {
	value           reflect.Value
	customProviders []Provider
}

// Invoke resolves dependencies and calls the function
func (d *DIContext) Invoke(ctx context.Context) ([]reflect.Value, error) {
	typeOf := d.value.Type()
	numArgs := typeOf.NumIn()

	argTypes := make([]reflect.Type, numArgs)
	for i := 0; i < numArgs; i++ {
		argTypes[i] = typeOf.In(i)
	}

	// All providers to try in order (custom first, then built-in)
	allProviders := append(d.customProviders, BuiltinProviders()...)

	matchedProviders := make([]Provider, numArgs)
	for i, argType := range argTypes {
		for _, provider := range allProviders {
			if provider.Ok(argType) {
				matchedProviders[i] = provider
				break
			}
		}

		if matchedProviders[i] == nil {
			return nil, fmt.Errorf("no provider found for type: %v", argType)
		}
	}

	args := make([]reflect.Value, numArgs)

	// Resolve each argument using precomputed matched providers
	for i, argType := range argTypes {
		provider := matchedProviders[i]
		value, err := provider.Provide(argType, ctx)
		if err != nil {
			return nil, err
		}
		args[i] = value
	}

	return d.value.Call(args), nil
}

// DIHandler is a handler that uses dependency injection to resolve its arguments
type DIHandler struct {
	value           reflect.Value
	customProviders []Provider
}

func (d *DIHandler) Handler() http.HandlerFunc {
	typeOf := d.value.Type()
	numArgs := typeOf.NumIn()

	argTypes := make([]reflect.Type, numArgs)
	for i := 0; i < numArgs; i++ {
		argTypes[i] = typeOf.In(i)
	}

	// All providers to try in order (custom first, then built-in)
	allProviders := append(d.customProviders, BuiltinProviders()...)

	matchedProviders := make([]Provider, numArgs)
	for i, argType := range argTypes {
		for _, provider := range allProviders {
			if provider.Ok(argType) {
				matchedProviders[i] = provider
				break
			}
		}

		if matchedProviders[i] == nil {
			// Return a handler that will return an error for this specific type
			errorMsg := fmt.Sprintf("No provider found for type: %v", argType)
			return func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, errorMsg, http.StatusInternalServerError)
			}
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		args := make([]reflect.Value, numArgs)

		// Check for HTTP response writer or request directly
		for i, argType := range argTypes {
			// Special case direct injection for *http.Request
			if argType == reflect.TypeOf((*http.Request)(nil)) {
				args[i] = reflect.ValueOf(r)
				continue
			}

			// Special case direct injection for http.ResponseWriter
			writerInterface := reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()
			if argType == writerInterface || (argType.Kind() == reflect.Interface && writerInterface.Implements(argType)) {
				args[i] = reflect.ValueOf(w)
				continue
			}

			// For other types, use provider system
			provider := matchedProviders[i]
			value, err := provider.Provide(argType, r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			args[i] = value
		}

		d.value.Call(args)
	}
}
