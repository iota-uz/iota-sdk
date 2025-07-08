package controllers

import (
	"github.com/iota-uz/go-i18n/v2/i18n"
	importpkg "github.com/iota-uz/iota-sdk/pkg/import"
)

// PositionImportConfig implements ImportPageConfig for warehouse positions
type PositionImportConfig struct {
	*importpkg.BaseImportPageConfig
}

// NewPositionImportConfig creates a new configuration for position imports
func NewPositionImportConfig() *PositionImportConfig {
	return newPositionImportConfig(nil)
}

// NewPositionImportConfigWithLocalizer creates a new configuration with translations
func NewPositionImportConfigWithLocalizer(localizer *i18n.Localizer) *PositionImportConfig {
	return newPositionImportConfig(localizer)
}

func newPositionImportConfig(localizer *i18n.Localizer) *PositionImportConfig {
	config := &PositionImportConfig{
		BaseImportPageConfig: importpkg.NewBaseImportPageConfig(),
	}

	// Helper function to translate or return key
	t := func(key string) string {
		if localizer != nil {
			translated, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: key})
			if err == nil {
				return translated
			}
		}
		return key
	}

	// Configure page settings
	config.Title = t("WarehousePositions.Import.Title")
	config.Description = t("WarehousePositions.Import._Description")
	config.SaveURL = "/warehouse/positions/import"
	config.LocalePrefix = "WarehousePositions.Import"
	config.TemplateDownloadURL = "/warehouse/positions/template.xlsx"

	// Configure columns
	config.Columns = []importpkg.ImportColumn{
		{
			Header:      t("WarehousePositions.Import.Example.ItemName"),
			Description: t("WarehousePositions.Import.Example.ItemNameDesc"),
			Required:    true,
		},
		{
			Header:      t("WarehousePositions.Import.Example.ItemCode"),
			Description: t("WarehousePositions.Import.Example.ItemCodeDesc"),
			Required:    true,
		},
		{
			Header:      t("WarehousePositions.Import.Example.Unit"),
			Description: t("WarehousePositions.Import.Example.UnitDesc"),
			Required:    true,
		},
		{
			Header:      t("WarehousePositions.Import.Example.Quantity"),
			Description: t("WarehousePositions.Import.Example.QuantityDesc"),
			Required:    true,
		},
	}

	// Configure example rows
	config.ExampleRows = [][]string{
		{"Дрель Молоток N.C.V (900W)", "30232478", "шт", "1"},
		{"Дрель Ударная (650W)", "30232477", "шт", "1"},
		{"Комплект плакатов по предмету \"Математика\", 40 листов", "00017492", "компл", "7"},
		{"Комплект плакатов цветных по \"Технике безопасности\" (500x700мм, 5 листов) на туркменском", "00028544", "компл", "127"},
	}

	// Configure HTMX settings
	config.HTMXConfig = importpkg.HTMXConfig{
		Target:    "#import-content",
		Swap:      "outerHTML",
		Indicator: "#save-btn",
	}

	return config
}
