package export

import (
	"strings"

	"github.com/sirupsen/logrus"

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
			if variable.Ref != nil && result.Frames != nil {
				if primary := result.Frames.Primary(); primary != nil {
					if field, ok := primary.Field(variable.Ref.Name); ok && field != nil && len(field.Values) > 0 {
						value = formattedValue{Value: field.Values[0], Format: panelFormats(result.Panel)(*field)}
					}
				}
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
