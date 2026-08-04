package composition

import (
	"errors"

	"github.com/iota-uz/iota-sdk/pkg/config"
)

// ProvideConfig loads T at prefix from the builder's config.Source (attached
// via bootstrap.WithSource), validates it (if T implements config.Validatable),
// and registers the resulting *T into the DI container so that any constructor
// taking *T as a parameter receives the loaded value via auto-wiring.
//
// Returns an error if no Source is attached to the builder's BuildContext, or
// if the underlying config.Register fails (unmarshal or validation). Callers
// that need a Source must pass bootstrap.WithSource(...) when building the
// Runtime.
func ProvideConfig[T any](b *Builder, prefix string) error {
	if b == nil {
		return errors.New("composition.ProvideConfig: builder is nil")
	}
	src := b.context.Source()
	if src == nil {
		return errors.New("composition.ProvideConfig: no config.Source is attached to the BuildContext; use bootstrap.WithSource(...) when constructing the Runtime")
	}

	// Lazy-init a shared Registry on the BuildContext (pointer receiver required
	// for mutation — b.context is addressable as a struct field).
	reg := (&b.context).Registry()

	ptr, err := config.RegisterAt[T](reg, prefix)
	if err != nil {
		return err
	}

	// Register the pointer into the DI container so constructors receive *T.
	Provide[*T](b, ptr)
	return nil
}
