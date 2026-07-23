package document

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

func (d *DashboardDocument) Validate() error {
	const op serrors.Op = "lens/document.DashboardDocument.Validate"
	if d == nil {
		return serrors.E(op, fmt.Errorf("document is required"))
	}
	if d.Version != ContractVersion {
		return serrors.E(op, fmt.Errorf("unsupported contract version %q", d.Version))
	}
	if strings.TrimSpace(d.SnapshotID) == "" {
		return serrors.E(op, fmt.Errorf("snapshot id is required"))
	}
	if strings.TrimSpace(d.Meta.DashboardID) == "" {
		return serrors.E(op, fmt.Errorf("dashboard id is required"))
	}
	if d.Drill.InlineDepth < 0 {
		return serrors.E(op, fmt.Errorf("inline depth cannot be negative"))
	}
	panelIDs := make(map[string]struct{}, len(d.Panels))
	for ref, frame := range d.Frames {
		if strings.TrimSpace(string(ref)) == "" {
			return serrors.E(op, fmt.Errorf("frame reference is required"))
		}
		if err := validateFrame(ref, frame); err != nil {
			return serrors.E(op, err)
		}
	}
	for _, panel := range d.Panels {
		if _, duplicate := panelIDs[panel.ID]; duplicate {
			return serrors.E(op, fmt.Errorf("duplicate panel %q", panel.ID))
		}
		panelIDs[panel.ID] = struct{}{}
		if err := d.validatePanel(panel); err != nil {
			return serrors.E(op, err)
		}
	}
	for rowIndex, row := range d.Layout.Rows {
		for _, item := range row.Panels {
			if _, ok := panelIDs[item.PanelID]; !ok {
				return serrors.E(op, fmt.Errorf("layout row %d references missing panel %q", rowIndex, item.PanelID))
			}
			if item.Span < 1 || item.Span > 12 {
				return serrors.E(op, fmt.Errorf("layout panel %s span must be between 1 and 12", item.PanelID))
			}
			if err := validateLayoutGroup(item); err != nil {
				return serrors.E(op, err)
			}
		}
	}
	if err := d.validateDrill(); err != nil {
		return serrors.E(op, err)
	}
	if err := validateFilters(d.Filters); err != nil {
		return serrors.E(op, err)
	}
	perspectiveIDs := make(map[string]struct{}, len(d.Perspectives))
	for _, perspective := range d.Perspectives {
		if strings.TrimSpace(perspective.ID) == "" {
			return serrors.E(op, fmt.Errorf("perspective id is required"))
		}
		if _, duplicate := perspectiveIDs[perspective.ID]; duplicate {
			return serrors.E(op, fmt.Errorf("duplicate perspective %q", perspective.ID))
		}
		perspectiveIDs[perspective.ID] = struct{}{}
		if !validSemantics(perspective.Semantics) {
			return serrors.E(op, fmt.Errorf("perspective %s has unsupported semantics %q", perspective.ID, perspective.Semantics))
		}
		if _, ok := d.Drill.Edges[perspective.Root]; !ok {
			return serrors.E(op, fmt.Errorf("perspective %s references missing root %q", perspective.ID, perspective.Root))
		}
	}
	for key, level := range d.Drill.Edges {
		for _, ref := range level.Perspectives {
			if _, ok := perspectiveIDs[ref.ID]; !ok {
				return serrors.E(op, fmt.Errorf("drill level %q references missing perspective %q", key, ref.ID))
			}
			perspective := findPerspective(d.Perspectives, ref.ID)
			if perspective.Semantics == SemanticsPartition && level.Frame != "" {
				if level.Encoding == nil {
					return serrors.E(op, fmt.Errorf("partition drill level %q requires an encoding", key))
				}
				if err := validatePartitionFrame("drill level "+string(key), *level.Encoding, d.Frames[level.Frame]); err != nil {
					return serrors.E(op, err)
				}
			}
		}
	}
	return nil
}

// PeriodDateLayout is the wire layout of every period-filter date string.
const PeriodDateLayout = "2006-01-02"

func validPeriodDate(raw string) bool {
	if raw == "" {
		return true
	}
	parsed, err := time.Parse(PeriodDateLayout, raw)
	return err == nil && parsed.Format(PeriodDateLayout) == raw
}

func validateFilters(filters []Filter) error {
	ids := make(map[string]struct{}, len(filters))
	for _, filter := range filters {
		if strings.TrimSpace(filter.ID) == "" {
			return fmt.Errorf("filter id is required")
		}
		if _, duplicate := ids[filter.ID]; duplicate {
			return fmt.Errorf("duplicate filter %q", filter.ID)
		}
		ids[filter.ID] = struct{}{}
		switch filter.Kind {
		case FilterKindPeriod:
			if filter.Period == nil {
				return fmt.Errorf("filter %s requires a period payload", filter.ID)
			}
			if err := validatePeriodFilter(filter.ID, *filter.Period); err != nil {
				return err
			}
		default:
			return fmt.Errorf("filter %s has unsupported kind %q", filter.ID, filter.Kind)
		}
	}
	return nil
}

func validatePeriodFilter(id string, period PeriodFilter) error {
	if strings.TrimSpace(period.StartParam) == "" || strings.TrimSpace(period.EndParam) == "" {
		return fmt.Errorf("filter %s requires start and end parameter names", id)
	}
	if period.StartParam == period.EndParam {
		return fmt.Errorf("filter %s start and end parameters must differ", id)
	}
	if err := validatePeriodValue(id+" value", period.Value, period.AllowEmpty); err != nil {
		return err
	}
	if !validPeriodDate(period.Min) || !validPeriodDate(period.Max) {
		return fmt.Errorf("filter %s min/max must be %s dates", id, PeriodDateLayout)
	}
	if period.Min != "" && period.Max != "" && period.Max < period.Min {
		return fmt.Errorf("filter %s max precedes min", id)
	}
	presetIDs := make(map[string]struct{}, len(period.Presets))
	for _, preset := range period.Presets {
		if strings.TrimSpace(preset.ID) == "" {
			return fmt.Errorf("filter %s preset id is required", id)
		}
		if _, duplicate := presetIDs[preset.ID]; duplicate {
			return fmt.Errorf("filter %s has duplicate preset %q", id, preset.ID)
		}
		presetIDs[preset.ID] = struct{}{}
		if strings.TrimSpace(preset.Label) == "" {
			return fmt.Errorf("filter %s preset %s requires a label", id, preset.ID)
		}
		if err := validatePeriodValue(fmt.Sprintf("%s preset %s", id, preset.ID), preset.Value, period.AllowEmpty); err != nil {
			return err
		}
	}
	return nil
}

func validatePeriodValue(owner string, value PeriodValue, allowEmpty bool) error {
	if !validPeriodDate(value.Start) || !validPeriodDate(value.End) {
		return fmt.Errorf("filter %s must use %s dates", owner, PeriodDateLayout)
	}
	if !allowEmpty && (value.Start == "" || value.End == "") {
		return fmt.Errorf("filter %s has an open boundary but the filter does not allow empty", owner)
	}
	if value.Start != "" && value.End != "" && value.End < value.Start {
		return fmt.Errorf("filter %s end precedes start", owner)
	}
	return nil
}

func validateLayoutGroup(item LayoutItem) error {
	group := item.Group
	if group == nil {
		return nil
	}
	if strings.TrimSpace(group.ID) == "" {
		return fmt.Errorf("layout panel %s group id is required", item.PanelID)
	}
	switch group.Kind {
	case LayoutGroupMetrics:
		switch group.Layout {
		case "", LayoutGroupColumns, LayoutGroupRows:
		default:
			return fmt.Errorf("layout group %s has unsupported layout %q", group.ID, group.Layout)
		}
	case LayoutGroupTabs:
		if strings.TrimSpace(group.Tab) == "" {
			return fmt.Errorf("layout panel %s in tabs group %s requires a tab", item.PanelID, group.ID)
		}
	default:
		return fmt.Errorf("layout group %s has unsupported kind %q", group.ID, group.Kind)
	}
	if group.Span < 1 || group.Span > 12 {
		return fmt.Errorf("layout group %s span must be between 1 and 12", group.ID)
	}
	return nil
}

func (d *DashboardDocument) validatePanel(panel Panel) error {
	if strings.TrimSpace(panel.ID) == "" {
		return fmt.Errorf("panel id is required")
	}
	if _, ok := d.Frames[panel.Frame]; !ok {
		return fmt.Errorf("panel %s references missing frame %q", panel.ID, panel.Frame)
	}
	if !validPanelKind(panel.Kind) {
		return fmt.Errorf("panel %s has unsupported kind %q", panel.ID, panel.Kind)
	}
	if !validSemantics(panel.Semantics) {
		return fmt.Errorf("panel %s has unsupported semantics %q", panel.ID, panel.Semantics)
	}
	if panel.Semantics == SemanticsReconciliation && (panel.Kind == PanelKindPie || panel.Kind == PanelKindDonut) {
		return fmt.Errorf("panel %s reconciliation semantics cannot use %s encoding", panel.ID, panel.Kind)
	}
	if panel.Semantics == SemanticsEvidence && !hasLeafAction(panel.Actions) && !hasLeafTableColumnAction(panel.Columns) {
		return fmt.Errorf("panel %s evidence semantics requires a leaf action", panel.ID)
	}
	if panel.DrillRoot != nil {
		if _, ok := d.Drill.Edges[*panel.DrillRoot]; !ok {
			return fmt.Errorf("panel %s references missing drill root %q", panel.ID, *panel.DrillRoot)
		}
	}
	if panel.Status != nil {
		if strings.TrimSpace(panel.Status.Label) == "" {
			return fmt.Errorf("panel %s status requires a label", panel.ID)
		}
		switch panel.Status.Tone {
		case "", StatusToneNeutral, StatusTonePositive, StatusToneWarning:
		default:
			return fmt.Errorf("panel %s has unsupported status tone %q", panel.ID, panel.Status.Tone)
		}
	}
	if err := validatePresentation(panel); err != nil {
		return err
	}
	for field, format := range panel.Format {
		if strings.TrimSpace(field) == "" {
			return fmt.Errorf("panel %s has a format with an empty field", panel.ID)
		}
		if format.Kind == FormatMoney && strings.TrimSpace(format.Currency) == "" {
			return fmt.Errorf("panel %s money field %s requires currency", panel.ID, field)
		}
		switch format.Kind {
		case FormatMoney, FormatPercent, FormatDate, FormatNumber, FormatString:
		default:
			return fmt.Errorf("panel %s field %s has unsupported format %q", panel.ID, field, format.Kind)
		}
		if !frameHasColumn(d.Frames[panel.Frame], field) {
			return fmt.Errorf("panel %s format references missing field %q", panel.ID, field)
		}
	}
	for role, field := range map[string]string{
		"label": panel.Encoding.Label, "value": panel.Encoding.Value, "id": panel.Encoding.ID,
		"series": panel.Encoding.Series, "category": panel.Encoding.Category, "cut": panel.Encoding.Cut,
		"cutLabel": panel.Encoding.CutLabel, "final": panel.Encoding.Final,
	} {
		if field != "" && !frameHasColumn(d.Frames[panel.Frame], field) {
			return fmt.Errorf("panel %s %s encoding references missing field %q", panel.ID, role, field)
		}
	}
	if panel.Semantics == SemanticsPartition {
		if err := validatePartitionFrame("panel "+panel.ID, panel.Encoding, d.Frames[panel.Frame]); err != nil {
			return err
		}
	}
	// A panel-level action is resolved against the rows currently on screen,
	// which for a drillable panel are the current level's frame — not the
	// root frame. Accept a field that exists on any frame the panel can show.
	actionFrames := d.panelActionFrames(panel)
	for _, action := range panel.Actions {
		if err := validateAction(panel.ID, action); err != nil {
			return err
		}
		if err := validateActionFields(panel.ID, action, actionFrames...); err != nil {
			return err
		}
	}
	if panel.Kind != PanelKindTable && len(panel.Columns) > 0 {
		return fmt.Errorf("panel %s has table columns for kind %q", panel.ID, panel.Kind)
	}
	if panel.Kind == PanelKindTable {
		if err := validateTableColumns(panel, d.Frames[panel.Frame]); err != nil {
			return err
		}
	}
	return nil
}

func validatePresentation(panel Panel) error {
	presentation := panel.Presentation
	if presentation == nil {
		return nil
	}
	switch presentation.Legend {
	case "", LegendBelow:
	default:
		return fmt.Errorf("panel %s has unsupported legend placement %q", panel.ID, presentation.Legend)
	}
	switch presentation.SliceLabels {
	case "", SliceLabelsPercent:
	default:
		return fmt.Errorf("panel %s has unsupported slice labels %q", panel.ID, presentation.SliceLabels)
	}
	switch presentation.TotalBadge {
	case "", TotalBadgeHeader, TotalBadgePlot, TotalBadgeNone:
	default:
		return fmt.Errorf("panel %s has unsupported total badge placement %q", panel.ID, presentation.TotalBadge)
	}
	switch presentation.ColorBy {
	case "", ColorByCategory:
	default:
		return fmt.Errorf("panel %s has unsupported color mode %q", panel.ID, presentation.ColorBy)
	}
	if presentation.BarWidthPx < 0 {
		return fmt.Errorf("panel %s bar width cannot be negative", panel.ID)
	}
	return nil
}

func validateTableColumns(panel Panel, frame Frame) error {
	if panel.Presentation != nil && panel.Presentation.RowGroupField != "" &&
		!frameHasColumn(frame, panel.Presentation.RowGroupField) {
		return fmt.Errorf("panel %s references missing row group field %q", panel.ID, panel.Presentation.RowGroupField)
	}
	fields := make(map[string]struct{}, len(panel.Columns))
	for index, column := range panel.Columns {
		owner := fmt.Sprintf("panel %s table column %d", panel.ID, index)
		if strings.TrimSpace(column.Field) == "" {
			if column.Action == nil {
				return fmt.Errorf("%s requires a field or action", owner)
			}
		} else {
			if _, duplicate := fields[column.Field]; duplicate {
				return fmt.Errorf("panel %s has duplicate table column %q", panel.ID, column.Field)
			}
			fields[column.Field] = struct{}{}
			if !frameHasColumn(frame, column.Field) {
				return fmt.Errorf("%s references missing field %q", owner, column.Field)
			}
		}
		switch column.Align {
		case "", TableAlignLeft, TableAlignRight:
		default:
			return fmt.Errorf("%s has unsupported alignment %q", owner, column.Align)
		}
		if column.WidthPx < 0 {
			return fmt.Errorf("%s width cannot be negative", owner)
		}
		if column.Clamp < 0 {
			return fmt.Errorf("%s clamp cannot be negative", owner)
		}
		switch column.Affordance {
		case "", TableAffordancePill, TableAffordanceQuiet:
		default:
			return fmt.Errorf("%s has unsupported affordance %q", owner, column.Affordance)
		}
		if column.BadgeField != "" && !frameHasColumn(frame, column.BadgeField) {
			return fmt.Errorf("%s references missing badge field %q", owner, column.BadgeField)
		}
		if column.Cell.ToneField != "" && !frameHasColumn(frame, column.Cell.ToneField) {
			return fmt.Errorf("%s references missing tone field %q", owner, column.Cell.ToneField)
		}
		switch column.Cell.Layout {
		case "", TableCellStacked:
		default:
			return fmt.Errorf("%s has unsupported cell layout %q", owner, column.Cell.Layout)
		}
		switch column.Cell.Kind {
		case TableCellPlain, TableCellBar, TableCellUnderline:
			if column.Cell.SecondaryField != "" {
				return fmt.Errorf("%s %s cell cannot have a secondary field", owner, column.Cell.Kind)
			}
		case TableCellDelta:
			if strings.TrimSpace(column.Cell.SecondaryField) == "" {
				return fmt.Errorf("%s delta cell requires a secondary field", owner)
			}
			if !frameHasColumn(frame, column.Cell.SecondaryField) {
				return fmt.Errorf("%s delta cell references missing secondary field %q", owner, column.Cell.SecondaryField)
			}
		default:
			return fmt.Errorf("%s has unsupported cell kind %q", owner, column.Cell.Kind)
		}
		if column.Action != nil {
			if err := validateAction(owner, *column.Action); err != nil {
				return err
			}
			if err := validateActionFields(owner, *column.Action, frame); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *DashboardDocument) validateDrill() error {
	paths := make(map[string]NodeKey)
	for key, level := range d.Drill.Edges {
		if err := validNodeKey("drill level", key); err != nil {
			return err
		}
		if err := validateNodePath("drill level", key, level.Path); err != nil {
			return err
		}
		if level.Frame != "" {
			if _, ok := d.Frames[level.Frame]; !ok {
				return fmt.Errorf("drill level %q references missing frame %q", key, level.Frame)
			}
		}
		seen := make(map[NodeKey]struct{}, len(level.Children))
		if level.DynamicChildren != nil {
			if err := validateDynamicChildren(string(key), *level.DynamicChildren); err != nil {
				return err
			}
			if level.Frame != "" {
				if err := validateDynamicChildFields(string(key), *level.DynamicChildren, d.Frames[level.Frame]); err != nil {
					return err
				}
				if err := ValidateResolvedChildren(level, d.Frames[level.Frame], d.Drill.Edges); err != nil {
					return err
				}
			}
			if level.DynamicChildren.Target != nil && level.DynamicChildren.Target.Kind == ValueSourceLiteral {
				target, ok := level.DynamicChildren.Target.Value.(string)
				if !ok || strings.TrimSpace(target) == "" {
					return fmt.Errorf("drill level %q dynamic child literal target must be a nonblank string", key)
				}
				resolved := dynamicTarget(level, target)
				if _, ok := d.Drill.Edges[resolved]; !ok {
					return fmt.Errorf("drill level %q dynamic children reference missing target %q", key, resolved)
				}
			}
		}
		for _, child := range level.Children {
			if err := validNodeKey("drill child", child.Key); err != nil {
				return err
			}
			if _, duplicate := seen[child.Key]; duplicate {
				return fmt.Errorf("drill level %q has duplicate child key %q", key, child.Key)
			}
			seen[child.Key] = struct{}{}
			if err := validateChildPath(key, level.Path, child); err != nil {
				return err
			}
			pathID := pathIdentity(child.Path)
			if previous, duplicate := paths[pathID]; duplicate {
				return fmt.Errorf("drill child %q duplicates full path used by %q", child.Key, previous)
			}
			paths[pathID] = child.Key
			if child.Target != "" {
				if _, ok := d.Drill.Edges[child.Target]; !ok {
					return fmt.Errorf("drill child %q references missing target %q", child.Key, child.Target)
				}
			}
			if child.Action != nil {
				if err := validateAction(string(child.Key), *child.Action); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func validateDynamicChildren(owner string, declaration DynamicChildren) error {
	if declaration.Key.Kind != ValueSourceField || strings.TrimSpace(declaration.Key.Name) == "" {
		return fmt.Errorf("drill level %q dynamic child key requires a field source", owner)
	}
	if declaration.Label.Kind != ValueSourceField || strings.TrimSpace(declaration.Label.Name) == "" {
		return fmt.Errorf("drill level %q dynamic child label requires a field source", owner)
	}
	if (declaration.Target == nil) == (declaration.Action == nil) {
		return fmt.Errorf("drill level %q dynamic children require exactly one of target or action", owner)
	}
	if declaration.Target != nil {
		if declaration.Target.Kind != ValueSourceField && declaration.Target.Kind != ValueSourceLiteral {
			return fmt.Errorf("drill level %q dynamic child target requires a field or literal source", owner)
		}
		if err := validateSource(owner, *declaration.Target); err != nil {
			return err
		}
	}
	if declaration.Action != nil {
		if err := validateAction(owner, *declaration.Action); err != nil {
			return err
		}
	}
	return nil
}

func validateDynamicChildFields(owner string, declaration DynamicChildren, frame Frame) error {
	for _, source := range []Source{declaration.Key, declaration.Label} {
		if !frameHasColumn(frame, source.Name) {
			return fmt.Errorf("drill level %q dynamic children reference missing field %q", owner, source.Name)
		}
	}
	if declaration.Target != nil && declaration.Target.Kind == ValueSourceField && !frameHasColumn(frame, declaration.Target.Name) {
		return fmt.Errorf("drill level %q dynamic children reference missing field %q", owner, declaration.Target.Name)
	}
	if declaration.Action != nil {
		return validateActionFields(owner, *declaration.Action, frame)
	}
	return nil
}

func validateFrame(ref FrameRef, frame Frame) error {
	names := make(map[string]struct{}, len(frame.Columns))
	for _, column := range frame.Columns {
		if strings.TrimSpace(column.Name) == "" {
			return fmt.Errorf("frame %q has a column without a name", ref)
		}
		if _, duplicate := names[column.Name]; duplicate {
			return fmt.Errorf("frame %q has duplicate column %q", ref, column.Name)
		}
		names[column.Name] = struct{}{}
		switch column.Type {
		case ColumnString, ColumnNumber, ColumnBool, ColumnTime:
		default:
			return fmt.Errorf("frame %q column %s has unsupported type %q", ref, column.Name, column.Type)
		}
	}
	for rowIndex, row := range frame.Rows {
		if len(row) != len(frame.Columns) {
			return fmt.Errorf("frame %q row %d has %d cells, expected %d", ref, rowIndex, len(row), len(frame.Columns))
		}
		for columnIndex, cell := range row {
			if err := validateCell(frame.Columns[columnIndex], cell); err != nil {
				return fmt.Errorf("frame %q row %d column %s: %w", ref, rowIndex, frame.Columns[columnIndex].Name, err)
			}
		}
	}
	return nil
}

func validateCell(column Column, value any) error {
	if value == nil {
		return nil
	}
	switch column.Type {
	case ColumnString:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
	case ColumnNumber:
		number, ok := numericValue(value)
		if !ok {
			return fmt.Errorf("expected number, got %T", value)
		}
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return fmt.Errorf("number must be finite")
		}
	case ColumnBool:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected bool, got %T", value)
		}
	case ColumnTime:
		switch value.(type) {
		case time.Time, string:
		default:
			return fmt.Errorf("expected time, got %T", value)
		}
	}
	return nil
}

func validatePartitionFrame(owner string, encoding Encoding, frame Frame) error {
	valueIndex := -1
	for index, column := range frame.Columns {
		if column.Name == encoding.Value {
			valueIndex = index
			break
		}
	}
	if valueIndex < 0 {
		return fmt.Errorf("%s partition value field %q is missing", owner, encoding.Value)
	}
	for rowIndex, row := range frame.Rows {
		value, ok := numericValue(row[valueIndex])
		if !ok || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return fmt.Errorf("%s partition value row %d must be finite and non-negative", owner, rowIndex)
		}
	}
	return nil
}

func frameHasColumn(frame Frame, name string) bool {
	for _, column := range frame.Columns {
		if column.Name == name {
			return true
		}
	}
	return false
}

func validateAction(owner string, action Action) error {
	switch action.Kind {
	case ActionNavigate, ActionNavigateToLeaf, ActionOpenDrawer:
		if strings.TrimSpace(action.URLTemplate) == "" && action.URLSource == nil {
			return fmt.Errorf("%s action requires url", owner)
		}
	case ActionEmitEvent:
		if strings.TrimSpace(action.Event) == "" {
			return fmt.Errorf("%s emit action requires event", owner)
		}
	default:
		return fmt.Errorf("%s has unsupported action kind %q", owner, action.Kind)
	}
	if action.URLSource != nil {
		if err := validateSource(owner, *action.URLSource); err != nil {
			return err
		}
	}
	params := make(map[string]struct{}, len(action.Params))
	for _, param := range action.Params {
		if strings.TrimSpace(param.Name) == "" {
			return fmt.Errorf("%s action parameter name is required", owner)
		}
		if _, duplicate := params[param.Name]; duplicate {
			return fmt.Errorf("%s action has duplicate parameter %q", owner, param.Name)
		}
		params[param.Name] = struct{}{}
		if err := validateSource(owner, param.Source); err != nil {
			return err
		}
	}
	for _, source := range action.Payload {
		if err := validateSource(owner, source); err != nil {
			return err
		}
	}
	return nil
}

// panelActionFrames lists every frame a panel's rows can be drawn from: its
// own frame plus the frames of every drill level reachable from its root.
func (d *DashboardDocument) panelActionFrames(panel Panel) []Frame {
	frames := []Frame{d.Frames[panel.Frame]}
	if panel.DrillRoot == nil {
		return frames
	}
	seen := make(map[NodeKey]struct{})
	var walk func(key NodeKey)
	walk = func(key NodeKey) {
		if _, visited := seen[key]; visited {
			return
		}
		seen[key] = struct{}{}
		level, ok := d.Drill.Edges[key]
		if !ok {
			return
		}
		if level.Frame != "" {
			frames = append(frames, d.Frames[level.Frame])
		}
		for _, ref := range level.Perspectives {
			walk(findPerspective(d.Perspectives, ref.ID).Root)
		}
		for _, child := range level.Children {
			if child.Target != "" {
				walk(child.Target)
			}
		}
	}
	walk(*panel.DrillRoot)
	return frames
}

func validateActionFields(owner string, action Action, frames ...Frame) error {
	validate := func(source Source) error {
		if source.Kind != ValueSourceField {
			return nil
		}
		for _, frame := range frames {
			if frameHasColumn(frame, source.Name) {
				return nil
			}
		}
		return fmt.Errorf("%s action references missing field %q", owner, source.Name)
	}
	if action.URLSource != nil {
		if err := validate(*action.URLSource); err != nil {
			return err
		}
	}
	for _, param := range action.Params {
		if err := validate(param.Source); err != nil {
			return err
		}
	}
	for _, source := range action.Payload {
		if err := validate(source); err != nil {
			return err
		}
	}
	return nil
}

func validateSource(owner string, source Source) error {
	switch source.Kind {
	case ValueSourceField, ValueSourceVariable:
		if strings.TrimSpace(source.Name) == "" {
			return fmt.Errorf("%s action source name is required", owner)
		}
	case ValueSourceLiteral:
		if source.Value == nil {
			return fmt.Errorf("%s action literal value is required", owner)
		}
	default:
		return fmt.Errorf("%s action has unsupported value source %q", owner, source.Kind)
	}
	return nil
}

func hasLeafAction(actions []Action) bool {
	for _, action := range actions {
		if action.Kind == ActionNavigateToLeaf || action.Kind == ActionOpenDrawer {
			return true
		}
	}
	return false
}

func hasLeafTableColumnAction(columns []TableColumn) bool {
	for _, column := range columns {
		if column.Action != nil && (column.Action.Kind == ActionNavigateToLeaf || column.Action.Kind == ActionOpenDrawer) {
			return true
		}
	}
	return false
}

func validPanelKind(kind PanelKind) bool {
	switch kind {
	case PanelKindStat, PanelKindPie, PanelKindDonut, PanelKindBar, PanelKindHBar,
		PanelKindLine, PanelKindArea, PanelKindCascade, PanelKindTable, PanelKindCoverage:
		return true
	default:
		return false
	}
}

func validSemantics(semantics Semantics) bool {
	switch semantics {
	case SemanticsPartition, SemanticsReconciliation, SemanticsSeries, SemanticsEvidence:
		return true
	default:
		return false
	}
}

func validNodeKey(owner string, key NodeKey) error {
	trimmed := strings.TrimSpace(string(key))
	if trimmed == "" {
		return fmt.Errorf("%s key is required", owner)
	}
	if trimmed != string(key) {
		return fmt.Errorf("%s key %q has surrounding whitespace", owner, key)
	}
	return nil
}

func validateNodePath(owner string, key NodeKey, path NodePath) error {
	if len(path) == 0 || path[len(path)-1] != key {
		return fmt.Errorf("%s %q has an invalid full path", owner, key)
	}
	for _, segment := range path {
		if err := validNodeKey(owner+" path", segment); err != nil {
			return err
		}
	}
	return nil
}

func validateChildPath(parentKey NodeKey, parentPath NodePath, child Node) error {
	if err := validateNodePath("drill child", child.Key, child.Path); err != nil {
		return err
	}
	if len(child.Path) != len(parentPath)+1 {
		return fmt.Errorf("drill child %q path must extend parent level %q path", child.Key, parentKey)
	}
	for index, segment := range parentPath {
		if child.Path[index] != segment {
			return fmt.Errorf("drill child %q path must extend parent level %q path", child.Key, parentKey)
		}
	}
	return nil
}

func findPerspective(perspectives []Perspective, id string) Perspective {
	for _, perspective := range perspectives {
		if perspective.ID == id {
			return perspective
		}
	}
	return Perspective{}
}

func pathIdentity(path NodePath) string {
	var builder strings.Builder
	for _, key := range path {
		fmt.Fprintf(&builder, "%d:%s;", len(key), key)
	}
	return builder.String()
}

func numericValue(value any) (float64, bool) {
	switch number := value.(type) {
	case int:
		return float64(number), true
	case int8:
		return float64(number), true
	case int16:
		return float64(number), true
	case int32:
		return float64(number), true
	case int64:
		return float64(number), true
	case uint:
		return float64(number), true
	case uint8:
		return float64(number), true
	case uint16:
		return float64(number), true
	case uint32:
		return float64(number), true
	case uint64:
		return float64(number), true
	case float32:
		return float64(number), true
	case float64:
		return number, true
	default:
		return 0, false
	}
}
