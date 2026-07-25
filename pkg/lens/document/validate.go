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
	groupDescriptors := make(map[string]LayoutGroup)
	for rowIndex, row := range d.Layout.Rows {
		for _, item := range row.Panels {
			if _, ok := panelIDs[item.PanelID]; !ok {
				return serrors.E(op, fmt.Errorf("layout row %d references missing panel %q", rowIndex, item.PanelID))
			}
			if item.Span < 1 || item.Span > 12 {
				return serrors.E(op, fmt.Errorf("layout panel %s span must be between 1 and 12", item.PanelID))
			}
			if err := validateItemGroups(item, groupDescriptors); err != nil {
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
	if d.Drawer != nil {
		switch d.Drawer.Size {
		case "", DrawerSizeWide:
		default:
			return serrors.E(op, fmt.Errorf("drawer has unsupported size %q", d.Drawer.Size))
		}
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

// validateItemGroups validates an item's container chain. When Groups is
// non-empty it is authoritative and validated outermost → innermost; otherwise
// the legacy singular Group is validated as a one-element chain. A chain must
// not repeat a group ID, and a group ID must carry consistent descriptors
// (kind/label/layout/span) everywhere it appears in the document. Items with no
// grouping validate exactly as before.
func validateItemGroups(item LayoutItem, descriptors map[string]LayoutGroup) error {
	chain := item.Groups
	if len(chain) == 0 {
		if item.Group == nil {
			return nil
		}
		chain = []LayoutGroup{*item.Group}
	}
	seen := make(map[string]struct{}, len(chain))
	for _, group := range chain {
		if err := validateGroupDescriptor(item, group); err != nil {
			return err
		}
		if _, duplicate := seen[group.ID]; duplicate {
			return fmt.Errorf("layout panel %s repeats group %q in its chain", item.PanelID, group.ID)
		}
		seen[group.ID] = struct{}{}
		if prior, ok := descriptors[group.ID]; ok {
			if !sameGroupDescriptor(prior, group) {
				return fmt.Errorf("layout group %s has inconsistent descriptors", group.ID)
			}
		} else {
			descriptors[group.ID] = group
		}
	}
	return nil
}

// sameGroupDescriptor reports whether two occurrences of one group ID declare
// the same container. Tab is per-item and Status may be hoisted, so only the
// structural descriptor (kind/label/layout/span) must match.
func sameGroupDescriptor(a, b LayoutGroup) bool {
	return a.Kind == b.Kind && a.Label == b.Label && a.Layout == b.Layout && a.Span == b.Span
}

func validateGroupDescriptor(item LayoutItem, group LayoutGroup) error {
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
	if err := validateStatus("panel "+panel.ID, panel.Status); err != nil {
		return err
	}
	if panel.Target != nil {
		if panel.Kind != PanelKindCoverage && panel.Kind != PanelKindHBar {
			return fmt.Errorf("panel %s target marker is only supported on coverage and hbar kinds, got %q", panel.ID, panel.Kind)
		}
		if math.IsNaN(panel.Target.Value) || math.IsInf(panel.Target.Value, 0) {
			return fmt.Errorf("panel %s target value must be finite", panel.ID)
		}
	}
	if err := validatePresentation("panel "+panel.ID, panel.Presentation); err != nil {
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
		"annotation": panel.Encoding.Annotation, "tone": panel.Encoding.Tone,
		"share": panel.Encoding.Share, "confidence": panel.Encoding.Confidence,
		"availability": panel.Encoding.Availability,
	} {
		if field != "" && !frameHasColumn(d.Frames[panel.Frame], field) {
			return fmt.Errorf("panel %s %s encoding references missing field %q", panel.ID, role, field)
		}
	}
	if !validConfidence(panel.Confidence) {
		return fmt.Errorf("panel %s has unsupported confidence %q", panel.ID, panel.Confidence)
	}
	if !validAvailability(panel.Availability) {
		return fmt.Errorf("panel %s has unsupported availability %q", panel.ID, panel.Availability)
	}
	if err := d.validateMetricConfigs(panel); err != nil {
		return err
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

// validatePresentation checks a presentation block's enum values. Panels and
// drill levels share it; owner names the carrier in error messages.
func validatePresentation(owner string, presentation *Presentation) error {
	if presentation == nil {
		return nil
	}
	switch presentation.Legend {
	case "", LegendBelow:
	default:
		return fmt.Errorf("%s has unsupported legend placement %q", owner, presentation.Legend)
	}
	switch presentation.SliceLabels {
	case "", SliceLabelsPercent:
	default:
		return fmt.Errorf("%s has unsupported slice labels %q", owner, presentation.SliceLabels)
	}
	switch presentation.TotalBadge {
	case "", TotalBadgeHeader, TotalBadgePlot, TotalBadgeNone:
	default:
		return fmt.Errorf("%s has unsupported total badge placement %q", owner, presentation.TotalBadge)
	}
	switch presentation.ColorBy {
	case "", ColorByCategory:
	default:
		return fmt.Errorf("%s has unsupported color mode %q", owner, presentation.ColorBy)
	}
	switch presentation.Focus {
	case "", FocusModeCanvas:
	default:
		return fmt.Errorf("%s has unsupported focus mode %q", owner, presentation.Focus)
	}
	if presentation.BarWidthPx < 0 {
		return fmt.Errorf("%s bar width cannot be negative", owner)
	}
	return nil
}

// validateStatus checks a status chip. Panels and drill levels share it; owner
// names the carrier in error messages.
func validateStatus(owner string, status *PanelStatus) error {
	if status == nil {
		return nil
	}
	if strings.TrimSpace(status.Label) == "" {
		return fmt.Errorf("%s status requires a label", owner)
	}
	switch status.Tone {
	case "", StatusToneNeutral, StatusTonePositive, StatusToneWarning:
	default:
		return fmt.Errorf("%s has unsupported status tone %q", owner, status.Tone)
	}
	return nil
}

// validateMetricConfigs enforces kind↔config pairing both ways: each metric
// kind requires its own config and carries none of the others; every non-metric
// kind carries none of the three metric configs.
func (d *DashboardDocument) validateMetricConfigs(panel Panel) error {
	hasFlow := panel.MetricFlow != nil
	hasHierarchy := panel.MetricHierarchy != nil
	hasRelationship := panel.MetricRelationship != nil
	switch panel.Kind {
	case PanelKindMetricFlow:
		if !hasFlow {
			return fmt.Errorf("panel %s metric_flow kind requires a metric flow config", panel.ID)
		}
		if hasHierarchy || hasRelationship {
			return fmt.Errorf("panel %s carries a metric config for another kind", panel.ID)
		}
		return d.validateMetricFlow(panel)
	case PanelKindMetricHierarchy:
		if !hasHierarchy {
			return fmt.Errorf("panel %s metric_hierarchy kind requires a metric hierarchy config", panel.ID)
		}
		if hasFlow || hasRelationship {
			return fmt.Errorf("panel %s carries a metric config for another kind", panel.ID)
		}
		return d.validateMetricHierarchy(panel)
	case PanelKindMetricRelationship:
		if !hasRelationship {
			return fmt.Errorf("panel %s metric_relationship kind requires a metric relationship config", panel.ID)
		}
		if hasFlow || hasHierarchy {
			return fmt.Errorf("panel %s carries a metric config for another kind", panel.ID)
		}
		return d.validateMetricRelationship(panel)
	default:
		if hasFlow || hasHierarchy || hasRelationship {
			return fmt.Errorf("panel %s has a metric config for kind %q", panel.ID, panel.Kind)
		}
	}
	return nil
}

// requireMetricEncoding enforces the missing-data contract's mandatory join
// keys: every metric kind must declare id and value encoding roles. Their
// resolution against the frame columns is checked by the shared encoding loop.
func requireMetricEncoding(panel Panel) error {
	if strings.TrimSpace(panel.Encoding.ID) == "" {
		return fmt.Errorf("panel %s %s requires an id encoding", panel.ID, panel.Kind)
	}
	if strings.TrimSpace(panel.Encoding.Value) == "" {
		return fmt.Errorf("panel %s %s requires a value encoding", panel.ID, panel.Kind)
	}
	return nil
}

// validateNoDuplicateFrameKeys enforces the missing-data contract's
// duplicate-key rule: once the id encoding column resolves against the
// panel's frame, no two rows may carry the same key. Key order in the frame
// is irrelevant (join by key, not position); the error names the duplicated
// key so a producer can find the offending row.
func (d *DashboardDocument) validateNoDuplicateFrameKeys(panel Panel) error {
	if strings.TrimSpace(panel.Encoding.ID) == "" {
		return nil
	}
	frame := d.Frames[panel.Frame]
	idIndex := -1
	for index, column := range frame.Columns {
		if column.Name == panel.Encoding.ID {
			idIndex = index
			break
		}
	}
	if idIndex < 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(frame.Rows))
	for _, row := range frame.Rows {
		if idIndex >= len(row) {
			continue
		}
		key := fmt.Sprint(row[idIndex])
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("panel %s %s frame has duplicate key %q", panel.ID, panel.Kind, key)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func (d *DashboardDocument) validateMetricFlow(panel Panel) error {
	config := panel.MetricFlow
	if len(config.Stages) < 2 {
		return fmt.Errorf("panel %s metric flow requires at least 2 stages", panel.ID)
	}
	if err := requireMetricEncoding(panel); err != nil {
		return err
	}
	if err := d.validateNoDuplicateFrameKeys(panel); err != nil {
		return err
	}
	frames := d.panelActionFrames(panel)
	keys := make(map[string]struct{}, len(config.Stages))
	resultCount := 0
	resultIndex := -1
	for index, stage := range config.Stages {
		if strings.TrimSpace(stage.Key) == "" {
			return fmt.Errorf("panel %s metric flow stage %d requires a key", panel.ID, index)
		}
		if _, duplicate := keys[stage.Key]; duplicate {
			return fmt.Errorf("panel %s metric flow has duplicate stage %q", panel.ID, stage.Key)
		}
		keys[stage.Key] = struct{}{}
		if strings.TrimSpace(stage.Label) == "" {
			return fmt.Errorf("panel %s metric flow stage %s requires a label", panel.ID, stage.Key)
		}
		if !validFlowRole(stage.Role) {
			return fmt.Errorf("panel %s metric flow stage %s has unsupported role %q", panel.ID, stage.Key, stage.Role)
		}
		if stage.Role == MetricFlowRoleResult {
			resultCount++
			resultIndex = index
		}
		if !validConfidence(stage.Confidence) {
			return fmt.Errorf("panel %s metric flow stage %s has unsupported confidence %q", panel.ID, stage.Key, stage.Confidence)
		}
		if !validAvailability(stage.Availability) {
			return fmt.Errorf("panel %s metric flow stage %s has unsupported availability %q", panel.ID, stage.Key, stage.Availability)
		}
		if stage.Action != nil {
			owner := fmt.Sprintf("panel %s metric flow stage %s", panel.ID, stage.Key)
			if err := validateAction(owner, *stage.Action); err != nil {
				return err
			}
			if err := validateActionFields(owner, *stage.Action, frames...); err != nil {
				return err
			}
		}
	}
	if resultCount != 1 {
		return fmt.Errorf("panel %s metric flow requires exactly one result stage", panel.ID)
	}
	if resultIndex != len(config.Stages)-1 {
		return fmt.Errorf("panel %s metric flow result stage must be last", panel.ID)
	}
	return nil
}

func (d *DashboardDocument) validateMetricHierarchy(panel Panel) error {
	config := panel.MetricHierarchy
	if len(config.Rows) < 1 {
		return fmt.Errorf("panel %s metric hierarchy requires at least 1 row", panel.ID)
	}
	if err := requireMetricEncoding(panel); err != nil {
		return err
	}
	if err := d.validateNoDuplicateFrameKeys(panel); err != nil {
		return err
	}
	frames := d.panelActionFrames(panel)
	byKey := make(map[string]MetricHierarchyRow, len(config.Rows))
	for index, row := range config.Rows {
		if strings.TrimSpace(row.Key) == "" {
			return fmt.Errorf("panel %s metric hierarchy row %d requires a key", panel.ID, index)
		}
		if _, duplicate := byKey[row.Key]; duplicate {
			return fmt.Errorf("panel %s metric hierarchy has duplicate row %q", panel.ID, row.Key)
		}
		byKey[row.Key] = row
	}
	rootCount := 0
	selectedCount := 0
	unallocatedByParent := make(map[string]int)
	for _, row := range config.Rows {
		if strings.TrimSpace(row.Label) == "" {
			return fmt.Errorf("panel %s metric hierarchy row %s requires a label", panel.ID, row.Key)
		}
		if !validConfidence(row.Confidence) {
			return fmt.Errorf("panel %s metric hierarchy row %s has unsupported confidence %q", panel.ID, row.Key, row.Confidence)
		}
		if !validAvailability(row.Availability) {
			return fmt.Errorf("panel %s metric hierarchy row %s has unsupported availability %q", panel.ID, row.Key, row.Availability)
		}
		if row.Parent == "" {
			rootCount++
		} else {
			if row.Parent == row.Key {
				return fmt.Errorf("panel %s metric hierarchy row %s is its own parent", panel.ID, row.Key)
			}
			if _, ok := byKey[row.Parent]; !ok {
				return fmt.Errorf("panel %s metric hierarchy row %s references missing parent %q", panel.ID, row.Key, row.Parent)
			}
			if row.Unallocated {
				unallocatedByParent[row.Parent]++
				if unallocatedByParent[row.Parent] > 1 {
					return fmt.Errorf("panel %s metric hierarchy parent %s has multiple unallocated children", panel.ID, row.Parent)
				}
			}
		}
		if row.Selected {
			selectedCount++
			if selectedCount > 1 {
				return fmt.Errorf("panel %s metric hierarchy has multiple selected rows", panel.ID)
			}
		}
		if row.Action != nil {
			owner := fmt.Sprintf("panel %s metric hierarchy row %s", panel.ID, row.Key)
			if err := validateAction(owner, *row.Action); err != nil {
				return err
			}
			if err := validateActionFields(owner, *row.Action, frames...); err != nil {
				return err
			}
		}
	}
	if rootCount == 0 {
		return fmt.Errorf("panel %s metric hierarchy requires at least one root row", panel.ID)
	}
	for _, row := range config.Rows {
		depth, err := hierarchyDepth(panel.ID, byKey, row.Key)
		if err != nil {
			return err
		}
		if row.Depth != 0 && row.Depth != depth {
			return fmt.Errorf("panel %s metric hierarchy row %s depth %d does not match derived depth %d", panel.ID, row.Key, row.Depth, depth)
		}
	}
	if config.Reconcile != nil && config.Reconcile.Tolerance < 0 {
		return fmt.Errorf("panel %s metric hierarchy reconcile tolerance cannot be negative", panel.ID)
	}
	return nil
}

// hierarchyDepth walks a row's Parent chain to derive its depth (0 for a root)
// and rejects a cycle. Parent is the authoritative source of the hierarchy;
// Depth on the wire is only validated against this derivation.
func hierarchyDepth(panelID string, byKey map[string]MetricHierarchyRow, key string) (int, error) {
	depth := 0
	seen := make(map[string]struct{})
	current := key
	for {
		row := byKey[current]
		if row.Parent == "" {
			return depth, nil
		}
		if _, cycled := seen[current]; cycled {
			return 0, fmt.Errorf("panel %s metric hierarchy has a cycle at row %q", panelID, key)
		}
		seen[current] = struct{}{}
		current = row.Parent
		depth++
	}
}

func (d *DashboardDocument) validateMetricRelationship(panel Panel) error {
	config := panel.MetricRelationship
	if err := requireMetricEncoding(panel); err != nil {
		return err
	}
	if err := d.validateNoDuplicateFrameKeys(panel); err != nil {
		return err
	}
	frames := d.panelActionFrames(panel)
	for _, side := range []struct {
		role string
		end  MetricRelationshipEnd
	}{{"source", config.Source}, {"target", config.Target}} {
		if strings.TrimSpace(side.end.Key) == "" {
			return fmt.Errorf("panel %s metric relationship %s requires a key", panel.ID, side.role)
		}
		if strings.TrimSpace(side.end.Label) == "" {
			return fmt.Errorf("panel %s metric relationship %s requires a label", panel.ID, side.role)
		}
		if !validConfidence(side.end.Confidence) {
			return fmt.Errorf("panel %s metric relationship %s has unsupported confidence %q", panel.ID, side.role, side.end.Confidence)
		}
		if !validAvailability(side.end.Availability) {
			return fmt.Errorf("panel %s metric relationship %s has unsupported availability %q", panel.ID, side.role, side.end.Availability)
		}
		if side.end.Action != nil {
			owner := fmt.Sprintf("panel %s metric relationship %s", panel.ID, side.role)
			if err := validateAction(owner, *side.end.Action); err != nil {
				return err
			}
			if err := validateActionFields(owner, *side.end.Action, frames...); err != nil {
				return err
			}
		}
	}
	if config.Source.Key == config.Target.Key {
		return fmt.Errorf("panel %s metric relationship source and target must differ", panel.ID)
	}
	if !validRelationshipType(config.Type) {
		return fmt.Errorf("panel %s metric relationship has unsupported type %q", panel.ID, config.Type)
	}
	if !validRelationshipDirection(config.Direction) {
		return fmt.Errorf("panel %s metric relationship has unsupported direction %q", panel.ID, config.Direction)
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
		owner := fmt.Sprintf("drill level %q", key)
		if level.View != "" && !validLevelView(level.View) {
			return fmt.Errorf("%s view must be a chart kind, got %q", owner, level.View)
		}
		if err := validatePresentation(owner, level.Presentation); err != nil {
			return err
		}
		if err := validateStatus(owner, level.Status); err != nil {
			return err
		}
		if level.Source != nil {
			sourceFrame, ok := d.Frames[level.Source.Frame]
			if !ok {
				return fmt.Errorf("%s source references missing frame %q", owner, level.Source.Frame)
			}
			for _, column := range level.Source.Columns {
				if column.Field != "" && !frameHasColumn(sourceFrame, column.Field) {
					return fmt.Errorf("%s source column references missing field %q", owner, column.Field)
				}
			}
			for field := range level.Source.Format {
				if !frameHasColumn(sourceFrame, field) {
					return fmt.Errorf("%s source format references missing field %q", owner, field)
				}
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
		PanelKindLine, PanelKindArea, PanelKindCascade, PanelKindTable, PanelKindCoverage,
		PanelKindMetricFlow, PanelKindMetricHierarchy, PanelKindMetricRelationship:
		return true
	default:
		return false
	}
}

// validLevelView reports whether a drill level may declare kind as its own
// visualization: only chart kinds qualify. Tables live behind LevelSource, and
// stat/metric kinds carry configs a level cannot supply.
func validLevelView(kind PanelKind) bool {
	switch kind {
	case PanelKindPie, PanelKindDonut, PanelKindBar, PanelKindHBar,
		PanelKindLine, PanelKindArea, PanelKindCascade, PanelKindCoverage:
		return true
	default:
		return false
	}
}

func validConfidence(confidence Confidence) bool {
	switch confidence {
	case "", ConfidenceVerified, ConfidenceCalculated, ConfidenceProxy, ConfidenceRequiresReconciliation:
		return true
	default:
		return false
	}
}

func validAvailability(availability Availability) bool {
	switch availability {
	case "", AvailabilityAvailable, AvailabilityConfigRequired, AvailabilityEmptySource, AvailabilityUnavailable:
		return true
	default:
		return false
	}
}

func validFlowRole(role MetricFlowStageRole) bool {
	switch role {
	case MetricFlowRoleInput, MetricFlowRoleAdd, MetricFlowRoleSubtract,
		MetricFlowRoleIntermediate, MetricFlowRoleResult:
		return true
	default:
		return false
	}
}

func validRelationshipType(kind MetricRelationshipType) bool {
	switch kind {
	case MetricRelationshipAssociation, MetricRelationshipDerivation, MetricRelationshipReconciliation:
		return true
	default:
		return false
	}
}

func validRelationshipDirection(direction MetricRelationshipDirection) bool {
	switch direction {
	case "", MetricRelationshipBidirectional, MetricRelationshipSourceToTarget, MetricRelationshipTargetToSource:
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
