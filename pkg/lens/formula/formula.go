// Package formula is the authoring contract for a metric's methodology: a
// typeset formula, the variables it names, and the version of the method.
//
// A formula is data about a figure, not a figure: it never computes anything.
// Variable values come from the same frame (filters, period, selected row)
// that carries the figure itself, either joined on the client by Ref or handed
// over already resolved in Variable.Value by a server-rendered page.
package formula

import (
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens/format"
)

// RefKind says where a variable's value lives in the panel's frame.
type RefKind string

const (
	// RefMeasure reads a cube measure column (named like the measure).
	RefMeasure RefKind = "measure"
	// RefField reads a frame column.
	RefField RefKind = "field"
	// RefStage reads a MetricFlow stage (joined by stage key).
	RefStage RefKind = "stage"
)

// Ref points a variable at the data that carries its value.
type Ref struct {
	Kind RefKind `json:"kind"`
	Name string  `json:"name"`
	// Availability names a frame column, on the same row, that says whether the
	// value can be stated ("available", "unavailable", "config_required",
	// "empty_source" — the panel Availability values). A field read from a row
	// whose metric is not configured is then explained, not just blank. Only
	// for RefField: a stage already carries its availability in the flow.
	Availability string `json:"availability,omitempty"`
}

// Variable is one symbol of the formula.
type Variable struct {
	// Key is the LaTeX symbol as written in the formula, e.g. "N" for \mathrm{N}
	// or "revenue". It is the handle hover/focus uses to find the symbol.
	Key   string `json:"key"`
	Label string `json:"label"`
	Ref   *Ref   `json:"ref,omitempty"`
	// Source is a human sentence on where the value comes from (shown, not
	// resolved): «Бухгалтерский учёт, счёт 90».
	Source string `json:"source,omitempty"`
	// Value is a ready, already formatted value. A server-rendered page that
	// has no frame passes it here; with a frame the client resolves Ref and
	// this stays empty.
	Value string `json:"value,omitempty"`
	// Format formats the value Ref resolves. Without it the client uses the
	// panel's format for that column, which a panel declares only for its own
	// figure: a variable read from another column of the same frame (a stat
	// whose formula names the other KPIs of its row) needs its own.
	Format *format.Spec `json:"format,omitempty"`
}

// Spec describes how a figure is calculated.
type Spec struct {
	// Latex is the formula source. It is never shown to the user.
	Latex string `json:"latex"`
	// Alt is the author's plain-language reading of the formula, localized.
	// When empty the runtime derives a text alternative from Latex.
	Alt       string     `json:"alt,omitempty"`
	Variables []Variable `json:"variables,omitempty"`
	// Version and Date identify the methodology revision (Date is ISO 8601).
	Version string `json:"version,omitempty"`
	Date    string `json:"date,omitempty"`
}

// Validate reports a spec the runtime could not present: empty formula, markup
// that does not parse, duplicate or unreferenced-ref variables.
func (s Spec) Validate() error {
	if strings.TrimSpace(s.Latex) == "" {
		return fmt.Errorf("formula latex is required")
	}
	if _, err := parse(s.Latex); err != nil {
		return fmt.Errorf("formula latex: %w", err)
	}
	seen := make(map[string]struct{}, len(s.Variables))
	for _, v := range s.Variables {
		key := strings.TrimSpace(v.Key)
		if key == "" {
			return fmt.Errorf("formula variable key is required")
		}
		if _, dup := seen[key]; dup {
			return fmt.Errorf("formula variable %q is duplicated", key)
		}
		seen[key] = struct{}{}
		if v.Ref != nil {
			switch v.Ref.Kind {
			case RefMeasure, RefField, RefStage:
			default:
				return fmt.Errorf("formula variable %q has unknown ref kind %q", key, v.Ref.Kind)
			}
			if strings.TrimSpace(v.Ref.Name) == "" {
				return fmt.Errorf("formula variable %q ref name is required", key)
			}
			if v.Ref.Availability != "" && v.Ref.Kind != RefField {
				return fmt.Errorf("formula variable %q: an availability column applies to a field ref only", key)
			}
		}
	}
	return nil
}

// Markup renders the formula as MathML, ready to inline into HTML for print,
// PDF and no-JS pages. The client upgrades it to KaTeX when it can.
func (s Spec) Markup() (string, error) {
	n, err := parse(s.Latex)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(`<math xmlns="http://www.w3.org/1998/Math/MathML" display="block"><mrow>`)
	n.mathml(&b)
	b.WriteString(`</mrow></math>`)
	return b.String(), nil
}

// TextAlternative returns the formula as a plain-language text alternative.
func (s Spec) TextAlternative() (string, error) {
	if strings.TrimSpace(s.Alt) != "" {
		return strings.TrimSpace(s.Alt), nil
	}
	n, err := parse(s.Latex)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	n.text(&b)
	return strings.Join(strings.Fields(b.String()), " "), nil
}
