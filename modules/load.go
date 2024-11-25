package modules

import (
	"slices"

	"github.com/iota-agency/iota-sdk/modules/finance"
	"github.com/iota-agency/iota-sdk/modules/warehouse"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
)

var (
	BuiltInModules = []application.Module{
		finance.NewModule(),
		warehouse.NewModule(),
	}
)

func Load(externalModules ...application.Module) []application.Module {
	jsonConf := configuration.UseJsonConfig()
	var result []application.Module
	modules := append(BuiltInModules, externalModules...)
	for _, module := range modules {
		if slices.Contains(jsonConf.Modules, module.Name()) {
			result = append(result, module)
		}
	}
	return result
}
