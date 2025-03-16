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

var builtinProviders = []Provider{
	func(t reflect.Type, w http.ResponseWriter, r *http.Request) (reflect.Value, bool, error) {
		pageCtxType := reflect.TypeOf((*types.PageContext)(nil))
		if t == pageCtxType {
			return reflect.ValueOf(composables.UsePageCtx(r.Context())), true, nil
		}
		return reflect.Value{}, false, nil
	},

	func(t reflect.Type, w http.ResponseWriter, r *http.Request) (reflect.Value, bool, error) {
		writerType := reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()
		if t.Implements(writerType) {
			return reflect.ValueOf(w), true, nil
		}
		return reflect.Value{}, false, nil
	},

	func(t reflect.Type, w http.ResponseWriter, r *http.Request) (reflect.Value, bool, error) {
		requestType := reflect.TypeOf((*http.Request)(nil))
		if t == requestType {
			return reflect.ValueOf(r), true, nil
		}
		return reflect.Value{}, false, nil
	},

	func(t reflect.Type, w http.ResponseWriter, r *http.Request) (reflect.Value, bool, error) {
		localizerType := reflect.TypeOf((*i18n.Localizer)(nil))
		if t == localizerType {
			localizer, ok := composables.UseLocalizer(r.Context())
			if !ok {
				return reflect.Value{}, true, fmt.Errorf("localizer not found in request context")
			}
			return reflect.ValueOf(localizer), true, nil
		}
		return reflect.Value{}, false, nil
	},

	func(t reflect.Type, w http.ResponseWriter, r *http.Request) (reflect.Value, bool, error) {
		userType := reflect.TypeOf((*user.User)(nil)).Elem()
		if t.Implements(userType) {
			u, err := composables.UseUser(r.Context())
			if err != nil {
				return reflect.Value{}, true, fmt.Errorf("user not found in request context")
			}
			return reflect.ValueOf(u), true, nil
		}
		return reflect.Value{}, false, nil
	},

	func(t reflect.Type, w http.ResponseWriter, r *http.Request) (reflect.Value, bool, error) {
		appType := reflect.TypeOf((*application.Application)(nil)).Elem()
		if t.Implements(appType) {
			app, err := composables.UseApp(r.Context())
			if err != nil {
				return reflect.Value{}, true, err
			}
			return reflect.ValueOf(app), true, nil
		}
		return reflect.Value{}, false, nil
	},

	func(t reflect.Type, w http.ResponseWriter, r *http.Request) (reflect.Value, bool, error) {
		app, err := composables.UseApp(r.Context())
		if err != nil {
			return reflect.Value{}, false, err
		}

		services := app.Services()
		if service, exists := services[t.Elem()]; exists {
			return reflect.ValueOf(service), true, nil
		}

		return reflect.Value{}, false, nil
	},
}

// Provider is a function that can provide a value for a given type
// It returns:
// - The value (if it can provide it)
// - A boolean indicating whether this provider can handle the requested type
// - Any error that occurred during resolution
type Provider func(t reflect.Type, w http.ResponseWriter, r *http.Request) (reflect.Value, bool, error)

func NewDIHandler(handler interface{}, customProviders ...Provider) *DIHandler {
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

	// Precompute argument types and their resolvers
	argTypes := make([]reflect.Type, numArgs)
	for i := 0; i < numArgs; i++ {
		argTypes[i] = typeOf.In(i)
	}

	// All providers to try in order (custom first, then built-in)
	allProviders := append(d.customProviders, builtinProviders...)

	return func(w http.ResponseWriter, r *http.Request) {
		args := make([]reflect.Value, numArgs)

		// Resolve each argument
		for i, argType := range argTypes {
			var resolved bool
			var err error

			// Try each provider in order
			for _, provider := range allProviders {
				args[i], resolved, err = provider(argType, w, r)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if resolved {
					break
				}
			}

			if !resolved {
				http.Error(w, fmt.Sprintf("No provider found for type: %v", argType), http.StatusInternalServerError)
				return
			}
		}

		d.value.Call(args)
	}
}
