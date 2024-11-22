package modules

import (
	"github.com/iota-agency/iota-sdk/modules/finance"
	"github.com/iota-agency/iota-sdk/modules/warehouse"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"slices"
)

var (
	AllModules = []application.Module{
		finance.NewModule(),
		warehouse.NewModule(),
	}
)

func Load() []application.Module {
	jsonConf := configuration.UseJsonConfig()
	var result []application.Module
	for _, module := range AllModules {
		if slices.Contains(jsonConf.Modules, module.Name()) {
			result = append(result, module)
		}
	}
	return result
}
