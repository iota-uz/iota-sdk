package composition

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/application"
)

func Adopt(module application.Module) Component {
	if module == nil {
		panic("composition: module is nil")
	}
	return adoptedModule{module: module}
}

type adoptedModule struct {
	module application.Module
}

func (m adoptedModule) Descriptor() Descriptor {
	return Descriptor{
		Name: m.module.Name(),
	}
}

func (m adoptedModule) Build(builder *Builder) error {
	if builder == nil {
		return fmt.Errorf("composition: builder is nil")
	}
	app := builder.Context().App
	if app == nil {
		return fmt.Errorf("composition: adopted module %q requires BuildContext.App", m.module.Name())
	}
	if err := m.module.RegisterWiring(app); err != nil {
		return err
	}
	if builder.Context().HasCapability(CapabilityAPI) {
		if err := m.module.RegisterTransports(app); err != nil {
			return err
		}
	}
	return nil
}
