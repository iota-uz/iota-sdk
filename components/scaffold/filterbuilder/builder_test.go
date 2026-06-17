package filterbuilder

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/filterq"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

// stubPageCtx satisfies types.PageContext returning the key itself for every
// translation, which is enough to assert markup structure.
type stubPageCtx struct{}

func (stubPageCtx) T(key string, _ ...map[string]interface{}) string     { return key }
func (stubPageCtx) TSafe(key string, _ ...map[string]interface{}) string { return key }
func (s stubPageCtx) Namespace(string) types.PageContext                 { return s }
func (stubPageCtx) ToJSLocale() string                                   { return "en-US" }
func (stubPageCtx) GetLocale() language.Tag                              { return language.English }
func (stubPageCtx) GetURL() *url.URL                                     { return &url.URL{Path: "/"} }
func (stubPageCtx) GetLocalizer() *i18n.Localizer                        { return nil }

func renderCtx() context.Context {
	return composables.WithPageCtx(context.Background(), stubPageCtx{})
}

func testRegistry() *Registry {
	return NewRegistry(
		FieldDef{Key: "status", Type: filterq.FieldTypeReference, Label: "Status", Group: "References", Options: []Option{
			{Value: "1", Label: "Active", Count: 120},
			{Value: "2", Label: "Annulled", Count: 0, Disabled: true},
		}},
		FieldDef{Key: "seria", Type: filterq.FieldTypeReference, Label: "Seria", Group: "References", Options: []Option{
			{Value: "EEIU", Label: "EEIU", Count: 12403, Group: "Mandatory"},
			{Value: "GBO", Label: "GBO", Count: 0, Disabled: true, Group: "Voluntary"},
		}},
		FieldDef{Key: "issue_at", Type: filterq.FieldTypeDate, Label: "Issue date", Group: "Dates"},
		FieldDef{Key: "premium", Type: filterq.FieldTypeNumber, Label: "Premium", Group: "Numbers"},
		FieldDef{Key: "reissued", Type: filterq.FieldTypeBool, Label: "Reissued only", Group: "Flags"},
	)
}

func render(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf strings.Builder
	require.NoError(t, c.Render(renderCtx(), &buf))
	return buf.String()
}

func renderComponent(t *testing.T, p Props, oob bool) string {
	t.Helper()
	var buf strings.Builder
	if oob {
		require.NoError(t, BuilderOOB(p).Render(renderCtx(), &buf))
	} else {
		require.NoError(t, Builder(p).Render(renderCtx(), &buf))
	}
	return buf.String()
}

func TestBuilderRendersChipsAndHiddenInputs(t *testing.T) {
	fs := filterq.FilterSet{
		{Field: "status", Op: filterq.OpIs, Values: []string{"1", "2"}},
		{Field: "issue_at", Op: filterq.OpBetween, Values: []string{"preset:this_year"}},
		{Field: "reissued", Op: filterq.OpIs, Values: []string{"true"}},
	}
	html := renderComponent(t, Props{Registry: testRegistry(), Filters: fs}, false)

	// Presence marker always submits.
	assert.Contains(t, html, `name="fb" value="1"`)
	// One hidden codec input per chip, indexed for Alpine edits.
	assert.Contains(t, html, `name="f" value="status:is:1,2" data-fb-chip="0"`)
	assert.Contains(t, html, `name="f" value="issue_at:between:preset:this_year" data-fb-chip="1"`)
	assert.Contains(t, html, `name="f" value="reissued:is:true" data-fb-chip="2"`)
	// Chip summaries resolve option labels and preset locale keys.
	assert.Contains(t, html, "Active, Annulled")
	assert.Contains(t, html, "Scaffold.FilterBuilder.Presets.ThisYear")
	// Alpine root + add button.
	assert.Contains(t, html, `x-data="filterBuilder"`)
	assert.Contains(t, html, "Scaffold.FilterBuilder.AddFilter")
	// Clear all appears with >1 chip.
	assert.Contains(t, html, "Scaffold.FilterBuilder.ClearAll")
}

func TestDateEditorEmitsExplicitFlatpickrFormat(t *testing.T) {
	// Regression for eai#3080: the date editor's pickers (single + range) must
	// pass an explicit DateFormat. An empty value serializes as "" and the
	// datePicker Alpine component falls back to the bogus 'z' flatpickr token
	// (dateFormat || 'z'), so the hidden input holds a value filterq.Decode
	// cannot time.Parse against DateLayout (2006-01-02). The condition is then
	// silently dropped and the date filter never applies. "Y-m-d" == DateLayout.
	html := renderComponent(t, Props{Registry: testRegistry()}, false)

	// The date field ("issue_at") draft editor renders flatpickr date pickers...
	assert.Contains(t, html, "datePicker(")
	// ...and they must declare the Y-m-d format, never the empty 'z'-fallback.
	assert.Contains(t, html, "Y-m-d")
	assert.NotContains(t, html, `dateFormat&#34;:&#34;&#34;`)
}

func TestBuilderUnknownFieldChipSkipped(t *testing.T) {
	fs := filterq.FilterSet{{Field: "ghost", Op: filterq.OpIs, Values: []string{"1"}}}
	html := renderComponent(t, Props{Registry: testRegistry(), Filters: fs}, false)
	assert.NotContains(t, html, "ghost")
}

func TestBuilderOOBMarksSwap(t *testing.T) {
	html := renderComponent(t, Props{Registry: testRegistry()}, true)
	assert.Contains(t, html, `hx-swap-oob="outerHTML"`)
	assert.Contains(t, html, `id="filter-builder"`)

	// Non-OOB render has no swap marker.
	plain := renderComponent(t, Props{Registry: testRegistry()}, false)
	assert.NotContains(t, plain, "hx-swap-oob")
}

func TestEditorControlsAreNameless(t *testing.T) {
	fs := filterq.FilterSet{
		{Field: "status", Op: filterq.OpIs, Values: []string{"1"}},
		{Field: "issue_at", Op: filterq.OpOn, Values: []string{"2026-06-01"}},
		{Field: "premium", Op: filterq.OpBetween, Values: []string{"100", "200"}},
	}
	html := renderComponent(t, Props{Registry: testRegistry(), Filters: fs}, false)

	// The only named controls are the codec inputs and the presence marker:
	// editor internals must never serialize into the HTMX form request.
	for _, frag := range strings.Split(html, "name=\"")[1:] {
		name := frag[:strings.Index(frag, "\"")]
		assert.Contains(t, []string{"f", "fb", ""}, name, "unexpected named control %q", name)
	}
}

func TestGroupedOptionsMarkup(t *testing.T) {
	field, _ := testRegistry().Field("seria")
	html := render(t, GroupedOptions(field.Options, []string{"EEIU"}))

	// Group headers are disabled marker options.
	assert.Contains(t, html, `disabled data-fb-header="1" value="__group:Mandatory"`)
	// Counts travel via data attribute; selection is server-marked.
	assert.Contains(t, html, `value="EEIU" selected data-fb-count="12403"`)
	// Zero-count options are disabled (dimmed).
	assert.Contains(t, html, `value="GBO" disabled data-fb-count="0"`)
}

func TestRegistryDecode(t *testing.T) {
	q := url.Values{filterq.ParamName: []string{"status:is:1", "bogus:is:1"}}
	fs := testRegistry().Decode(q)
	require.Len(t, fs, 1)
	assert.Equal(t, "status", fs[0].Field)
}

func TestChipValueSummaryCollapsesLongLists(t *testing.T) {
	reg := NewRegistry(FieldDef{Key: "r", Type: filterq.FieldTypeReference, Label: "R", Options: []Option{
		Opt("1", "One"), Opt("2", "Two"), Opt("3", "Three"), Opt("4", "Four"), Opt("5", "Five"),
	}})
	f, _ := reg.Field("r")
	got := chipValueSummary(stubPageCtx{}, f, filterq.Condition{
		Field: "r", Op: filterq.OpIs, Values: []string{"1", "2", "3", "4", "5"},
	})
	assert.Equal(t, "One, Two, Three +2", got)
}
