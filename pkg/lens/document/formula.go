package document

import (
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/pkg/lens/formula"
)

// FormulaRefKind says where a formula variable's value lives in the frame.
type FormulaRefKind string

const (
	FormulaRefMeasure FormulaRefKind = "measure"
	FormulaRefField   FormulaRefKind = "field"
	FormulaRefStage   FormulaRefKind = "stage"
)

// FormulaRef points a variable at the frame data that carries its value.
type FormulaRef struct {
	Kind FormulaRefKind `json:"kind"`
	Name string         `json:"name"`
}

// FormulaVariable is one symbol of a formula.
type FormulaVariable struct {
	Key    string      `json:"key"`
	Label  string      `json:"label"`
	Ref    *FormulaRef `json:"ref,omitempty"`
	Source string      `json:"source,omitempty"`
	// Value is a ready, formatted value for hosts without a frame (templ pages).
	// With a frame the client resolves Ref instead.
	Value string `json:"value,omitempty"`
}

// Formula is the wire form of formula.Spec. The LaTeX source is carried for the
// renderer but never shown. Markup is the server-rendered MathML for print, PDF
// and environments without JavaScript; Alt is the text alternative. Error is set
// when the LaTeX does not parse: the panel then shows a message, not a formula.
type Formula struct {
	Latex     string            `json:"latex"`
	Alt       string            `json:"alt,omitempty"`
	Markup    string            `json:"markup,omitempty"`
	Variables []FormulaVariable `json:"variables,omitempty"`
	Version   string            `json:"version,omitempty"`
	Date      string            `json:"date,omitempty"`
	Error     bool              `json:"error,omitempty"`
}

// BuildFormula converts an authored spec to its wire form. Invalid markup never
// fails the document: the error is logged and the wire formula is flagged.
func BuildFormula(owner string, spec *formula.Spec) *Formula {
	if spec == nil || strings.TrimSpace(spec.Latex) == "" {
		return nil
	}
	out := &Formula{
		Latex:   spec.Latex,
		Version: strings.TrimSpace(spec.Version),
		Date:    strings.TrimSpace(spec.Date),
	}
	for _, v := range spec.Variables {
		variable := FormulaVariable{Key: v.Key, Label: v.Label, Source: v.Source, Value: v.Value}
		if v.Ref != nil {
			variable.Ref = &FormulaRef{Kind: FormulaRefKind(v.Ref.Kind), Name: v.Ref.Name}
		}
		out.Variables = append(out.Variables, variable)
	}
	markup, err := spec.Markup()
	if err != nil {
		logrus.WithFields(logrus.Fields{"panel": owner, "error": err.Error()}).Error("lens: invalid formula markup")
		out.Error = true
		return out
	}
	out.Markup = markup
	out.Alt, _ = spec.TextAlternative()
	return out
}
