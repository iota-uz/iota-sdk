package modules

import (
	"github.com/iota-uz/iota-sdk/modules/billing"
	"slices"

	"github.com/iota-uz/iota-sdk/modules/bichat"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/crm"
	"github.com/iota-uz/iota-sdk/modules/finance"
	"github.com/iota-uz/iota-sdk/modules/hrm"
	"github.com/iota-uz/iota-sdk/modules/logging"
	"github.com/iota-uz/iota-sdk/modules/warehouse"
	"github.com/iota-uz/iota-sdk/modules/website"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

var (
	BuiltInModules = []application.Module{
		core.NewModule(),
		bichat.NewModule(),
		hrm.NewModule(),
		finance.NewModule(),
		logging.NewModule(),
		warehouse.NewModule(),
		crm.NewModule(),
		website.NewModule(),
		billing.NewModule(),
	}

	NavLinks = slices.Concat(
		core.NavItems,
		bichat.NavItems,
		hrm.NavItems,
		finance.NavItems,
		warehouse.NavItems,
		crm.NavItems,
		website.NavItems,
	)
)

func Load(app application.Application, externalModules ...application.Module) error {
	for _, module := range externalModules {
		if err := module.Register(app); err != nil {
			return err
		}
	}
	return nil
}
