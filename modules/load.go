package modules

import (
	"slices"

	"github.com/iota-uz/iota-sdk/modules/billing"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/crm"
	"github.com/iota-uz/iota-sdk/modules/finance"
	"github.com/iota-uz/iota-sdk/modules/hrm"
	"github.com/iota-uz/iota-sdk/modules/logging"
	"github.com/iota-uz/iota-sdk/modules/projects"
	"github.com/iota-uz/iota-sdk/modules/testkit"
	"github.com/iota-uz/iota-sdk/modules/warehouse"
	"github.com/iota-uz/iota-sdk/modules/website"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
)

var (
	BuiltInModules = []application.Module{
		core.NewModule(&core.ModuleOptions{
			PermissionSchema: defaults.PermissionSchema(),
		}),
		hrm.NewModule(),
		finance.NewModule(),
		projects.NewModule(),
		logging.NewModule(),
		warehouse.NewModule(),
		crm.NewModule(),
		website.NewModule(),
		billing.NewModule(),
		testkit.NewModule(), // Test endpoints - only active when ENABLE_TEST_ENDPOINTS=true
	}

	// NOTE: bichat.NavItems is intentionally excluded from default NavLinks.
	// The BiChat module registers its own nav items when loaded (requires OPENAI_API_KEY).
	// Including them here would cause translation errors when the module is not loaded.
	NavLinks = slices.Concat(
		core.NavItems,
		hrm.NavItems,
		finance.NavItems,
		projects.NavItems,
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
