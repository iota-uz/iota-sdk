// Package filterbuilder renders a GitHub-Projects / Linear style filter
// chip-builder on top of pkg/filterq: a row of editable/removable chips plus
// an "+ Add filter" popover (field → operator → value).
//
// Chips are server-rendered from the decoded FilterSet; Alpine only drives
// popovers and writes the hidden `f` inputs, so the enclosing HTMX form
// (hx-include="closest form" + hx-push-url) reproduces the filterq URL codec
// byte-for-byte. The component dispatches a bubbling `filter-changed` event
// after every mutation — the same convention the scaffold table listens to.
package filterbuilder

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/pkg/filterq"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

// Option is one selectable value of a reference field.
type Option struct {
	Value string
	Label string // pre-translated by the consumer
	// Count is a live usage counter rendered as a badge (`EEIU (12 403)`).
	// Negative means "no counter".
	Count int
	// Disabled dims the option and makes it unselectable (e.g. zero-count series).
	Disabled bool
	// Group is a pre-translated optgroup heading; options sharing consecutive
	// Group values render under one header.
	Group string
}

// Opt is a shorthand constructor for a counterless option.
func Opt(value, label string) Option {
	return Option{Value: value, Label: label, Count: -1}
}

// FieldDef describes one filterable field: filterq validation surface plus UI
// metadata. Labels are pre-translated by the consumer — the SDK only owns
// generic strings (operators, presets, buttons).
type FieldDef struct {
	Key  string
	Type filterq.FieldType
	// Label is the field display name.
	Label string
	// Group clusters fields inside the "+ Add filter" popover
	// (e.g. "References" / "Dates" / "Numbers" / "Flags").
	Group string
	// Operators allowed for the field; nil → filterq.DefaultOperators(Type).
	Operators []filterq.Operator
	// Options for reference fields.
	Options []Option
	// Presets for date fields; nil → filterq.AllPresets().
	Presets []filterq.DatePreset
}

func (f FieldDef) operators() []filterq.Operator {
	// Fall back to the type defaults when no operators are set — treat an empty
	// (non-nil) slice the same as nil so callers never get a zero-length list.
	if len(f.Operators) > 0 {
		return f.Operators
	}
	return filterq.DefaultOperators(f.Type)
}

func (f FieldDef) presets() []filterq.DatePreset {
	if f.Type != filterq.FieldTypeDate {
		return nil
	}
	// Presets resolve to ranges, so they require the between operator.
	allowsBetween := filterq.Field{Key: f.Key, Type: f.Type, Operators: f.Operators}.AllowsOp(filterq.OpBetween)
	if !allowsBetween {
		return nil
	}
	if f.Presets != nil {
		return f.Presets
	}
	return filterq.AllPresets()
}

// Registry is the ordered set of filterable fields for one page.
type Registry struct {
	fields []FieldDef
	index  map[string]int
}

// NewRegistry builds a registry preserving field order (which drives the
// "+ Add filter" menu order). Duplicate keys are ignored after the first
// occurrence so Fields(), Field() and Schema() stay consistent (otherwise the
// last-wins index would disagree with the all-kept fields slice).
func NewRegistry(fields ...FieldDef) *Registry {
	r := &Registry{index: make(map[string]int, len(fields))}
	for _, f := range fields {
		if _, dup := r.index[f.Key]; dup {
			continue
		}
		r.index[f.Key] = len(r.fields)
		r.fields = append(r.fields, f)
	}
	return r
}

// Fields returns the fields in registration order.
func (r *Registry) Fields() []FieldDef { return r.fields }

// Field looks up a field by key.
func (r *Registry) Field(key string) (FieldDef, bool) {
	i, ok := r.index[key]
	if !ok {
		return FieldDef{}, false
	}
	return r.fields[i], true
}

// Schema derives the filterq validation schema.
func (r *Registry) Schema() filterq.Schema {
	fields := make([]filterq.Field, 0, len(r.fields))
	for _, f := range r.fields {
		fields = append(fields, filterq.Field{Key: f.Key, Type: f.Type, Operators: f.Operators})
	}
	return filterq.Schema{Fields: fields}
}

// Decode parses request query values against this registry's schema.
func (r *Registry) Decode(q url.Values) filterq.FilterSet {
	return filterq.Decode(q, r.Schema())
}

// Props configures the Builder component.
type Props struct {
	Registry *Registry
	Filters  filterq.FilterSet
	// ID lets a page host several builders; defaults to "filter-builder".
	ID    string
	Class templ.CSSClasses
}

func (p Props) id() string {
	if p.ID != "" {
		return p.ID
	}
	return "filter-builder"
}

// fieldGroups returns field definitions clustered by Group, preserving first
// appearance order of each group.
func fieldGroups(fields []FieldDef) [][]FieldDef {
	var order []string
	byGroup := map[string][]FieldDef{}
	for _, f := range fields {
		if _, ok := byGroup[f.Group]; !ok {
			order = append(order, f.Group)
		}
		byGroup[f.Group] = append(byGroup[f.Group], f)
	}
	out := make([][]FieldDef, 0, len(order))
	for _, g := range order {
		out = append(out, byGroup[g])
	}
	return out
}

// operatorKey maps an operator to its SDK locale key.
func operatorKey(op filterq.Operator) string {
	switch op {
	case filterq.OpIs:
		return "Scaffold.FilterBuilder.Operators.Is"
	case filterq.OpIsNot:
		return "Scaffold.FilterBuilder.Operators.IsNot"
	case filterq.OpBefore:
		return "Scaffold.FilterBuilder.Operators.Before"
	case filterq.OpAfter:
		return "Scaffold.FilterBuilder.Operators.After"
	case filterq.OpOn:
		return "Scaffold.FilterBuilder.Operators.On"
	case filterq.OpBetween:
		return "Scaffold.FilterBuilder.Operators.Between"
	case filterq.OpEq:
		return "Scaffold.FilterBuilder.Operators.Eq"
	case filterq.OpGt:
		return "Scaffold.FilterBuilder.Operators.Gt"
	case filterq.OpLt:
		return "Scaffold.FilterBuilder.Operators.Lt"
	}
	return ""
}

// presetKey maps a preset to its SDK locale key.
func presetKey(p filterq.DatePreset) string {
	switch p {
	case filterq.PresetThisMonth:
		return "Scaffold.FilterBuilder.Presets.ThisMonth"
	case filterq.PresetLastMonth:
		return "Scaffold.FilterBuilder.Presets.LastMonth"
	case filterq.PresetLast30D:
		return "Scaffold.FilterBuilder.Presets.Last30Days"
	case filterq.PresetNext30D:
		return "Scaffold.FilterBuilder.Presets.Next30Days"
	case filterq.PresetThisYear:
		return "Scaffold.FilterBuilder.Presets.ThisYear"
	case filterq.PresetLastYear:
		return "Scaffold.FilterBuilder.Presets.LastYear"
	}
	return ""
}

// maxChipValues caps how many value labels a chip shows before collapsing the
// rest into "+N".
const maxChipValues = 3

// chipValueSummary renders the human-readable value part of a chip:
// reference values resolve to option labels, presets resolve to their
// localized label, everything else renders verbatim.
func chipValueSummary(pageCtx types.PageContext, f FieldDef, c filterq.Condition) string {
	if f.Type == filterq.FieldTypeBool {
		return ""
	}
	if p, ok := c.Preset(); ok {
		return pageCtx.T(presetKey(p))
	}
	labelOf := make(map[string]string, len(f.Options))
	for _, o := range f.Options {
		labelOf[o.Value] = o.Label
	}
	labels := make([]string, 0, len(c.Values))
	for _, v := range c.Values {
		if l, ok := labelOf[v]; ok {
			labels = append(labels, l)
		} else {
			labels = append(labels, v)
		}
	}
	if c.Op == filterq.OpBetween && len(labels) == 2 {
		return labels[0] + " – " + labels[1]
	}
	if len(labels) > maxChipValues {
		return strings.Join(labels[:maxChipValues], ", ") + " +" + strconv.Itoa(len(labels)-maxChipValues)
	}
	return strings.Join(labels, ", ")
}
