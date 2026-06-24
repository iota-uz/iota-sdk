package export_test

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"

	"github.com/iota-uz/iota-sdk/components/export"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

// fakePageCtx is a minimal types.PageContext for rendering the dropdown in
// isolation; T echoes the key so assertions don't depend on a loaded bundle.
type fakePageCtx struct{}

func (fakePageCtx) T(key string, _ ...map[string]interface{}) string     { return key }
func (fakePageCtx) TSafe(key string, _ ...map[string]interface{}) string { return key }
func (f fakePageCtx) Namespace(string) types.PageContext                 { return f }
func (fakePageCtx) ToJSLocale() string                                   { return "en-US" }
func (fakePageCtx) GetLocale() language.Tag                              { return language.English }
func (fakePageCtx) GetURL() *url.URL                                     { return &url.URL{} }
func (fakePageCtx) GetLocalizer() *i18n.Localizer                        { return nil }

func renderDropdown(t *testing.T, props export.ExportDropdownProps) string {
	t.Helper()
	ctx := composables.WithPageCtx(context.Background(), fakePageCtx{})
	var sb strings.Builder
	require.NoError(t, export.ExportDropdown(props).Render(ctx, &sb))
	return sb.String()
}

// TestExportDropdown_DownloadMode verifies the GET-download variant: a real
// download trigger (no htmx POST, which is what 404'd against GET export
// routes), the export URL + params-form wiring, and the busy overlay.
func TestExportDropdown_DownloadMode(t *testing.T) {
	t.Parallel()

	html := renderDropdown(t, export.ExportDropdownProps{
		Formats:      []export.ExportFormat{export.ExportFormatExcel, export.ExportFormatCSV},
		ExportURL:    "/portfolio/policies/export",
		Download:     true,
		ParamsFormID: "filters-form",
	})

	// Download mode must NOT emit htmx attributes — those POST to the export URL
	// and 404 against GET-streaming routes, and can't save a binary body anyway.
	assert.NotContains(t, html, "hx-post", "download mode must not use htmx POST")
	assert.NotContains(t, html, `hx-swap="none"`)

	// GET-download wiring + token-cookie loader.
	assert.Contains(t, html, `data-export-url="/portfolio/policies/export"`)
	assert.Contains(t, html, `data-params-form="filters-form"`)
	assert.Contains(t, html, "runExport(")
	assert.Contains(t, html, "download_token")
	assert.Contains(t, html, "Export.Preparing", "busy overlay label must render")
	// Per-format triggers wired to runExport with each format param. Assert the
	// format args are present without pinning the exact HTML-entity encoding of
	// the surrounding quotes (an implementation artifact).
	assert.Regexp(t, `runExport\(\S*excel`, html)
	assert.Regexp(t, `runExport\(\S*csv`, html)
}

// TestExportDropdown_LegacyHtmxMode guards backward compatibility: the default
// (Download=false) still emits the htmx-POST behavior the MinIO/HX-Redirect
// export pages rely on.
func TestExportDropdown_LegacyHtmxMode(t *testing.T) {
	t.Parallel()

	html := renderDropdown(t, export.ExportDropdownProps{
		Formats:   []export.ExportFormat{export.ExportFormatExcel},
		ExportURL: "/clients/export",
	})

	assert.Contains(t, html, `hx-post="/clients/export?format=excel"`)
	assert.Contains(t, html, `hx-swap="none"`)
	assert.NotContains(t, html, "runExport(")
	assert.NotContains(t, html, "data-export-url")
}
