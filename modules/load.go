package modules

import (
	"embed"
	"encoding/json"
	"github.com/iota-agency/iota-sdk/modules/finance"
	"github.com/iota-agency/iota-sdk/modules/warehouse"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"github.com/iota-agency/iota-sdk/pkg/shared"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"slices"
)

//go:embed locales/*.json
var localeFiles embed.FS

var (
	AllModules = []shared.Module{
		finance.NewModule(),
		warehouse.NewModule(),
	}
)

func RegisterModule(module shared.Module) {
	AllModules = append(AllModules, module)
}

func Load() *ModuleRegistry {
	jsonConf := configuration.UseJsonConfig()
	registry := &ModuleRegistry{}
	for _, module := range AllModules {
		if slices.Contains(jsonConf.Modules, module.Name()) {
			// TODO: verbose logging
			registry.RegisterModules(module)
		}
	}
	return registry
}

func LoadBundle(registry *ModuleRegistry) *i18n.Bundle {
	bundle := i18n.NewBundle(language.Russian)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	localeDirs := append([]*embed.FS{&localeFiles}, registry.LocaleFiles()...)
	for _, localeFs := range localeDirs {
		files, err := localeFs.ReadDir("locales")
		if err != nil {
			panic(err)
		}
		for _, file := range files {
			if !file.IsDir() {
				localeFile, err := localeFs.ReadFile("locales/" + file.Name())
				if err != nil {
					panic(err)
				}
				bundle.MustParseMessageFileBytes(localeFile, file.Name())
			}
		}
	}
	return bundle
}
