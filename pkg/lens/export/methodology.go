package export

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/pkg/lens/formula"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

// methodologyRows lists every exported panel that carries a formula: the
// formula in words (Excel cannot typeset it, and the LaTeX source is not for
// readers), its version and date, and each variable with its value in the same
// frame the figure was read from. It returns nil when no panel has a formula,
// so such a workbook gets no empty sheet.
func methodologyRows(labels Labels, panels []*lensruntime.PanelResult) [][]any {
	rows := [][]any{{labels.Metric, labels.Formula, labels.Variable, labels.Value, labels.Source, labels.Version, labels.Date}}
	for _, result := range panels {
		if result == nil || result.Panel.Formula == nil {
			continue
		}
		spec := result.Panel.Formula
		text, err := spec.TextAlternative()
		if err != nil {
			logrus.WithFields(logrus.Fields{"panel": result.Panel.ID, "error": err.Error()}).Error("lens: invalid formula markup in export")
			continue
		}
		if len(spec.Variables) == 0 {
			rows = append(rows, []any{result.Panel.Title, text, "", "", "", spec.Version, spec.Date})
			continue
		}
		for i, variable := range spec.Variables {
			head, version, date := []any{"", ""}, any(""), any("")
			if i == 0 {
				head, version, date = []any{result.Panel.Title, text}, spec.Version, spec.Date
			}
			var value any = variable.Value
			if resolved, ok := variableValue(result, variable); ok {
				value = resolved
			}
			label := strings.TrimSpace(variable.Label)
			if label == "" {
				label = variable.Key
			}
			rows = append(rows, append(head, label, value, variable.Source, version, date))
		}
	}
	if len(rows) == 1 {
		return nil
	}
	return rows
}

// variableValue reads a variable from the frame the figure came from, the way
// the browser does: a stage by its key through the ID and value columns (a
// duplicated key has no single value), a measure or field by column name. A
// frame formula contributes its computed result — replaying the expression on
// the Methodology sheet would point at that sheet's cells, not the data's.
func variableValue(result *lensruntime.PanelResult, variable formula.Variable) (any, bool) {
	if variable.Ref == nil || result.Frames == nil {
		return nil, false
	}
	primary := result.Frames.Primary()
	if primary == nil {
		return nil, false
	}
	if variable.Ref.Kind == formula.RefStage {
		idField, okID := primary.Field(result.Panel.Fields.ID.Name())
		valueField, okValue := primary.Field(result.Panel.Fields.Value.Name())
		if !okID || !okValue || idField == nil || valueField == nil {
			return nil, false
		}
		match := -1
		for i, id := range idField.Values {
			if fmt.Sprint(id) != variable.Ref.Name {
				continue
			}
			if match >= 0 {
				return nil, false
			}
			match = i
		}
		if match < 0 || match >= len(valueField.Values) {
			return nil, false
		}
		if stageUnavailable(result.Panel, primary, variable.Ref.Name, match) {
			// An empty cell that wins over a supplied fallback value: the
			// stage cannot have a number in this slice.
			return "", true
		}
		return exportable(valueField.Values[match], result.Panel, *valueField)
	}
	field, ok := primary.Field(variable.Ref.Name)
	if !ok || field == nil || len(field.Values) == 0 {
		return nil, false
	}
	return exportable(field.Values[0], result.Panel, *field)
}

// exportable turns a frame cell into the Methodology sheet's value. A formula
// cell (by value or by pointer) contributes its computed result; its expression
// is never copied, since on this sheet its references would address the wrong
// cells. A formula with no result has no value to show.
func exportable(value any, spec panel.Spec, field frame.Field) (any, bool) {
	switch f := value.(type) {
	case frame.Formula:
		if f.Result == nil {
			return "", true
		}
		value = f.Result
	case *frame.Formula:
		// A formula cell that has no computed result is an empty cell, not a
		// reason to fall back to a supplied value for a different slice.
		if f == nil || f.Result == nil {
			return "", true
		}
		value = f.Result
	}
	if value == nil {
		return nil, false
	}
	return formattedValue{Value: value, Format: panelFormats(spec)(field)}, true
}

// stageUnavailable applies the browser's precedence (frame column, then the
// stage's own declaration, then the panel default): a stage that cannot have a
// number must not export one, even if the frame carries a zero.
func stageUnavailable(spec panel.Spec, primary *frame.Frame, key string, row int) bool {
	declared := panel.Availability("")
	if name := spec.Fields.Availability.Name(); name != "" {
		if field, ok := primary.Field(name); ok && field != nil && row < len(field.Values) {
			if text, isText := field.Values[row].(string); isText {
				declared = panel.Availability(text)
			}
		}
	}
	if declared == "" {
		for _, stage := range spec.FlowStages {
			if stage.Key == key {
				declared = stage.Availability
			}
		}
	}
	if declared == "" {
		for _, row := range spec.HierarchyRows {
			if row.Key == key {
				declared = row.Availability
			}
		}
	}
	if declared == "" {
		declared = spec.Availability
	}
	return declared != "" && declared != panel.AvailabilityAvailable
}
