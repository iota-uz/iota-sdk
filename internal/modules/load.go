package modules

import (
	"github.com/iota-agency/iota-erp/internal/configuration"
	"github.com/iota-agency/iota-erp/internal/modules/elxolding"
	"github.com/iota-agency/iota-erp/internal/modules/iota"
	"github.com/iota-agency/iota-erp/internal/modules/shared"
	"slices"
)

var AllModules = []shared.Module{
	iota.NewModule(),
	elxolding.NewModule(),
}

func Load() []shared.Module {
	jsonConf := configuration.UseJsonConfig()
	modules := make([]shared.Module, 0, len(AllModules))
	for _, module := range AllModules {
		if slices.Contains(jsonConf.Modules, module.Name()) {
			modules = append(modules, module)
		}
	}
	return modules
}
