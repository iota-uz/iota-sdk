// Package comparison joins a baseline period into a dashboard so panels can
// render deltas against it.
package comparison

import (
	"fmt"
	"math"
	"reflect"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
)

const defaultMaxTableMetrics = 3

func PreviousField(name string) string { return "previous_" + name }

func DeltaField(name string) string { return name + "_delta" }

func DeltaRatioField(name string) string { return name + "_delta_ratio" }

func DeltaPercentField(name string) string { return name + "_delta_percent" }

type Labels struct {
	Before string
	After  string
	Delta  string
	Trend  string
}

type StaticOptions struct {
	Labels          Labels
	MaxTableMetrics int
	// InvertTrend is the legacy two-state direction hook: true means a fall is
	// the good news. It cannot say "no opinion", so a producer that answers
	// false for a metric nobody has judged gets it coloured as though higher
	// were better. Prefer TrendPolarity.
	InvertTrend func(fieldName string) bool
	// TrendPolarity is the tri-state form and wins wherever it answers. Return
	// panel.TrendNeutral — or leave the field nil — for a metric with no
	// verdict attached, such as sum insured, where a rise is exposure rather
	// than good or bad news.
	TrendPolarity     func(fieldName string) panel.TrendPolarity
	AbsoluteDeltaUnit func(fieldName string) panel.TrendDeltaUnit
}

// trendPolarity resolves the two hooks into the single value a TrendSpec
// carries. A producer that supplies TrendPolarity owns the answer completely,
// including its silences: an unnamed field is neutral, which is the point of
// the tri-state. InvertTrend is consulted only when no tri-state hook exists,
// so producers that have not migrated keep the colours they had.
func (o StaticOptions) trendPolarity(fieldName string) panel.TrendPolarity {
	if o.TrendPolarity != nil {
		if polarity := o.TrendPolarity(fieldName); polarity != "" {
			return polarity
		}
		return panel.TrendNeutral
	}
	if o.InvertTrend != nil && o.InvertTrend(fieldName) {
		return panel.TrendLowerBetter
	}
	return panel.TrendNeutral
}

// ConfigureProgressivePresentation declares the comparison fields that a
// progressive document must expose before its deferred frames have been
// calculated. ApplyStatic still owns the data join; this function only keeps
// the frozen document contract in sync with the frame it will receive later.
//
// Tables are intentionally left to ApplyStatic because their before/current/
// delta column expansion depends on the materialized fields. Progressive
// callers that freeze table metadata early must declare those columns in their
// dashboard-specific shell.
func ConfigureProgressivePresentation(current *lens.DashboardSpec, enabled bool, options StaticOptions) {
	if current == nil {
		return
	}
	for rowIndex := range current.Rows {
		for panelIndex := range current.Rows[rowIndex].Panels {
			configureProgressivePanel(&current.Rows[rowIndex].Panels[panelIndex], enabled, options)
		}
	}
	if enabled {
		ensureProgressiveComparisonFields(current)
	}
}

// ensureProgressiveComparisonFields keeps the structural dataset contract in
// step with the presentation contract declared above. Progressive documents
// are compiled before their deferred query runs; without placeholder fields,
// the frozen panel can legitimately reference previous/delta columns which
// are absent from its structural frame and later fail runtime validation.
func ensureProgressiveComparisonFields(spec *lens.DashboardSpec) {
	datasets := make(map[string]*lens.DatasetSpec, len(spec.Datasets))
	for index := range spec.Datasets {
		datasets[spec.Datasets[index].Name] = &spec.Datasets[index]
	}
	var visit func(panel.Spec)
	visit = func(current panel.Spec) {
		for _, child := range current.Children {
			visit(child)
		}
		dataset := datasets[current.Dataset]
		if dataset == nil || dataset.Static == nil || dataset.Static.Primary() == nil {
			return
		}
		currentFrame := dataset.Static.Primary()
		refs := []panel.FieldRef{current.Fields.Previous}
		if current.Trend != nil {
			refs = append(refs, current.Trend.AbsoluteField, current.Trend.PercentField)
		}
		for _, column := range current.Columns {
			refs = append(refs, column.Field)
			if column.Cell != nil {
				refs = append(refs, column.Cell.PercentField)
			}
		}
		for _, ref := range refs {
			name := ref.Name()
			if name == "" {
				continue
			}
			if _, exists := currentFrame.Field(name); exists {
				continue
			}
			values := make([]any, currentFrame.RowCount)
			currentFrame.Fields = append(currentFrame.Fields, frame.Field{
				Name: name, Type: frame.FieldTypeNumber, Role: frame.RoleMetric, Values: values,
			})
		}
		_ = currentFrame.Normalize()
	}
	for _, row := range spec.Rows {
		for _, current := range row.Panels {
			visit(current)
		}
	}
}

func configureProgressivePanel(spec *panel.Spec, enabled bool, options StaticOptions) {
	if spec == nil {
		return
	}
	for index := range spec.Children {
		configureProgressivePanel(&spec.Children[index], enabled, options)
	}
	valueField := spec.Fields.Value.Name()
	if !enabled {
		spec.ComparisonUnsupported = false
		if valueField != "" && spec.Fields.Previous.Name() == PreviousField(valueField) {
			spec.Fields.Previous = ""
		}
		if valueField != "" && spec.Trend != nil && spec.Trend.AbsoluteField.Name() == DeltaField(valueField) {
			spec.Trend = nil
		}
		return
	}
	if spec.Kind == panel.KindTable {
		maxTableMetrics := options.MaxTableMetrics
		if maxTableMetrics <= 0 {
			maxTableMetrics = defaultMaxTableMetrics
		}
		columns := make([]panel.TableColumn, 0, len(spec.Columns)*3)
		expanded := 0
		for _, column := range spec.Columns {
			fieldName := column.Field.Name()
			if column.Formatter == nil || !column.ShareOf.Empty() || expanded >= maxTableMetrics ||
				strings.HasPrefix(fieldName, "previous_") || strings.HasSuffix(fieldName, "_delta") {
				columns = append(columns, column)
				continue
			}
			expanded++
			before := column
			before.Field = panel.Ref(PreviousField(fieldName))
			before.Label = columnLabel(column.Label, options.Labels.Before)
			after := column
			after.Label = columnLabel(column.Label, options.Labels.After)
			delta := column
			delta.Field = panel.Ref(DeltaField(fieldName))
			delta.Label = columnLabel(column.Label, options.Labels.Delta)
			delta.Cell = &panel.TableCellSpec{Kind: panel.TableCellDelta, PercentField: panel.Ref(DeltaPercentField(fieldName))}
			delta.SampleSizeField = ""
			delta.MinSampleSize = 0
			columns = append(columns, before, after, delta)
		}
		spec.Columns = columns
		return
	}
	//nolint:exhaustive // Kinds that support comparison are handled below or via default.
	switch spec.Kind {
	case panel.KindPie, panel.KindDonut, panel.KindMap, panel.KindHistogram, panel.KindBoxPlot, panel.KindHeatmap:
		spec.ComparisonUnsupported = true
		return
	case panel.KindTimeSeries, panel.KindBar, panel.KindHorizontalBar, panel.KindStackedBar:
		if valueField != "" {
			spec.Fields.Previous = panel.Ref(PreviousField(valueField))
		}
	case panel.KindStat:
		if valueField == "" {
			return
		}
		unit := panel.TrendDeltaValue
		if options.AbsoluteDeltaUnit != nil {
			unit = options.AbsoluteDeltaUnit(valueField)
		}
		spec.Trend = &panel.TrendSpec{
			Label:             options.Labels.Trend,
			Polarity:          options.trendPolarity(valueField),
			AbsoluteField:     panel.Ref(DeltaField(valueField)),
			PercentField:      panel.Ref(DeltaPercentField(valueField)),
			AbsoluteDeltaUnit: unit,
		}
	default:
		// Containers and the remaining leaf kinds carry no comparison of their
		// own: a container compares through its children, and a kind that never
		// declares a value field has nothing to compare.
	}
}

// ApplyStatic joins a separately queried baseline into a dashboard whose
// datasets have already been materialized as static frames.
func ApplyStatic(current, baseline *lens.DashboardSpec, options StaticOptions) error {
	if current == nil || baseline == nil {
		return nil
	}
	baselineDatasets := make(map[string]lens.DatasetSpec, len(baseline.Datasets))
	for _, dataset := range baseline.Datasets {
		baselineDatasets[dataset.Name] = dataset
	}
	for index := range current.Datasets {
		base, ok := baselineDatasets[current.Datasets[index].Name]
		if !ok || current.Datasets[index].Static == nil || base.Static == nil {
			continue
		}
		merged, err := mergeFrameSet(
			current.Datasets[index].Static,
			base.Static,
			current.Datasets[index].ComparisonAlignment,
		)
		if err != nil {
			return fmt.Errorf("compare dataset %s: %w", current.Datasets[index].Name, err)
		}
		current.Datasets[index].Static = merged
	}
	maxTableMetrics := options.MaxTableMetrics
	if maxTableMetrics <= 0 {
		maxTableMetrics = defaultMaxTableMetrics
	}
	for rowIndex := range current.Rows {
		for panelIndex := range current.Rows[rowIndex].Panels {
			applyPanel(&current.Rows[rowIndex].Panels[panelIndex], current, options, maxTableMetrics)
		}
	}
	return nil
}

func mergeFrameSet(current, baseline *frame.FrameSet, alignment lens.ComparisonAlignment) (*frame.FrameSet, error) {
	merged := current.Clone()
	if alignment != lens.ComparisonAlignmentInferred && alignment != lens.ComparisonAlignmentOrdinal {
		return nil, fmt.Errorf("unsupported comparison alignment %q", alignment)
	}
	currentFrame := merged.Primary()
	baselineFrame := baseline.Primary()
	if currentFrame == nil {
		return merged, nil
	}
	if baselineFrame == nil {
		if alignment == lens.ComparisonAlignmentOrdinal {
			return appendComparisonMetrics(merged, currentFrame, currentFrame.Rows(), func(int, map[string]any) map[string]any { return nil })
		}
		return merged, nil
	}
	identity := identityFields(currentFrame, baselineFrame)
	baselineRows := baselineFrame.Rows()
	currentRows := currentFrame.Rows()
	if alignment == lens.ComparisonAlignmentOrdinal {
		return mergeOrdinalFrames(merged, currentFrame, baselineFrame, currentRows, baselineRows)
	}
	if len(identity) == 0 && (len(currentRows) > 1 || len(baselineRows) > 1) {
		return nil, fmt.Errorf("multi-row comparison has no stable identity field")
	}
	baselineByKey := make(map[string]map[string]any, len(baselineRows))
	for _, row := range baselineRows {
		baselineByKey[rowKey(row, identity)] = row
	}
	matchedRows := 0
	for _, row := range currentRows {
		if _, ok := baselineByKey[rowKey(row, identity)]; ok {
			matchedRows++
		}
	}
	if len(currentRows) > 0 && len(baselineRows) > 0 && matchedRows == 0 {
		return nil, fmt.Errorf("comparison identity fields %v matched zero rows", identity)
	}
	return appendComparisonMetrics(merged, currentFrame, currentRows, func(_ int, row map[string]any) map[string]any {
		return baselineByKey[rowKey(row, identity)]
	})
}

func mergeOrdinalFrames(
	merged *frame.FrameSet,
	currentFrame, baselineFrame *frame.Frame,
	currentRows, baselineRows []map[string]any,
) (*frame.FrameSet, error) {
	stableFields, err := ordinalStableFields(currentFrame, baselineFrame)
	if err != nil {
		return nil, err
	}
	matchedRows := min(len(currentRows), len(baselineRows))
	for rowIndex := range matchedRows {
		for _, field := range stableFields {
			if !reflect.DeepEqual(currentRows[rowIndex][field], baselineRows[rowIndex][field]) {
				return nil, fmt.Errorf(
					"ordinal comparison field %q mismatch at row %d: current=%v baseline=%v",
					field, rowIndex, currentRows[rowIndex][field], baselineRows[rowIndex][field],
				)
			}
		}
	}

	return appendComparisonMetrics(merged, currentFrame, currentRows, func(rowIndex int, _ map[string]any) map[string]any {
		if rowIndex >= len(baselineRows) {
			return nil
		}
		return baselineRows[rowIndex]
	})
}

func appendComparisonMetrics(
	merged *frame.FrameSet,
	currentFrame *frame.Frame,
	currentRows []map[string]any,
	baselineRow func(rowIndex int, currentRow map[string]any) map[string]any,
) (*frame.FrameSet, error) {
	// Capture source fields before appending derived metrics so repeated range
	// growth can never make a derived field derive another derived field.
	sourceFields := append([]frame.Field(nil), currentFrame.Fields...)
	for _, sourceField := range sourceFields {
		if sourceField.Type != frame.FieldTypeNumber {
			continue
		}
		previousValues := make([]any, len(currentRows))
		deltaValues := make([]any, len(currentRows))
		percentValues := make([]any, len(currentRows))
		for rowIndex, row := range currentRows {
			previous, previousOK := numericCell(baselineRow(rowIndex, row)[sourceField.Name])
			currentValue, currentOK := numericCell(row[sourceField.Name])
			if !previousOK {
				continue
			}
			previousValues[rowIndex] = previous
			if !currentOK {
				continue
			}
			delta := currentValue - previous
			deltaValues[rowIndex] = delta
			if previous != 0 {
				percentValues[rowIndex] = delta / math.Abs(previous) * 100
			}
		}
		currentFrame.Fields = append(currentFrame.Fields,
			frame.Field{Name: PreviousField(sourceField.Name), Type: frame.FieldTypeNumber, Role: frame.RoleMetric, Values: previousValues},
			frame.Field{Name: DeltaField(sourceField.Name), Type: frame.FieldTypeNumber, Role: frame.RoleMetric, Values: deltaValues},
			frame.Field{Name: DeltaPercentField(sourceField.Name), Type: frame.FieldTypeNumber, Role: frame.RoleMetric, Values: percentValues},
		)
	}
	if err := currentFrame.Normalize(); err != nil {
		return nil, err
	}
	return merged, nil
}

// ordinalStableFields returns shared series/group fields that must remain
// equal at each position. A time field, or otherwise the first dimension, is
// the ordered axis. IDs and link parameters are deliberately excluded because
// they commonly encode that axis (for example month|series and a month-scoped
// drill URL) and therefore necessarily differ between comparison periods.
func ordinalStableFields(current, baseline *frame.Frame) ([]string, error) {
	type fieldContract struct {
		fieldType frame.FieldType
		role      frame.FieldRole
	}
	baselineFields := make(map[string]fieldContract, len(baseline.Fields))
	for _, field := range baseline.Fields {
		baselineFields[field.Name] = fieldContract{fieldType: field.Type, role: field.Role}
	}
	ordinalField := ""
	for _, field := range current.Fields {
		baselineField, ok := baselineFields[field.Name]
		if ok && baselineField.fieldType == field.Type && baselineField.role == frame.RoleTime && field.Role == frame.RoleTime {
			ordinalField = field.Name
			break
		}
	}
	if ordinalField == "" {
		for _, field := range current.Fields {
			baselineField, ok := baselineFields[field.Name]
			if ok && baselineField.fieldType == field.Type && baselineField.role == frame.RoleDimension && field.Role == frame.RoleDimension {
				ordinalField = field.Name
				break
			}
		}
	}
	currentStructuralFields := make(map[string]fieldContract)
	for _, field := range current.Fields {
		if isComparisonStructuralRole(field.Role) {
			currentStructuralFields[field.Name] = fieldContract{fieldType: field.Type, role: field.Role}
		}
	}
	for name, contract := range currentStructuralFields {
		baselineContract, ok := baselineFields[name]
		if !ok || baselineContract != contract {
			return nil, fmt.Errorf("ordinal comparison structural field %q is incompatible with baseline", name)
		}
	}
	for _, field := range baseline.Fields {
		if !isComparisonStructuralRole(field.Role) {
			continue
		}
		currentContract, ok := currentStructuralFields[field.Name]
		if !ok || currentContract != (fieldContract{fieldType: field.Type, role: field.Role}) {
			return nil, fmt.Errorf("ordinal comparison structural field %q is incompatible with current dataset", field.Name)
		}
	}
	stable := make([]string, 0)
	for _, field := range current.Fields {
		baselineField, ok := baselineFields[field.Name]
		if !ok || baselineField.fieldType != field.Type || baselineField.role != field.Role {
			continue
		}
		if field.Name == ordinalField {
			continue
		}
		//nolint:exhaustive // Only structural roles can hold a row in place; default covers the rest.
		switch field.Role {
		case frame.RoleTime, frame.RoleDimension, frame.RoleSeries:
			stable = append(stable, field.Name)
		default:
			// Metrics move between periods by definition, and ids and link
			// params address rows rather than describe them, so none of them
			// can hold a row in place across the join.
		}
	}
	return stable, nil
}

func isComparisonStructuralRole(role frame.FieldRole) bool {
	//nolint:exhaustive // Metrics are the values being compared, never structure; default covers them.
	switch role {
	case frame.RoleTime, frame.RoleDimension, frame.RoleID, frame.RoleSeries, frame.RoleLinkParam:
		return true
	default:
		return false
	}
}

func identityFields(current, baseline *frame.Frame) []string {
	type fieldContract struct {
		fieldType frame.FieldType
		role      frame.FieldRole
	}
	baselineFields := make(map[string]fieldContract, len(baseline.Fields))
	for _, field := range baseline.Fields {
		baselineFields[field.Name] = fieldContract{fieldType: field.Type, role: field.Role}
	}
	explicit := make([]string, 0)
	dimensions := make([]string, 0)
	for _, field := range current.Fields {
		baselineField, ok := baselineFields[field.Name]
		if !ok || baselineField.fieldType != field.Type || baselineField.role != field.Role {
			continue
		}
		//nolint:exhaustive // Only ids and dimensions identify a row; default covers the rest.
		switch field.Role {
		case frame.RoleID:
			explicit = append(explicit, field.Name)
		case frame.RoleDimension, frame.RoleSeries:
			dimensions = append(dimensions, field.Name)
		default:
			// Time, metrics and link params never identify a row on their own.
		}
	}
	if len(explicit) > 0 {
		return explicit
	}
	return dimensions
}

func rowKey(row map[string]any, fields []string) string {
	if len(fields) == 0 {
		return "singleton"
	}
	parts := make([]string, len(fields))
	for index, field := range fields {
		parts[index] = fmt.Sprint(row[field])
	}
	return strings.Join(parts, "\x1f")
}

func numericCell(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	default:
		return 0, false
	}
}

func applyPanel(spec *panel.Spec, dashboard *lens.DashboardSpec, options StaticOptions, maxTableMetrics int) {
	if spec == nil {
		return
	}
	for index := range spec.Children {
		applyPanel(&spec.Children[index], dashboard, options, maxTableMetrics)
	}
	if spec.Kind == panel.KindTable {
		columns := make([]panel.TableColumn, 0, len(spec.Columns)*3)
		expandedMetrics := 0
		for _, column := range spec.Columns {
			fieldName := column.Field.Name()
			if !column.ShareOf.Empty() || expandedMetrics >= maxTableMetrics ||
				!dashboardFieldExists(dashboard, spec.Dataset, PreviousField(fieldName)) {
				columns = append(columns, column)
				continue
			}
			expandedMetrics++
			before := column
			before.Field = panel.Ref(PreviousField(fieldName))
			before.Label = columnLabel(column.Label, options.Labels.Before)
			if sampleSize := column.SampleSizeField.Name(); sampleSize != "" &&
				dashboardFieldExists(dashboard, spec.Dataset, PreviousField(sampleSize)) {
				before.SampleSizeField = panel.Ref(PreviousField(sampleSize))
			}
			after := column
			after.Label = columnLabel(column.Label, options.Labels.After)
			delta := column
			delta.Field = panel.Ref(DeltaField(fieldName))
			delta.Label = columnLabel(column.Label, options.Labels.Delta)
			delta.Cell = &panel.TableCellSpec{Kind: panel.TableCellDelta, PercentField: panel.Ref(DeltaPercentField(fieldName))}
			delta.SampleSizeField = ""
			delta.MinSampleSize = 0
			columns = append(columns, before, after, delta)
		}
		spec.Columns = columns
		return
	}
	//nolint:exhaustive // Kinds that support comparison are handled below or via default.
	switch spec.Kind {
	case panel.KindPie, panel.KindDonut, panel.KindMap, panel.KindHistogram, panel.KindBoxPlot, panel.KindHeatmap:
		spec.ComparisonUnsupported = true
		return
	default:
		// Every other kind either supports comparison below or ignores it.
	}
	valueField := spec.Fields.Value.Name()
	if valueField == "" || !dashboardFieldExists(dashboard, spec.Dataset, PreviousField(valueField)) {
		return
	}
	//nolint:exhaustive // Only stat and series kinds carry a comparison; default covers the rest.
	switch spec.Kind {
	case panel.KindStat:
		unit := panel.TrendDeltaValue
		if options.AbsoluteDeltaUnit != nil {
			unit = options.AbsoluteDeltaUnit(valueField)
		}
		spec.Trend = &panel.TrendSpec{
			Label: options.Labels.Trend, Polarity: options.trendPolarity(valueField),
			AbsoluteField:     panel.Ref(DeltaField(valueField)),
			PercentField:      panel.Ref(DeltaPercentField(valueField)),
			AbsoluteDeltaUnit: unit,
		}
	case panel.KindTimeSeries, panel.KindBar, panel.KindHorizontalBar, panel.KindStackedBar:
		spec.Fields.Previous = panel.Ref(PreviousField(valueField))
	default:
		// Containers compare through their children; the rest opt out above.
	}
}

func columnLabel(metric, period string) string {
	metric = strings.TrimSpace(metric)
	period = strings.TrimSpace(period)
	if metric == "" {
		return period
	}
	if period == "" {
		return metric
	}
	return metric + " · " + period
}

func dashboardFieldExists(spec *lens.DashboardSpec, datasetName, fieldName string) bool {
	for _, dataset := range spec.Datasets {
		if dataset.Name != datasetName || dataset.Static == nil || dataset.Static.Primary() == nil {
			continue
		}
		_, ok := dataset.Static.Primary().Field(fieldName)
		return ok
	}
	return false
}
