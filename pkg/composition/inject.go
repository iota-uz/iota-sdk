package composition

import (
	"fmt"
	"reflect"
	"runtime"

	"github.com/iota-uz/iota-sdk/pkg/application"
)

// runtimeFuncName returns a human-readable name for the function value —
// used in error messages to point callers at the offending constructor.
func runtimeFuncName(v reflect.Value) string {
	if v.Kind() != reflect.Func || v.IsNil() {
		return "<unknown>"
	}
	fn := runtime.FuncForPC(v.Pointer())
	if fn == nil {
		return "<unknown>"
	}
	return fn.Name()
}

// ProvideFunc registers a provider whose factory is a constructor function
// taking typed dependencies as parameters. The engine resolves each parameter
// from the container by Go type at provider-instantiation time, using the
// constructor's return type as the provider key.
//
// Compared to Provide(builder, func(container *Container) (T, error) { ... }),
// ProvideFunc removes the manual `composition.Resolve[X](container)` unpacking
// for every dependency. Constructors with 5+ parameters become readable.
//
// Example:
//
//	composition.ProvideFunc(builder, services.NewPaymentService)
//
// where `services.NewPaymentService` has signature
// `func(*PaymentRepo, *MoneyAccountSvc, eventbus.EventBus) *PaymentService`.
//
// The constructor may optionally return an error as its second return value.
// All parameter types must be resolvable from the container at the time the
// provider runs; otherwise the provider returns the path-annotated NOT_PROVIDED
// error.
//
// Special parameter types:
//   - *Container is passed through directly (gives access to dynamic resolution)
//
// Panics at Build time if `constructor` is not a function or has the wrong
// shape.
//
// Use ProvideFuncAs[I] when the constructor returns a concrete type but you
// want the provider keyed under an interface.
func ProvideFunc(builder *Builder, constructor any) {
	if builder == nil {
		panic("composition: builder is nil")
	}
	caller, err := newInjectorCaller(constructor, nil)
	if err != nil {
		panic(fmt.Sprintf("composition: ProvideFunc: %v", err))
	}
	produced := caller.fnType.Out(0)
	key := keyFor(produced, "")
	entry := &providerEntry{
		key:           key,
		componentName: builder.descriptor.Name,
		capabilities:  append([]Capability(nil), builder.descriptor.Capabilities...),
		displayName:   key.DisplayName(),
		factory: func(container *Container) (any, error) {
			out, err := caller.call(container)
			if err != nil {
				return nil, err
			}
			return out[0], nil
		},
	}
	builder.providers = append(builder.providers, entry)
}

// ProvideFuncAs is the same as ProvideFunc but registers the constructor's
// return value under an explicit interface key I (in addition to the concrete
// key inferred from the constructor's return type). Useful when consumers
// depend on an interface and the constructor returns a concrete pointer.
//
// Panics at Build time if the constructor's return type is exactly I (which
// would result in a duplicate provider registration). Use ProvideFunc in
// that case instead.
func ProvideFuncAs[I any](builder *Builder, constructor any) {
	if builder == nil {
		panic("composition: builder is nil")
	}
	caller, err := newInjectorCaller(constructor, nil)
	if err != nil {
		panic(fmt.Sprintf("composition: ProvideFuncAs[%s]: %v", typeOf[I]().String(), err))
	}
	interfaceType := typeOf[I]()
	producedType := caller.fnType.Out(0)
	if producedType == interfaceType {
		panic(fmt.Sprintf(
			"composition: ProvideFuncAs[%s]: constructor already returns %s; use ProvideFunc instead",
			interfaceType, interfaceType,
		))
	}
	concreteKey := keyFor(producedType, "")
	concreteEntry := &providerEntry{
		key:           concreteKey,
		componentName: builder.descriptor.Name,
		capabilities:  append([]Capability(nil), builder.descriptor.Capabilities...),
		displayName:   concreteKey.DisplayName(),
		factory: func(container *Container) (any, error) {
			out, err := caller.call(container)
			if err != nil {
				return nil, err
			}
			return out[0], nil
		},
	}
	builder.providers = append(builder.providers, concreteEntry)

	// Bridge interface key to the same value via the concrete provider so
	// the constructor only runs once.
	appendProvider[I](builder, "", typeOf[I](), func(container *Container) (I, error) {
		raw, err := container.resolveAny(concreteKey)
		if err != nil {
			var zero I
			return zero, err
		}
		typed, ok := raw.(I)
		if !ok {
			var zero I
			return zero, fmt.Errorf("composition: %s does not implement %s", concreteKey, typeOf[I]())
		}
		return typed, nil
	})
}

// ContributeControllersFunc accepts a constructor whose parameters are typed
// services and whose return value is one or more controllers. The engine
// resolves each param from the container at materialization time.
//
// Supported return shapes:
//   - application.Controller
//   - []application.Controller
//   - (application.Controller, error)
//   - ([]application.Controller, error)
func ContributeControllersFunc(builder *Builder, constructor any) {
	if builder == nil {
		panic("composition: builder is nil")
	}
	caller, err := newInjectorCaller(constructor, controllerReturnTypes)
	if err != nil {
		panic(fmt.Sprintf("composition: ContributeControllersFunc: %v", err))
	}
	ContributeControllers(builder, func(container *Container) ([]application.Controller, error) {
		out, err := caller.call(container)
		if err != nil {
			return nil, err
		}
		return coerceControllers(out)
	})
}

// ----- internals -----

// injectorCaller stashes a reflected constructor and the resolution metadata
// needed to invoke it.
type injectorCaller struct {
	fn       reflect.Value
	fnType   reflect.Type
	paramKey []Key // length == NumIn; entries with empty Type signal a special parameter
}

var (
	containerType = reflect.TypeOf((*Container)(nil))
	errorType     = reflect.TypeOf((*error)(nil)).Elem()
)

// allowedReturn matchers
var controllerReturnTypes = []reflect.Type{
	reflect.TypeOf((*application.Controller)(nil)).Elem(),
	reflect.TypeOf([]application.Controller(nil)),
}

// newInjectorCaller validates the function shape and prepares it for repeated
// invocation. allowedNonErrReturns gates which non-error return types are
// acceptable; nil means "any single value".
//
// Variadic constructors are rejected at registration time. Silently dropping
// a trailing `opts ...Option` parameter is a footgun: a developer who later
// adds options expects them to take effect, but the injector always calls
// the constructor with an empty slice. Wrap variadic constructors in a
// non-variadic adapter that applies the desired options explicitly.
func newInjectorCaller(constructor any, allowedNonErrReturns []reflect.Type) (*injectorCaller, error) {
	if constructor == nil {
		return nil, fmt.Errorf("constructor is nil")
	}
	v := reflect.ValueOf(constructor)
	if v.Kind() != reflect.Func {
		return nil, fmt.Errorf("constructor must be a function, got %s", v.Kind())
	}
	t := v.Type()
	if t.IsVariadic() {
		return nil, fmt.Errorf(
			"variadic constructors are not supported by the reflection injector (function at %s); "+
				"wrap it in a non-variadic adapter that supplies the options explicitly",
			runtimeFuncName(v),
		)
	}

	// Return shape: must be (T) or (T, error). T is checked against the
	// allowed list when caller passed one.
	if t.NumOut() != 1 && t.NumOut() != 2 {
		return nil, fmt.Errorf("constructor must return one value or value+error, got %d", t.NumOut())
	}
	if t.NumOut() == 2 && !t.Out(1).Implements(errorType) {
		return nil, fmt.Errorf("constructor's second return value must be error, got %s", t.Out(1))
	}
	if len(allowedNonErrReturns) > 0 {
		ok := false
		for _, allowed := range allowedNonErrReturns {
			if t.Out(0) == allowed || (allowed.Kind() == reflect.Interface && t.Out(0).Implements(allowed)) {
				ok = true
				break
			}
		}
		if !ok {
			names := make([]string, len(allowedNonErrReturns))
			for i, a := range allowedNonErrReturns {
				names[i] = a.String()
			}
			return nil, fmt.Errorf("constructor return type %s not in allowed set %v", t.Out(0), names)
		}
	}

	numIn := t.NumIn()
	paramKeys := make([]Key, numIn)
	for i := 0; i < numIn; i++ {
		paramType := t.In(i)
		if paramType == containerType {
			// Marker — handled at call time.
			paramKeys[i] = Key{}
			continue
		}
		paramKeys[i] = keyFor(paramType, "")
	}

	return &injectorCaller{
		fn:       v,
		fnType:   t,
		paramKey: paramKeys,
	}, nil
}

// call resolves each parameter from the container and invokes the function.
// Returns the non-error result(s) on success, or an error.
func (c *injectorCaller) call(container *Container) ([]any, error) {
	args := make([]reflect.Value, len(c.paramKey))
	for i, key := range c.paramKey {
		if key.Type == nil {
			// Special parameter — currently only *Container.
			args[i] = reflect.ValueOf(container)
			continue
		}
		raw, err := container.resolveAny(key)
		if err != nil {
			return nil, err
		}
		args[i] = reflect.ValueOf(raw)
		if !args[i].IsValid() {
			// reflect.ValueOf(nil) yields the zero Value. That happens
			// when a provider explicitly returns nil — a valid pattern
			// for optional dependencies where downstream consumers
			// expect a nil-safe default. Fall back to the parameter's
			// typed zero so the Call below doesn't panic with
			// "reflect: Call using zero Value as ...".
			args[i] = reflect.Zero(c.fnType.In(i))
		}
	}

	out := c.fn.Call(args)
	if c.fnType.NumOut() == 2 && !out[1].IsNil() {
		return nil, out[1].Interface().(error)
	}
	return []any{out[0].Interface()}, nil
}

func coerceControllers(out []any) ([]application.Controller, error) {
	if len(out) == 0 {
		return nil, nil
	}
	switch v := out[0].(type) {
	case nil:
		return nil, nil
	case application.Controller:
		if v == nil {
			return nil, nil
		}
		return []application.Controller{v}, nil
	case []application.Controller:
		// Drop nil entries — some constructors return nil to signal "this
		// controller is disabled in this build" (e.g. showcase_controller_prod).
		filtered := make([]application.Controller, 0, len(v))
		for _, c := range v {
			if c != nil {
				filtered = append(filtered, c)
			}
		}
		return filtered, nil
	default:
		return nil, fmt.Errorf("composition: ContributeControllersFunc: unsupported return type %T", v)
	}
}
