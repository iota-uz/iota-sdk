package filterbuilder

import (
	"context"
	"net/url"
	"strconv"
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

	// The date field ("issue_at") draft editor renders both flatpickr pickers
	// (single + range). Each must serialize an explicit Y-m-d dateFormat into
	// its x-data; assert on the serialized field for BOTH (count >= 2), not just
	// a loose "Y-m-d" substring, so a regression on one picker is still caught.
	// HTML-attribute escaping of the JSON quotes is &#34; (verified).
	const wantFmt = `&#34;dateFormat&#34;:&#34;Y-m-d&#34;`
	assert.Contains(t, html, "datePicker(")
	assert.GreaterOrEqual(t, strings.Count(html, wantFmt), 2)
	// Never the empty value that triggers the bogus 'z' flatpickr fallback.
	assert.NotContains(t, html, `&#34;dateFormat&#34;:&#34;&#34;`)
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
	// editor internals must never serialize into the HTMX form request, and must
	// not even carry an empty name="" (the date editors omit the attribute).
	for _, frag := range strings.Split(html, "name=\"")[1:] {
		name := frag[:strings.Index(frag, "\"")]
		assert.Contains(t, []string{"f", "fb"}, name, "unexpected named control %q", name)
	}
	// Guard the templ `else if` trap: omitting the empty name must not leave a
	// stray literal `else` token in the date editor's hidden inputs.
	assert.NotContains(t, html, " else", "stray literal else leaked into rendered markup")
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

func referenceOptions(n int) []Option {
	options := make([]Option, n)
	for i := range options {
		options[i] = Opt(strconv.Itoa(i), "Option "+strconv.Itoa(i))
	}
	return options
}

func TestUsesCheckboxList(t *testing.T) {
	grouped := referenceOptions(2)
	grouped[1].Group = "Voluntary"

	tests := []struct {
		name    string
		options []Option
		want    bool
	}{
		{name: "no options keep the empty state", options: nil, want: false},
		{name: "one option", options: referenceOptions(1), want: true},
		{name: "just under the limit", options: referenceOptions(4), want: true},
		{name: "at the limit", options: referenceOptions(5), want: false},
		{name: "any grouped option", options: grouped, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, usesCheckboxList(tt.options))
		})
	}
}

func TestReferenceEditorPicksControlByOptions(t *testing.T) {
	registry := NewRegistry(
		FieldDef{Key: "short", Type: filterq.FieldTypeReference, Label: "Short", Options: referenceOptions(4)},
		FieldDef{Key: "long", Type: filterq.FieldTypeReference, Label: "Long", Options: referenceOptions(5)},
		FieldDef{Key: "seria", Type: filterq.FieldTypeReference, Label: "Seria", Options: []Option{
			{Value: "EEIU", Label: "EEIU", Count: 1, Group: "Mandatory"},
		}},
	)
	editor := func(key string) string {
		field, ok := registry.Field(key)
		require.True(t, ok)
		return render(t, editorPanel(field, nil, -1))
	}

	short := editor("short")
	assert.Equal(t, 4, strings.Count(short, `type="checkbox"`))
	assert.NotContains(t, short, " multiple", "the combobox renders a multiple select")

	for _, key := range []string{"long", "seria"} {
		html := editor(key)
		assert.Contains(t, html, " multiple", key)
		assert.NotContains(t, html, "data-fb-checkboxes", key)
	}
}

func TestCheckboxOptionsMarkup(t *testing.T) {
	html := render(t, CheckboxOptions([]Option{
		{Value: "1", Label: "Active", Count: 12403},
		{Value: "2", Label: "Annulled", Count: 0, Disabled: true},
		Opt("3", "Draft"),
	}, []string{"1"}))

	inputs := checkboxInputs(t, html)
	require.Len(t, inputs, 3)

	assert.Contains(t, inputs[0], ` checked`, "selected values are pre-checked")
	assert.NotContains(t, inputs[1], ` checked`)
	assert.NotContains(t, inputs[2], ` checked`)

	assert.Contains(t, inputs[1], ` disabled`, "zero-count options stay unselectable")
	assert.NotContains(t, inputs[0], ` disabled`)

	for i, value := range []string{"1", "2", "3"} {
		assert.Contains(t, inputs[i], `value="`+value+`"`)
		assert.Contains(t, inputs[i], `data-fb-value`, "fbPanel.collect reads this marker")
		assert.NotContains(t, inputs[i], `name=`, "editor controls must not serialize into the form")
	}

	assert.Contains(t, html, `Number(12403)`, "a counter renders as a badge")
	assert.Contains(t, html, `Number(0)`, "a zero counter is still a counter")
	assert.Equal(t, 2, strings.Count(html, `toLocaleString`), "a negative count means no counter")
}

func TestChipEditorPreChecksConditionValues(t *testing.T) {
	fs := filterq.FilterSet{{Field: "status", Op: filterq.OpIs, Values: []string{"2"}}}
	html := renderComponent(t, Props{Registry: testRegistry(), Filters: fs}, false)

	var checked []string
	for _, tag := range checkboxInputs(t, html) {
		if strings.Contains(tag, ` checked`) {
			checked = append(checked, tag)
		}
	}
	require.Len(t, checked, 1)
	assert.Contains(t, checked[0], `value="2"`)
}

// checkboxInputs returns the opening tag of every option checkbox in html.
func checkboxInputs(t *testing.T, html string) []string {
	t.Helper()
	var tags []string
	for _, frag := range strings.Split(html, "<input")[1:] {
		end := strings.Index(frag, ">")
		require.GreaterOrEqual(t, end, 0)
		if tag := frag[:end]; strings.Contains(tag, `type="checkbox"`) && strings.Contains(tag, "data-fb-value") {
			tags = append(tags, tag)
		}
	}
	return tags
}

func TestRegistryDecode(t *testing.T) {
	q := url.Values{filterq.ParamName: []string{"status:is:1", "bogus:is:1"}}
	fs := testRegistry().Decode(q)
	require.Len(t, fs, 1)
	assert.Equal(t, "status", fs[0].Field)
}

func TestPanelArgsEmptyOperatorsNoPanic(t *testing.T) {
	// A field with an explicit empty operator slice and an unknown type would
	// otherwise index operators()[0] out of range and panic at chip render.
	field := FieldDef{Key: "x", Type: filterq.FieldType("mystery"), Operators: []filterq.Operator{}}
	assert.NotPanics(t, func() {
		_ = panelArgs(field, nil, -1)
	})
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

func TestNewRegistryDeduplicatesKeys(t *testing.T) {
	reg := NewRegistry(
		FieldDef{Key: "status", Type: filterq.FieldTypeReference, Label: "First"},
		FieldDef{Key: "agency", Type: filterq.FieldTypeReference, Label: "Agency"},
		FieldDef{Key: "status", Type: filterq.FieldTypeReference, Label: "Second"},
	)

	// Fields(), Field() and Schema() must agree: the duplicate "status" key keeps
	// only its first registration, so no view of the registry diverges.
	require.Len(t, reg.Fields(), 2)
	assert.Equal(t, []string{"status", "agency"}, []string{reg.Fields()[0].Key, reg.Fields()[1].Key})

	f, ok := reg.Field("status")
	require.True(t, ok)
	assert.Equal(t, "First", f.Label, "Field() must resolve to the same entry Fields() keeps")

	assert.Len(t, reg.Schema().Fields, 2, "Schema() must not emit duplicate fields")
}
