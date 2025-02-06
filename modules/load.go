package modules

import (
	"github.com/iota-uz/iota-sdk/modules/bichat"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/crm"
	"github.com/iota-uz/iota-sdk/modules/finance"
	"github.com/iota-uz/iota-sdk/modules/warehouse"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

var (
	BuiltInModules = []application.Module{
		core.NewModule(),
		bichat.NewModule(),
		finance.NewModule(),
		warehouse.NewModule(),
		crm.NewModule(),
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
