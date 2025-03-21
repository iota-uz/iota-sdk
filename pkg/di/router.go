package di

import (
	"fmt"
	"net/http"
	"reflect"
)

func NewHandler(handler interface{}, customProviders ...Provider) *DIHandler {
	return &DIHandler{
		value:           reflect.ValueOf(handler),
		customProviders: customProviders,
	}
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

		// Resolve each argument using precomputed matched providers
		for i, argType := range argTypes {
			provider := matchedProviders[i]
			value, err := provider.Provide(argType, w, r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			args[i] = value
		}

		d.value.Call(args)
	}
}
