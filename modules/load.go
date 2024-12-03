package modules

import (
	"github.com/iota-agency/iota-sdk/modules/finance"
	"github.com/iota-agency/iota-sdk/modules/warehouse"
	"github.com/iota-agency/iota-sdk/pkg/application"
)

var (
	BuiltInModules = []application.Module{
		finance.NewModule(),
		warehouse.NewModule(),
	}
)

func Load(app application.Application, externalModules ...application.Module) error {
	for _, module := range externalModules {
		if err := module.Register(app); err != nil {
			return err
		}
	}
	return nil
}
