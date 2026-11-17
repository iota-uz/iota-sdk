package serve

import (
	"cmp"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

func tableFrameView(spec panel.Spec, source document.Frame, search string, ordering *document.TableSort) (document.Frame, *document.TableSummary) {
	if spec.Kind != panel.KindTable {
		return source, nil
	}
	full := document.Frame{Columns: append([]document.Column(nil), source.Columns...), Rows: make([][]any, len(source.Rows)), Children: source.Children}
	for index, row := range source.Rows {
		full.Rows[index] = append([]any(nil), row...)
	}
	recomputeShares(spec, &full)
	view := document.Frame{Columns: append([]document.Column(nil), full.Columns...), Rows: make([][]any, 0, len(full.Rows)), Children: full.Children}
	search = strings.ToLower(strings.TrimSpace(search))
	searchIndexes := tableSearchIndexes(spec, source)
	for _, sourceRow := range full.Rows {
		row := append([]any(nil), sourceRow...)
		if search != "" && !tableRowMatches(row, searchIndexes, search) {
			continue
		}
		view.Rows = append(view.Rows, row)
	}
	recomputeShares(spec, &view)
	if ordering != nil {
		index := frameColumnIndex(view, ordering.Field)
		sort.SliceStable(view.Rows, func(left, right int) bool {
			comparison := compareTableCells(view.Rows[left][index], view.Rows[right][index])
			if ordering.Direction == document.SortDescending {
				return comparison > 0
			}
			return comparison < 0
		})
	}
	values := tableTotals(spec, view)
	fullValues := tableTotals(spec, full)
	if spec.Table == nil && len(values) == 0 {
		return view, nil
	}
	return view, &document.TableSummary{
		Values: values, FullValues: fullValues, FilteredRows: len(view.Rows), TotalRows: len(full.Rows),
	}
}

// levelTableSummary is the footer of a table level. A level whose rows are all
// in hand (the host returned the whole selection, even if the wire trims it to a
// page afterwards) keeps the sum computed over them. A level the host loaded one
// page of cannot total the selection from that page, so each column takes the
// host's authoritative total or goes without one; the runtime then prints an
// em dash instead of the page's sum under a "Total" label.
func levelTableSummary(
	spec panel.Spec,
	summary *document.TableSummary,
	pagination *lensruntime.TablePagination,
	totals map[string]float64,
) *document.TableSummary {
	if summary == nil {
		return nil
	}
	partial := pagination != nil && (pagination.Page > 1 || pagination.HasMore)
	values := make(map[string]any, len(summary.Values))
	for _, column := range spec.Columns {
		if !column.Total {
			continue
		}
		name := column.Field.Name()
		if total, ok := totals[name]; ok {
			values[name] = total
		} else if value, ok := summary.Values[name]; ok && !partial {
			values[name] = value
		}
	}
	if len(values) == 0 {
		return nil
	}
	return &document.TableSummary{Values: values, FilteredRows: summary.FilteredRows, TotalRows: summary.TotalRows}
}

func tableTotals(spec panel.Spec, source document.Frame) map[string]any {
	values := make(map[string]any)
	for _, column := range spec.Columns {
		if !column.Total {
			continue
		}
		index := frameColumnIndex(source, column.Field.Name())
		if index < 0 {
			continue
		}
		var total float64
		for _, row := range source.Rows {
			if index < len(row) {
				total += tableNumber(row[index])
			}
		}
		values[column.Field.Name()] = total
	}
	return values
}

func compareTableCells(left, right any) int {
	leftNumber, leftNumeric := tableComparableNumber(left)
	rightNumber, rightNumeric := tableComparableNumber(right)
	if leftNumeric && rightNumeric {
		return cmp.Compare(leftNumber, rightNumber)
	}
	return strings.Compare(strings.ToLower(fmt.Sprint(left)), strings.ToLower(fmt.Sprint(right)))
}

func tableComparableNumber(value any) (float64, bool) {
	switch value.(type) {
	case float64, float32, int, int64, int32:
		return tableNumber(value), true
	default:
		return 0, false
	}
}

func tableSearchIndexes(spec panel.Spec, source document.Frame) []int {
	indexes := make([]int, 0, len(spec.Columns))
	for _, column := range spec.Columns {
		if index := frameColumnIndex(source, column.Field.Name()); index >= 0 {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func tableRowMatches(row []any, indexes []int, search string) bool {
	for _, index := range indexes {
		if index < len(row) && strings.Contains(strings.ToLower(fmt.Sprint(row[index])), search) {
			return true
		}
	}
	return false
}

func recomputeShares(spec panel.Spec, frame *document.Frame) {
	for _, column := range spec.Columns {
		if column.ShareOf.Empty() {
			continue
		}
		output := frameColumnIndex(*frame, column.Field.Name())
		source := frameColumnIndex(*frame, column.ShareOf.Name())
		if output < 0 || source < 0 {
			continue
		}
		var total float64
		for _, row := range frame.Rows {
			if source < len(row) {
				total += tableNumber(row[source])
			}
		}
		for _, row := range frame.Rows {
			if output >= len(row) {
				continue
			}
			if total == 0 || source >= len(row) {
				row[output] = float64(0)
				continue
			}
			row[output] = tableNumber(row[source]) / total * 100
		}
	}
}

func frameColumnIndex(frame document.Frame, name string) int {
	for index, column := range frame.Columns {
		if column.Name == name {
			return index
		}
	}
	return -1
}

func tableNumber(value any) float64 {
	switch typed := value.(type) {
	case float64:
		if !math.IsNaN(typed) && !math.IsInf(typed, 0) {
			return typed
		}
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case int32:
		return float64(typed)
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed
	}
	return 0
}
