package composition

import (
	"fmt"
	"reflect"
)

// typedContribution is one Contribute[T] factory; typ is T.
type typedContribution struct {
	typ reflect.Type
	namedFactory[[]any]
}

// Contribute adds values of T that any component can list with
// Contributions[T] once the container is compiled. It is the open-ended
// counterpart of the fixed Contribute* helpers (controllers, hooks, help
// sources): a package that owns an extension point declares its own typed
// wrapper over it, for example
//
//	func ContributeConsumers(b *composition.Builder, f func(*composition.Container) ([]events.Consumer, error)) {
//	    composition.Contribute(b, f)
//	}
//
// The factories run once, in component registration order, after every
// provider is instantiated, so they may resolve any provider of the container.
func Contribute[T any](builder *Builder, factory func(*Container) ([]T, error)) {
	if builder == nil {
		panic("composition: builder is nil")
	}
	if factory == nil {
		return
	}
	typ := typeOf[T]()
	builder.contributions = append(builder.contributions, typedContribution{
		typ: typ,
		namedFactory: namedFactory[[]any]{
			component: builder.descriptor.Name,
			label:     "contributions of " + typeName(typ),
			factory: func(c *Container) ([]any, error) {
				values, err := factory(c)
				if err != nil {
					return nil, err
				}
				out := make([]any, len(values))
				for i, v := range values {
					out[i] = v
				}
				return out, nil
			},
		},
	})
}

// Contributions lists every value contributed with Contribute[T], in
// component registration order. The values are collected when the container
// is compiled, after its providers are instantiated: a provider that needs
// them reads them lazily (at Start or per call), never from its constructor,
// where Contributions returns an error.
func Contributions[T any](container *Container) ([]T, error) {
	if container == nil {
		return nil, fmt.Errorf("composition: container is nil")
	}
	typ := typeOf[T]()
	if !container.contributionsCollected {
		return nil, fmt.Errorf("composition: contributions of %s are read before the container is compiled", typeName(typ))
	}
	values := container.contributions[typ]
	out := make([]T, 0, len(values))
	for _, v := range values {
		out = append(out, v.(T))
	}
	return out, nil
}

func (c *Container) collectContributions() error {
	c.contributions = make(map[reflect.Type][]any)
	for _, entry := range c.contributionFactories {
		previousPath := c.resolutionPath
		c.resolutionPath = []string{entry.component, entry.label}
		values, err := entry.factory(c)
		c.resolutionPath = previousPath
		if err != nil {
			return fmt.Errorf("composition: %s for %q: %w", entry.label, entry.component, err)
		}
		c.contributions[entry.typ] = append(c.contributions[entry.typ], values...)
	}
	c.contributionsCollected = true
	return nil
}
