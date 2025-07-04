package importpkg

// BaseImportPageConfig provides default implementation
type BaseImportPageConfig struct {
	Title               string
	Description         string
	Columns             []ImportColumn
	ExampleRows         [][]string
	SaveURL             string
	AcceptedFileTypes   string
	LocalePrefix        string
	TemplateDownloadURL string
	HTMXConfig          HTMXConfig
}

// Implement ImportPageConfig interface
func (c *BaseImportPageConfig) GetTitle() string               { return c.Title }
func (c *BaseImportPageConfig) GetDescription() string         { return c.Description }
func (c *BaseImportPageConfig) GetColumns() []ImportColumn     { return c.Columns }
func (c *BaseImportPageConfig) GetExampleRows() [][]string     { return c.ExampleRows }
func (c *BaseImportPageConfig) GetSaveURL() string             { return c.SaveURL }
func (c *BaseImportPageConfig) GetAcceptedFileTypes() string   { return c.AcceptedFileTypes }
func (c *BaseImportPageConfig) GetLocalePrefix() string        { return c.LocalePrefix }
func (c *BaseImportPageConfig) GetTemplateDownloadURL() string { return c.TemplateDownloadURL }
func (c *BaseImportPageConfig) GetHTMXConfig() HTMXConfig      { return c.HTMXConfig }

// NewBaseImportPageConfig creates a new import page configuration with sensible defaults
func NewBaseImportPageConfig() *BaseImportPageConfig {
	return &BaseImportPageConfig{
		AcceptedFileTypes: "text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		HTMXConfig: HTMXConfig{
			Target:    "#import-content",
			Swap:      "innerHTML",
			Indicator: "#save-btn",
		},
	}
}
