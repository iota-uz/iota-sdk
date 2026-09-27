// Package importpkg provides this package.
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
	// SubmitLabel overrides the primary action button text. Empty falls back to
	// the generic "Submit" translation. A two-step preview→confirm importer sets
	// this to e.g. "Preview" so the button reflects that the first submit is a
	// dry run, not a commit.
	SubmitLabel string
	// SubmitHint is optional muted text shown beside the submit button (e.g.
	// "A preview is shown before any data is changed"). Empty renders nothing.
	SubmitHint string
}

// GetTitle returns the page title from the import configuration.
func (c *BaseImportPageConfig) GetTitle() string               { return c.Title }
func (c *BaseImportPageConfig) GetDescription() string         { return c.Description }
func (c *BaseImportPageConfig) GetColumns() []ImportColumn     { return c.Columns }
func (c *BaseImportPageConfig) GetExampleRows() [][]string     { return c.ExampleRows }
func (c *BaseImportPageConfig) GetSaveURL() string             { return c.SaveURL }
func (c *BaseImportPageConfig) GetAcceptedFileTypes() string   { return c.AcceptedFileTypes }
func (c *BaseImportPageConfig) GetLocalePrefix() string        { return c.LocalePrefix }
func (c *BaseImportPageConfig) GetTemplateDownloadURL() string { return c.TemplateDownloadURL }
func (c *BaseImportPageConfig) GetHTMXConfig() HTMXConfig      { return c.HTMXConfig }
func (c *BaseImportPageConfig) GetSubmitLabel() string         { return c.SubmitLabel }
func (c *BaseImportPageConfig) GetSubmitHint() string          { return c.SubmitHint }

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
