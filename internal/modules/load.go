package modules

import (
	"encoding/json"
	"github.com/iota-agency/iota-erp/elxolding"
	"github.com/iota-agency/iota-erp/internal/configuration"
	"github.com/iota-agency/iota-erp/internal/modules/finance"
	"github.com/iota-agency/iota-erp/internal/modules/shared"
	"github.com/iota-agency/iota-erp/internal/modules/warehouse"
	"github.com/iota-agency/iota-erp/iota"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"slices"
)

var (
	AllModules = []shared.Module{
		iota.NewModule(),
		elxolding.NewModule(),
		finance.NewModule(),
		warehouse.NewModule(),
	}
)

func Load() *ModuleRegistry {
	jsonConf := configuration.UseJsonConfig()
	registry := &ModuleRegistry{}
	for _, module := range AllModules {
		if slices.Contains(jsonConf.Modules, module.Name()) {
			registry.RegisterModules(module)
		}
	}
	return registry
}

func LoadBundle(registry *ModuleRegistry) *i18n.Bundle {
	bundle := i18n.NewBundle(language.Russian)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	bundle.MustLoadMessageFile("pkg/locales/en.json")
	bundle.MustLoadMessageFile("pkg/locales/ru.json")
	for _, localeFile := range registry.localeFiles {
		bundle.MustLoadMessageFile(localeFile)
	}
	return bundle
}
