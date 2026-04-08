// Package modules provides this package.
package modules

import (
	"slices"

	"github.com/iota-uz/iota-sdk/modules/billing"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/crm"
	"github.com/iota-uz/iota-sdk/modules/finance"
	"github.com/iota-uz/iota-sdk/modules/hrm"
	"github.com/iota-uz/iota-sdk/modules/logging"
	"github.com/iota-uz/iota-sdk/modules/oidc"
	"github.com/iota-uz/iota-sdk/modules/projects"
	"github.com/iota-uz/iota-sdk/modules/testkit"
	"github.com/iota-uz/iota-sdk/modules/warehouse"
	"github.com/iota-uz/iota-sdk/modules/website"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
)

var (
	coreModuleOptions = &core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}

	// NOTE: bichat.NavItems is intentionally excluded from default NavLinks.
	// The BiChat module registers its own nav items when loaded (requires OPENAI_API_KEY).
	// Including them here would cause translation errors when the module is not loaded.
	NavLinks = slices.Concat(
		core.BuildNavItems(coreModuleOptions.DashboardLinkPermissions, coreModuleOptions.SettingsLinkPermissions),
		hrm.NavItems,
		finance.NavItems,
		projects.NavItems,
		warehouse.NavItems,
		crm.NavItems,
		website.NavItems,
	)
)

func Components() []composition.Component {
	return []composition.Component{
		core.NewComponent(coreModuleOptions),
		hrm.NewComponent(),
		finance.NewComponent(),
		projects.NewComponent(),
		logging.NewComponent(),
		warehouse.NewComponent(),
		crm.NewComponent(),
		website.NewComponent(),
		billing.NewComponent(),
		oidc.NewComponent(&oidc.ModuleOptions{}),
		testkit.NewComponent(),
	}
}
