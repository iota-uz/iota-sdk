package document

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/explore"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	sdkmoney "github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type BuildOptions struct {
	SnapshotID  string
	GeneratedAt time.Time
	Locale      string
	InlineDepth int
	Endpoints   Endpoints
	I18n        map[string]string
	Theme       Theme
	// Header supplies the optional document identity block rendered above the
	// dashboard body. Drawer-hosted documents suppress it because drawer chrome
	// owns that identity.
	Header *DocumentHeader
	// Filters declares the dashboard's controls; the producer supplies the
	// normalized current value and localized labels. See Filter.
	Filters []Filter
	// Drawer, when set, marks this document as drawer-hosted: the drawer chrome
	// renders this identity block once in its top bar, and the document's own
	// dashboard heading is suppressed (Meta.Title cleared) so the heading is not
	// stated twice.
	Drawer *DrawerHeader
}

func Build(spec lens.DashboardSpec, result *runtime.Result, opts BuildOptions) (*DashboardDocument, error) {
	const op serrors.Op = "lens/document.Build"
	if result == nil {
		return nil, serrors.E(op, fmt.Errorf("runtime result is required"))
	}
	if err := runtime.Validate(spec); err != nil {
		return nil, serrors.E(op, err)
	}
	if opts.InlineDepth < 0 {
		return nil, serrors.E(op, fmt.Errorf("inline depth cannot be negative"))
	}

	snapshotID := strings.TrimSpace(opts.SnapshotID)
	if snapshotID == "" {
		var err error
		snapshotID, err = newSnapshotID()
		if err != nil {
			return nil, serrors.E(op, err)
		}
	}
	generatedAt := opts.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = result.StartedAt
	}
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	locale := strings.TrimSpace(opts.Locale)
	if locale == "" {
		locale = result.Locale
	}

	doc := &DashboardDocument{
		Version:      ContractVersion,
		SnapshotID:   snapshotID,
		Meta:         Meta{DashboardID: spec.ID, Title: spec.Title, GeneratedAt: generatedAt, Locale: locale},
		Header:       opts.Header,
		Layout:       Layout{Rows: make([]LayoutRow, 0, len(spec.Rows))},
		Panels:       make([]Panel, 0),
		Frames:       make(map[FrameRef]Frame),
		Drill:        Drill{Edges: make(map[NodeKey]Level), InlineDepth: opts.InlineDepth},
		Perspectives: make([]Perspective, 0),
		Filters:      cloneFilters(opts.Filters),
		Endpoints:    opts.Endpoints,
		I18n:         cloneStrings(opts.I18n),
		Theme:        cloneTheme(opts.Theme),
	}
	if doc.I18n == nil {
		doc.I18n = make(map[string]string)
	}
	if opts.Drawer != nil {
		doc.Drawer = opts.Drawer
		doc.Header = nil
		// The drawer chrome owns the heading; clearing the document title stops
		// the dashboard body from repeating it. "An empty title lets a host page
		// own the heading" is the renderer's documented contract for this.
		doc.Meta.Title = ""
	}

	hosts := explorerHosts(spec.Explorers)
	for _, rowSpec := range spec.Rows {
		layoutRow := LayoutRow{Heading: rowSpec.Heading, Class: rowSpec.Class, Panels: make([]LayoutItem, 0)}
		for _, panelSpec := range rowSpec.Panels {
			if err := appendPanelTree(doc, panelSpec, result, hosts, &layoutRow, nil); err != nil {
				return nil, serrors.E(op, err)
			}
		}
		doc.Layout.Rows = append(doc.Layout.Rows, layoutRow)
	}
	for _, explorerSpec := range spec.Explorers {
		if err := buildExplorer(doc, explorerSpec, result); err != nil {
			return nil, serrors.E(op, err)
		}
	}
	if err := doc.Validate(); err != nil {
		return nil, serrors.E(op, err)
	}
	return doc, nil
}

// appendPanelTree walks a panel spec tree, flattening it into wire panels and
// layout items. groups is the nested-container chain accumulated so far,
// ordered outermost -> innermost: StatGroup and Tabs each append their own
// descriptor (Tabs additionally sets the per-child Tab on its own copy); every
// other container passes the chain through unchanged. Each recursive step
// clones the chain before appending so sibling subtrees never alias one
// another's slice.
func appendPanelTree(
	doc *DashboardDocument,
	spec panel.Spec,
	result *runtime.Result,
	hosts map[string]explore.Spec,
	row *LayoutRow,
	groups []LayoutGroup,
) error {
	if spec.Kind.IsContainer() {
		//nolint:exhaustive // Only stat groups and tabs become wire containers; the rest flatten.
		switch spec.Kind {
		case panel.KindStatGroup:
			descriptor := LayoutGroup{
				ID: spec.ID, Kind: LayoutGroupMetrics, Label: spec.Title,
				Layout: groupLayout(spec.GroupLayout), Span: containerSpan(spec),
				// A uniform-status group hoists its single chip to the heading
				// row; per-metric chips then drop for the members below.
				Status: buildStatus(spec),
			}
			chain := appendGroup(groups, descriptor)
			for _, child := range spec.Children {
				if err := appendPanelTree(doc, child, result, hosts, row, chain); err != nil {
					return err
				}
			}
			return nil
		case panel.KindTabs:
			base := LayoutGroup{ID: spec.ID, Kind: LayoutGroupTabs, Label: spec.Title, Span: containerSpan(spec)}
			for index, child := range spec.Children {
				tab := base
				tab.Tab = strings.TrimSpace(child.Title)
				if tab.Tab == "" {
					tab.Tab = fmt.Sprintf("%s %d", spec.ID, index+1)
				}
				chain := appendGroup(groups, tab)
				if err := appendPanelTree(doc, child, result, hosts, row, chain); err != nil {
					return err
				}
			}
			return nil
		default:
			for _, child := range spec.Children {
				if err := appendPanelTree(doc, child, result, hosts, row, groups); err != nil {
					return err
				}
			}
			return nil
		}
	}
	kind, err := panelKind(spec.Kind)
	if err != nil {
		return fmt.Errorf("panel %s: %w", spec.ID, err)
	}
	panelResult := result.Panel(spec.ID)
	if panelResult == nil {
		return fmt.Errorf("panel %s is missing from runtime result", spec.ID)
	}
	if panelResult.Error != nil {
		return fmt.Errorf("panel %s runtime result: %w", spec.ID, panelResult.Error)
	}
	primary := panelResult.Frames.Primary()
	if primary == nil {
		return fmt.Errorf("panel %s has no primary frame", spec.ID)
	}
	frameRef := FrameRef("panel:" + spec.ID)
	wireFrame, err := buildPanelFrame(spec, primary)
	if err != nil {
		return fmt.Errorf("panel %s: %w", spec.ID, err)
	}
	doc.Frames[frameRef] = wireFrame
	actions := panelActions(spec)
	columns := buildTableColumns(spec)
	semantics := inferSemantics(kind)
	// Evidence is a claim about the panel's rows: each one is a source record
	// with a leaf link. An aggregate table whose interactions live outside the
	// wire contract (e.g. renderer-local HTMX actions) is series-shaped data
	// in a tabular encoding, and must not be forced into the evidence
	// invariant that Validate enforces.
	if semantics == SemanticsEvidence && !hasLeafAction(actions) && !hasLeafTableColumnAction(columns) {
		semantics = SemanticsSeries
	}
	var metricFlow *MetricFlowConfig
	var metricHierarchy *MetricHierarchyConfig
	var metricRelationship *MetricRelationshipConfig
	var radial *RadialConfig
	//nolint:exhaustive // Only the three metric kinds carry a metric config.
	switch kind {
	case PanelKindMetricFlow:
		metricFlow = buildMetricFlow(spec)
	case PanelKindMetricHierarchy:
		metricHierarchy = buildMetricHierarchy(spec)
	case PanelKindMetricRelationship:
		metricRelationship = buildMetricRelationship(spec)
		semantics = metricRelationshipSemantics(metricRelationship)
	case PanelKindRadial:
		radial = buildRadial(spec)
		if radial != nil && radial.Mode == RadialModePartition {
			semantics = SemanticsPartition
		}
	}
	var drillRoot *NodeKey
	if explorerSpec, ok := hosts[spec.ID]; ok {
		semantics = defaultExplorerSemantics(explorerSpec, semantics)
		key := explorerRootKey(explorerSpec.ID)
		drillRoot = &key
	} else if spec.DrillTree != nil {
		key, err := buildStaticDrillTree(doc, spec, frameRef)
		if err != nil {
			return fmt.Errorf("panel %s drill tree: %w", spec.ID, err)
		}
		drillRoot = &key
	}
	doc.Panels = append(doc.Panels, Panel{
		ID: spec.ID, Kind: kind, Title: spec.Title, Semantics: semantics, Frame: frameRef,
		Encoding: buildEncoding(spec.Fields, wireFrame), Format: buildFormats(spec), Total: spec.TotalBadgeValue, Columns: columns,
		DrillRoot: drillRoot, Actions: actions,
		Accent: panelAccent(spec), Status: buildStatus(spec), Caption: strings.TrimSpace(spec.Description),
		Headline: spec.HeadlineValue, Trend: buildTrend(spec), Presentation: buildPresentation(spec),
		Sparkline: buildSparkline(spec), Target: buildTarget(spec),
		MetricFlow: metricFlow, MetricHierarchy: metricHierarchy, MetricRelationship: metricRelationship,
		Radial:     radial,
		Confidence: Confidence(spec.Confidence), Availability: Availability(spec.Availability),
	})
	span := spec.Span
	if span == 0 {
		span = 6
	}
	// A single-level group keeps the legacy singular Group only, so an
	// existing document's output stays byte-identical: Groups is emitted only
	// once a chain actually nests (length >= 2). The innermost entry always
	// mirrors into Group for old readers, at any chain depth.
	var legacyGroup *LayoutGroup
	var nestedGroups []LayoutGroup
	if len(groups) > 0 {
		innermost := groups[len(groups)-1]
		legacyGroup = &innermost
		if len(groups) >= 2 {
			nestedGroups = append([]LayoutGroup(nil), groups...)
		}
	}
	row.Panels = append(row.Panels, LayoutItem{PanelID: spec.ID, Span: span, Group: legacyGroup, Groups: nestedGroups})
	labels := seriesLabels(spec, wireFrame)
	for index, color := range spec.Colors {
		if strings.TrimSpace(color) == "" {
			continue
		}
		doc.Theme.Series[fmt.Sprintf("%s:%d", spec.ID, index)] = color
		// Chart renderers that resolve a slice/series color from its own name
		// (a partition's category) cannot see the panel-scoped index key, so
		// publish the label alias too. The first panel to claim a label wins:
		// a later panel reusing the same category keeps the color already
		// established for it instead of silently recoloring both.
		if index < len(labels) {
			if label := strings.TrimSpace(labels[index]); label != "" {
				if _, taken := doc.Theme.Series[label]; !taken {
					doc.Theme.Series[label] = color
				}
			}
		}
	}
	return nil
}

// buildStaticDrillTree translates the renderer-neutral, precomputed
// panel.DrillTree into the wire drill graph used by the React Lens runtime.
// The legacy renderer consumes DrillTree directly; without this bridge the
// same valid panel silently lost its in-place navigation in React documents.
func buildStaticDrillTree(doc *DashboardDocument, spec panel.Spec, rootFrame FrameRef) (NodeKey, error) {
	rootKey := qualifiedKey("panel-drill", spec.ID)
	rootPath := NodePath{rootKey}
	root := Level{
		Path:         rootPath,
		Label:        "",
		Children:     make([]Node, 0, len(spec.DrillTree.Branches)),
		Frame:        rootFrame,
		Perspectives: make([]PerspectiveRef, 0),
		Presentation: buildPresentation(spec),
	}
	for _, branch := range spec.DrillTree.Branches {
		target := qualifiedKey("panel-drill", spec.ID, branch.TriggerKey)
		childPath := appendPath(rootPath, NodeKey(branch.TriggerKey))
		root.Children = append(root.Children, Node{
			Key:    NodeKey(branch.TriggerKey),
			Path:   childPath,
			Label:  branch.Label,
			Target: target,
		})
		if err := buildStaticDrillLevel(
			doc,
			spec,
			target,
			appendPath(rootPath, target),
			branch.Label,
			branch.Children,
			[]string{branch.TriggerKey},
		); err != nil {
			return "", err
		}
	}
	doc.Drill.Edges[rootKey] = root
	return rootKey, nil
}

func buildStaticDrillLevel(
	doc *DashboardDocument,
	spec panel.Spec,
	levelKey NodeKey,
	levelPath NodePath,
	label string,
	nodes []panel.DrillNode,
	identityPath []string,
) error {
	idField := spec.Fields.ID.Name()
	labelField := spec.Fields.Label.Name()
	valueField := spec.Fields.Value.Name()
	if idField == "" || labelField == "" || valueField == "" {
		return fmt.Errorf("requires id, label, and value fields")
	}

	frameRef := FrameRef(qualifiedKey(append([]string{"panel-drill-frame", spec.ID}, identityPath...)...))
	rows := make([][]any, 0, len(nodes))
	children := make([]Node, 0, len(nodes))
	for _, item := range nodes {
		rows = append(rows, []any{item.Key, item.Label, item.Value})
		childPath := appendPath(levelPath, NodeKey(item.Key))
		child := Node{Key: NodeKey(item.Key), Path: childPath, Label: item.Label}
		if len(item.Children) > 0 {
			targetParts := append(append([]string{"panel-drill", spec.ID}, identityPath...), item.Key)
			target := qualifiedKey(targetParts...)
			child.Target = target
			if err := buildStaticDrillLevel(
				doc,
				spec,
				target,
				appendPath(levelPath, target),
				item.Label,
				item.Children,
				append(append([]string(nil), identityPath...), item.Key),
			); err != nil {
				return err
			}
		} else if item.Action != nil {
			if converted, ok := convertAction(*item.Action, true); ok {
				child.Action = &converted
			}
		}
		children = append(children, child)
	}

	doc.Frames[frameRef] = Frame{
		Columns: []Column{
			{Name: idField, Type: ColumnString},
			{Name: labelField, Type: ColumnString},
			{Name: valueField, Type: ColumnNumber},
		},
		Rows: rows,
	}
	encoding := Encoding{ID: idField, Label: labelField, Value: valueField}
	doc.Drill.Edges[levelKey] = Level{
		Path:         levelPath,
		Label:        label,
		Children:     children,
		Frame:        frameRef,
		Encoding:     &encoding,
		Perspectives: make([]PerspectiveRef, 0),
	}
	return nil
}

// seriesLabels returns the panel's per-row label values in plot order, so a
// color list positioned by index can also be published under the label each
// color belongs to.
func seriesLabels(spec panel.Spec, wireFrame Frame) []string {
	field := spec.Fields.Label
	if field.Empty() {
		field = spec.Fields.Category
	}
	if field.Empty() {
		return nil
	}
	column := -1
	for index, item := range wireFrame.Columns {
		if item.Name == field.Name() {
			column = index
			break
		}
	}
	if column < 0 {
		return nil
	}
	labels := make([]string, 0, len(wireFrame.Rows))
	for _, row := range wireFrame.Rows {
		if column >= len(row) {
			labels = append(labels, "")
			continue
		}
		if value, ok := row[column].(string); ok {
			labels = append(labels, value)
			continue
		}
		labels = append(labels, "")
	}
	return labels
}

// appendGroup returns a fresh chain with descriptor appended after chain,
// always allocating a new backing array so sibling subtrees that received the
// same parent chain never alias each other's slice.
func appendGroup(chain []LayoutGroup, descriptor LayoutGroup) []LayoutGroup {
	next := make([]LayoutGroup, 0, len(chain)+1)
	next = append(next, chain...)
	return append(next, descriptor)
}

func containerSpan(spec panel.Spec) int {
	if spec.Span >= 1 && spec.Span <= 12 {
		return spec.Span
	}
	return 12
}

func groupLayout(layout panel.GroupLayout) LayoutGroupLayout {
	if layout == panel.GroupRows {
		return LayoutGroupRows
	}
	return LayoutGroupColumns
}

func panelAccent(spec panel.Spec) string {
	if accent := strings.TrimSpace(spec.Chrome.AccentColor); accent != "" {
		return accent
	}
	if len(spec.Colors) > 0 {
		return strings.TrimSpace(spec.Colors[0])
	}
	return ""
}

func buildStatus(spec panel.Spec) *PanelStatus { return convertStatus(spec.Status) }

// convertStatus maps a Go-side status chip onto the wire, dropping a labelless
// declaration. Levels and panels share it.
func convertStatus(source *panel.StatusSpec) *PanelStatus {
	if source == nil || strings.TrimSpace(source.Label) == "" {
		return nil
	}
	status := PanelStatus{Label: source.Label}
	switch source.Tone {
	case panel.StatusPositive:
		status.Tone = StatusTonePositive
	case panel.StatusWarning:
		status.Tone = StatusToneWarning
	case panel.StatusNeutral:
		status.Tone = StatusToneNeutral
	}
	return &status
}

func buildTrend(spec panel.Spec) *PanelTrend {
	if spec.Trend == nil {
		return nil
	}
	return &PanelTrend{Percent: spec.Trend.Percent, Label: spec.Trend.Label, Invert: spec.Trend.Invert}
}

func buildSparkline(spec panel.Spec) *Sparkline {
	if spec.Sparkline == nil || len(spec.Sparkline.Values) == 0 {
		return nil
	}
	return &Sparkline{Values: append([]float64(nil), spec.Sparkline.Values...), Color: spec.Sparkline.Color}
}

func buildTarget(spec panel.Spec) *PanelTarget {
	if spec.Target == nil {
		return nil
	}
	return &PanelTarget{Value: spec.Target.Value, Label: spec.Target.Label}
}

func buildPresentation(spec panel.Spec) *Presentation {
	hints := spec.Presentation
	if spec.Distributed {
		hints.ColorByCategory = true
	}
	return convertPresentation(hints)
}

// convertPresentation maps Go-side presentation hints onto the wire, returning
// nil when every hint is unset. Levels and panels share it.
func convertPresentation(hints panel.PresentationHints) *Presentation {
	presentation := Presentation{Fill: hints.FillPlot, BarWidthPx: hints.BarWidthPx}
	if hints.LegendBelow {
		presentation.Legend = LegendBelow
	}
	if hints.SliceLabelsPercent {
		presentation.SliceLabels = SliceLabelsPercent
	}
	switch {
	case hints.HideTotalBadge:
		presentation.TotalBadge = TotalBadgeNone
	case hints.TotalBadgeInPlot:
		presentation.TotalBadge = TotalBadgePlot
	}
	if hints.ColorByCategory {
		presentation.ColorBy = ColorByCategory
	}
	if hints.Waterfall {
		presentation.BridgeLayout = BridgeLayoutWaterfall
	}
	if hints.NonSortable {
		presentation.Sortable = boolPtr(false)
	}
	if hints.NonExpandable {
		presentation.Expandable = boolPtr(false)
	}
	if hints.NonExportable {
		presentation.Exportable = boolPtr(false)
	}
	presentation.RowGroupField = hints.RowGroupField
	if hints.FocusCanvas {
		presentation.Focus = FocusModeCanvas
	}
	if presentation == (Presentation{}) {
		return nil
	}
	return &presentation
}

func boolPtr(v bool) *bool { return &v }

// buildMetricFlow maps a metric_flow panel's builder stages onto the wire
// config. Per-element actions go through the same convertAction(a, true) path
// as table columns: v1 only supports literal/variable params, and a htmx/other
// unsupported action kind is silently dropped, matching the leaf column
// contract.
func buildMetricFlow(spec panel.Spec) *MetricFlowConfig {
	stages := make([]MetricFlowStage, 0, len(spec.FlowStages))
	for _, stage := range spec.FlowStages {
		wireStage := MetricFlowStage{
			Key: stage.Key, Label: stage.Label, Role: MetricFlowStageRole(stage.Role),
			Caption: stage.Caption, Confidence: Confidence(stage.Confidence), Availability: Availability(stage.Availability),
		}
		if stage.Action != nil {
			if converted, ok := convertAction(*stage.Action, true); ok {
				wireStage.Action = &converted
			}
		}
		stages = append(stages, wireStage)
	}
	config := &MetricFlowConfig{Stages: stages}
	if spec.FlowReconcile != nil {
		config.Reconcile = &FlowReconciliation{Tolerance: spec.FlowReconcile.Tolerance}
	}
	return config
}

// buildMetricHierarchy maps a metric_hierarchy panel's builder rows onto the
// wire config, deriving each row's Depth from its Parent chain (root = 0).
// Parent stays the single authoritative source of the hierarchy shape; Depth
// is only a renderer convenience computed here.
func buildMetricHierarchy(spec panel.Spec) *MetricHierarchyConfig {
	byKey := make(map[string]panel.HierarchyRow, len(spec.HierarchyRows))
	for _, row := range spec.HierarchyRows {
		byKey[row.Key] = row
	}
	rows := make([]MetricHierarchyRow, 0, len(spec.HierarchyRows))
	for _, row := range spec.HierarchyRows {
		wireRow := MetricHierarchyRow{
			Key: row.Key, Label: row.Label, Description: row.Description, Parent: row.Parent,
			Depth:        hierarchyRowDepth(byKey, row.Key),
			Unallocated:  row.Unallocated,
			Selected:     row.Selected,
			Confidence:   Confidence(row.Confidence),
			Availability: Availability(row.Availability),
		}
		if row.Action != nil {
			if converted, ok := convertAction(*row.Action, true); ok {
				wireRow.Action = &converted
			}
		}
		rows = append(rows, wireRow)
	}
	config := &MetricHierarchyConfig{Rows: rows}
	if spec.HierarchyReconcile != nil {
		config.Reconcile = &HierarchyReconciliation{Tolerance: spec.HierarchyReconcile.Tolerance}
	}
	return config
}

// hierarchyRowDepth walks a row's Parent chain to derive its depth (0 for a
// root). A cycle stops the walk instead of looping forever; Validate rejects a
// cyclic hierarchy independently, so the derived depth here only needs to
// terminate, not to be meaningful for an already-invalid document.
func hierarchyRowDepth(byKey map[string]panel.HierarchyRow, key string) int {
	depth := 0
	seen := make(map[string]struct{}, len(byKey))
	current := key
	for {
		row, ok := byKey[current]
		if !ok || row.Parent == "" {
			return depth
		}
		if _, cycled := seen[current]; cycled {
			return depth
		}
		seen[current] = struct{}{}
		current = row.Parent
		depth++
	}
}

// buildMetricRelationship maps a metric_relationship panel's builder spec onto
// the wire config. An empty Direction defaults to bidirectional on the wire,
// matching the documented "never inferred from end order" contract.
func buildMetricRelationship(spec panel.Spec) *MetricRelationshipConfig {
	rel := spec.Relationship
	if rel == nil {
		return nil
	}
	config := &MetricRelationshipConfig{
		Source: convertRelationshipEnd(rel.Source), Target: convertRelationshipEnd(rel.Target),
		Type: MetricRelationshipType(rel.Type), Direction: MetricRelationshipDirection(rel.Direction), Note: rel.Note,
	}
	if config.Direction == "" {
		config.Direction = MetricRelationshipBidirectional
	}
	return config
}

func convertRelationshipEnd(end panel.RelationshipEnd) MetricRelationshipEnd {
	result := MetricRelationshipEnd{
		Key: end.Key, Label: end.Label, Confidence: Confidence(end.Confidence), Availability: Availability(end.Availability),
	}
	if end.Action != nil {
		if converted, ok := convertAction(*end.Action, true); ok {
			result.Action = &converted
		}
	}
	return result
}

func panelKind(kind panel.Kind) (PanelKind, error) {
	//nolint:exhaustive // Container/gauge kinds are not part of the wire contract; default rejects them.
	switch kind {
	case panel.KindStat:
		return PanelKindStat, nil
	case panel.KindPie:
		return PanelKindPie, nil
	case panel.KindDonut:
		return PanelKindDonut, nil
	case panel.KindRadial:
		return PanelKindRadial, nil
	case panel.KindBar, panel.KindStackedBar:
		return PanelKindBar, nil
	case panel.KindHorizontalBar:
		return PanelKindHBar, nil
	case panel.KindSegmentBar:
		// A segment bar is a part-of-whole statement about one amount, which
		// the wire contract carries as its own composite kind rather than as
		// a bar chart.
		return PanelKindCoverage, nil
	case panel.KindTimeSeries:
		return PanelKindLine, nil
	case panel.KindCascade:
		return PanelKindCascade, nil
	case panel.KindTable:
		return PanelKindTable, nil
	case panel.KindMetricFlow:
		return PanelKindMetricFlow, nil
	case panel.KindMetricHierarchy:
		return PanelKindMetricHierarchy, nil
	case panel.KindMetricRelationship:
		return PanelKindMetricRelationship, nil
	default:
		return "", fmt.Errorf("unsupported document panel kind %q", kind)
	}
}

// inferSemantics returns a panel kind's default semantics. metric_relationship
// and metric_hierarchy need the panel's own config to settle their final
// semantics (relationship type; hierarchy reconciliation), so the leaf branch
// in appendPanelTree overrides this default once the panel spec is in scope.
//
// Deviation from the design doc: metric_hierarchy never resolves to
// SemanticsPartition, even when HierarchyReconcile is set. validatePartitionFrame
// (the shared partition-frame invariant used by pie/donut/coverage) rejects any
// negative row value, but a general keyed hierarchy frame commonly carries
// signed contributions (e.g. a contra-account child). Forcing partition
// semantics would incorrectly reject those valid hierarchies at the frame
// level, so metric_hierarchy stays series-shaped and the reconciliation
// summary line is a MetricHierarchy-config concern rendered independently of
// Semantics.
func inferSemantics(kind PanelKind) Semantics {
	//nolint:exhaustive // Remaining kinds are series-shaped by default.
	switch kind {
	case PanelKindPie, PanelKindDonut, PanelKindCoverage:
		return SemanticsPartition
	case PanelKindCascade, PanelKindMetricFlow:
		return SemanticsReconciliation
	case PanelKindTable:
		return SemanticsEvidence
	default:
		return SemanticsSeries
	}
}

func buildRadial(spec panel.Spec) *RadialConfig {
	if spec.Radial == nil {
		return nil
	}
	rings := make([]RadialRing, len(spec.Radial.Rings))
	for index, ring := range spec.Radial.Rings {
		rings[index] = RadialRing{Key: ring.Key, Label: ring.Label, Order: ring.Order, Total: ring.Total}
	}
	return &RadialConfig{
		Mode: RadialMode(spec.Radial.Mode), Max: spec.Radial.Max,
		Rings: rings, Tolerance: spec.Radial.Tolerance,
	}
}

// metricRelationshipSemantics resolves a metric_relationship panel's final
// semantics from its declared link type: reconciliation and derivation are
// reconciliation-shaped (one side explains the other), association is neutral
// series-shaped (no part-of/derivation claim).
func metricRelationshipSemantics(config *MetricRelationshipConfig) Semantics {
	if config == nil {
		return SemanticsSeries
	}
	switch config.Type {
	case MetricRelationshipReconciliation, MetricRelationshipDerivation:
		return SemanticsReconciliation
	case MetricRelationshipAssociation:
		return SemanticsSeries
	}
	return SemanticsSeries
}

// declaredEncoding maps a panel's declared fields into a wire encoding without
// consulting a frame. It backs lazy explore levels, whose frame does not exist
// at build time: the declaration is the contract the later query must satisfy.
func declaredEncoding(fields panel.FieldMapping) Encoding {
	encoding := Encoding{}
	assign := func(target *string, field panel.FieldRef) {
		if !field.Empty() {
			*target = field.Name()
		}
	}
	assign(&encoding.Label, fields.Label)
	assign(&encoding.Value, fields.Value)
	assign(&encoding.ID, fields.ID)
	assign(&encoding.Series, fields.Series)
	assign(&encoding.Category, fields.Category)
	assign(&encoding.Cut, fields.Cut)
	assign(&encoding.CutLabel, fields.CutLabel)
	assign(&encoding.Final, fields.Final)
	assign(&encoding.Annotation, fields.Annotation)
	assign(&encoding.Tone, fields.Tone)
	assign(&encoding.Share, fields.Share)
	assign(&encoding.Confidence, fields.Confidence)
	assign(&encoding.Availability, fields.Availability)
	return encoding
}

func buildEncoding(fields panel.FieldMapping, frame Frame) Encoding {
	encoding := Encoding{}
	available := make(map[string]struct{}, len(frame.Columns))
	for _, column := range frame.Columns {
		available[column.Name] = struct{}{}
	}
	assign := func(target *string, field panel.FieldRef) {
		if _, ok := available[field.Name()]; ok {
			*target = field.Name()
		}
	}
	assign(&encoding.Label, fields.Label)
	assign(&encoding.Value, fields.Value)
	assign(&encoding.ID, fields.ID)
	assign(&encoding.Series, fields.Series)
	assign(&encoding.Category, fields.Category)
	assign(&encoding.Cut, fields.Cut)
	assign(&encoding.CutLabel, fields.CutLabel)
	assign(&encoding.Final, fields.Final)
	assign(&encoding.Annotation, fields.Annotation)
	assign(&encoding.Tone, fields.Tone)
	assign(&encoding.Share, fields.Share)
	assign(&encoding.Confidence, fields.Confidence)
	assign(&encoding.Availability, fields.Availability)
	return encoding
}

func buildFormats(spec panel.Spec) map[string]FieldFormat {
	formats := make(map[string]FieldFormat)
	if !spec.Fields.Value.Empty() && spec.Formatter != nil {
		if converted, ok := convertFormat(*spec.Formatter); ok {
			formats[spec.Fields.Value.Name()] = converted
		}
	}
	for _, column := range spec.Columns {
		if column.Field.Empty() || column.Formatter == nil {
			continue
		}
		if converted, ok := convertFormat(*column.Formatter); ok {
			formats[column.Field.Name()] = converted
		}
	}
	for _, column := range spec.Columns {
		if column.Cell == nil || column.Cell.Kind != panel.TableCellDelta || column.Cell.PercentField.Empty() {
			continue
		}
		// Delta secondaries are percent changes by contract; default their wire
		// format so the runtime never renders a bare unlabeled number.
		if _, exists := formats[column.Cell.PercentField.Name()]; !exists {
			formats[column.Cell.PercentField.Name()] = FieldFormat{Kind: FormatPercent, Precision: PrecisionOf(1), DecimalSeparator: "."}
		}
	}
	return formats
}

func buildTableColumns(spec panel.Spec) []TableColumn {
	if spec.Kind != panel.KindTable {
		return nil
	}
	columns := make([]TableColumn, 0, len(spec.Columns))
	for _, column := range spec.Columns {
		wireColumn := TableColumn{
			Field: column.Field.Name(), Label: column.Label, Align: TableAlign(column.Align),
			Cell: TableCell{Kind: TableCellPlain}, Text: column.Text,
			WidthPx: column.WidthPx, Clamp: column.ClampLines,
			Affordance: TableAffordance(column.Affordance),
			BadgeField: column.BadgeField.Name(),
		}
		if column.Cell != nil {
			wireColumn.Cell.Kind = TableCellKind(column.Cell.Kind)
			if column.Cell.Kind == panel.TableCellDelta {
				wireColumn.Cell.SecondaryField = column.Cell.PercentField.Name()
			}
			if column.Cell.Stacked {
				wireColumn.Cell.Layout = TableCellStacked
			}
		}
		// Tone lives on the column (a plain cell has no TableCellSpec), so map it
		// onto the wire cell regardless of the rich-cell kind.
		wireColumn.Cell.ToneField = column.ToneField.Name()
		if column.Action != nil {
			if converted, ok := convertAction(*column.Action, true); ok {
				wireColumn.Action = &converted
			}
		}
		columns = append(columns, wireColumn)
	}
	return columns
}

func convertFormat(spec format.Spec) (FieldFormat, bool) {
	result := FieldFormat{Precision: PrecisionOf(spec.Precision), Layout: spec.Layout}
	//nolint:exhaustive // Formats without a wire representation are dropped via default.
	switch spec.Kind {
	case format.KindMoney:
		result.Kind = FormatMoney
		result.Currency = spec.Currency
		result.MinorUnits = false
		result.Symbol = currencySymbol(spec.Currency)
	case format.KindAbbreviatedMoney:
		if strings.TrimSpace(spec.Currency) == "" {
			result.Kind = FormatNumber
		} else {
			result.Kind = FormatMoney
			result.Currency = spec.Currency
			result.MinorUnits = false
		}
		result.Compact = true
		// format.abbreviate prints the mantissa with %.*f, i.e. a dot in every
		// locale. Pin the separator so both renderers agree byte for byte.
		result.DecimalSeparator = "."
	case format.KindPercent:
		result.Kind = FormatPercent
		// The Go renderer prints percents as %.*f%% — a dot and no space
		// before the sign in every locale. Pin the separator so a percent
		// cell reads identically on both renderers instead of drifting to
		// the locale's comma and non-breaking space.
		result.DecimalSeparator = "."
	case format.KindDate, format.KindMonthLabel:
		result.Kind = FormatDate
		if spec.Kind == format.KindMonthLabel && result.Layout == "" {
			result.Layout = "Jan 2006"
		}
	case format.KindInteger:
		result.Kind = FormatNumber
	default:
		return FieldFormat{}, false
	}
	return result, true
}

// currencySymbol resolves a currency code to the grapheme the Go money
// formatter prints (UZS → "so’m"), so both renderers show the same suffix.
// Unknown codes keep the code itself.
func currencySymbol(currency string) string {
	code := strings.TrimSpace(currency)
	if code == "" {
		return ""
	}
	definition := sdkmoney.GetCurrency(code)
	if definition == nil || strings.TrimSpace(definition.Grapheme) == "" {
		return ""
	}
	return definition.Grapheme
}

func buildFrame(source *frame.Frame) (Frame, error) {
	result := Frame{Columns: make([]Column, 0, len(source.Fields)), Rows: make([][]any, source.RowCount)}
	for _, field := range source.Fields {
		columnType, err := columnType(field.Type)
		if err != nil {
			return Frame{}, fmt.Errorf("field %s: %w", field.Name, err)
		}
		result.Columns = append(result.Columns, Column{Name: field.Name, Type: columnType})
	}
	for rowIndex := 0; rowIndex < source.RowCount; rowIndex++ {
		row := make([]any, len(source.Fields))
		for columnIndex := range source.Fields {
			row[columnIndex] = cloneAny(source.Fields[columnIndex].Values[rowIndex])
		}
		result.Rows[rowIndex] = row
	}
	return result, nil
}

type frameDependencies struct {
	sources []action.ValueSource
	actions []*action.Spec
}

func buildPanelFrame(spec panel.Spec, source *frame.Frame, extra ...frameDependencies) (Frame, error) {
	// Projection is driven by the declared columns. A table that declares
	// none has no projection to apply, and projecting anyway would emit an
	// empty frame and silently drop every row's data.
	if spec.Kind != panel.KindTable || len(spec.Columns) == 0 {
		return buildFrame(source)
	}

	selected := make([]string, 0, len(spec.Columns)+1)
	wanted := make(map[string]struct{})
	appendVisible := func(field panel.FieldRef) {
		name := field.Name()
		if strings.TrimSpace(name) == "" {
			return
		}
		if _, exists := wanted[name]; exists {
			return
		}
		wanted[name] = struct{}{}
		selected = append(selected, name)
	}
	addDependency := func(source action.ValueSource) {
		if source.Kind == action.SourceField && strings.TrimSpace(source.Name) != "" {
			wanted[source.Name] = struct{}{}
		}
	}
	addActionDependencies := func(spec *action.Spec) {
		if spec == nil {
			return
		}
		if _, ok := convertAction(*spec, true); !ok {
			return
		}
		if spec.URLSource != nil {
			addDependency(*spec.URLSource)
		}
		for _, param := range spec.Params {
			addDependency(param.Source)
		}
		for _, source := range spec.Payload {
			addDependency(source)
		}
	}

	for _, column := range spec.Columns {
		appendVisible(column.Field)
	}
	appendVisible(spec.Fields.ID)
	for _, column := range spec.Columns {
		if column.Cell != nil {
			addDependency(action.FieldValue(column.Cell.PercentField.Name()))
		}
		// Companion columns (tone, badge) are read per row by the renderer but
		// never rendered as their own column, so retain them explicitly or the
		// projection drops them.
		if !column.ToneField.Empty() {
			addDependency(action.FieldValue(column.ToneField.Name()))
		}
		if !column.BadgeField.Empty() {
			addDependency(action.FieldValue(column.BadgeField.Name()))
		}
		addActionDependencies(column.Action)
	}
	if spec.Presentation.RowGroupField != "" {
		addDependency(action.FieldValue(spec.Presentation.RowGroupField))
	}
	addActionDependencies(spec.Action)
	for _, dependencies := range extra {
		for _, source := range dependencies.sources {
			addDependency(source)
		}
		for _, actionSpec := range dependencies.actions {
			addActionDependencies(actionSpec)
		}
	}
	for _, field := range source.Fields {
		if _, ok := wanted[field.Name]; !ok || slices.Contains(selected, field.Name) {
			continue
		}
		selected = append(selected, field.Name)
	}

	result := Frame{Columns: make([]Column, 0, len(selected)), Rows: make([][]any, source.RowCount)}
	indexes := make([]int, 0, len(selected))
	for _, name := range selected {
		index := -1
		for candidateIndex, field := range source.Fields {
			if field.Name == name {
				index = candidateIndex
				break
			}
		}
		if index < 0 {
			return Frame{}, fmt.Errorf("projected table field %q is missing", name)
		}
		columnType, err := columnType(source.Fields[index].Type)
		if err != nil {
			return Frame{}, fmt.Errorf("field %s: %w", name, err)
		}
		indexes = append(indexes, index)
		result.Columns = append(result.Columns, Column{Name: name, Type: columnType})
	}
	for rowIndex := 0; rowIndex < source.RowCount; rowIndex++ {
		row := make([]any, len(indexes))
		for columnIndex, sourceIndex := range indexes {
			row[columnIndex] = cloneAny(source.Fields[sourceIndex].Values[rowIndex])
		}
		result.Rows[rowIndex] = row
	}
	return result, nil
}

func columnType(kind frame.FieldType) (ColumnType, error) {
	//nolint:exhaustive // FieldTypeUnknown is rejected via default.
	switch kind {
	case frame.FieldTypeString, frame.FieldTypeLocalized:
		return ColumnString, nil
	case frame.FieldTypeNumber:
		return ColumnNumber, nil
	case frame.FieldTypeBoolean:
		return ColumnBool, nil
	case frame.FieldTypeTime:
		return ColumnTime, nil
	default:
		return "", fmt.Errorf("unsupported frame field type %q", kind)
	}
}

func panelActions(spec panel.Spec) []Action {
	actions := make([]Action, 0)
	if spec.Action != nil {
		if converted, ok := convertAction(*spec.Action, spec.Kind == panel.KindTable); ok {
			actions = append(actions, converted)
		}
	}
	return actions
}

func convertAction(spec action.Spec, leaf bool) (Action, bool) {
	result := Action{
		Method: spec.Method, URLTemplate: spec.URL, Event: spec.Event, PreserveQuery: spec.PreserveQuery,
		Params: make([]ActionParam, 0, len(spec.Params)), Payload: make(map[string]Source),
	}
	if spec.URLSource != nil {
		converted := convertSource(*spec.URLSource)
		result.URLSource = &converted
	}
	//nolint:exhaustive // HTMX/cube-drill/explore actions are legacy renderer concerns, not wire actions.
	switch spec.Kind {
	case action.KindNavigate:
		if leaf {
			result.Kind = ActionNavigateToLeaf
		} else {
			result.Kind = ActionNavigate
		}
	case action.KindOpenDrawer:
		result.Kind = ActionOpenDrawer
	case action.KindEmitEvent:
		result.Kind = ActionEmitEvent
	default:
		return Action{}, false
	}
	for _, param := range spec.Params {
		result.Params = append(result.Params, ActionParam{Name: param.Name, Source: convertSource(param.Source)})
	}
	for name, source := range spec.Payload {
		result.Payload[name] = convertSource(source)
	}
	return result, true
}

func convertSource(source action.ValueSource) Source {
	result := Source{Name: source.Name, Value: cloneAny(source.Value), Fallback: cloneAny(source.Fallback)}
	switch source.Kind {
	case action.SourceField:
		result.Kind = ValueSourceField
	case action.SourceVariable:
		result.Kind = ValueSourceVariable
	case action.SourceLiteral:
		result.Kind = ValueSourceLiteral
	}
	return result
}

func explorerHosts(specs []explore.Spec) map[string]explore.Spec {
	result := make(map[string]explore.Spec, len(specs))
	for _, spec := range specs {
		result[spec.HostPanelID] = spec
	}
	return result
}

func defaultExplorerSemantics(spec explore.Spec, fallback Semantics) Semantics {
	if len(spec.Branches) == 0 {
		return fallback
	}
	branch := spec.Branches[0]
	perspective, ok := branch.Perspective(branch.DefaultPerspective)
	if !ok {
		return fallback
	}
	return Semantics(perspective.Semantics)
}

func buildExplorer(doc *DashboardDocument, spec explore.Spec, result *runtime.Result) error {
	rootKey := explorerRootKey(spec.ID)
	rootPath := NodePath{rootKey}
	root := Level{Path: rootPath, Label: "", Children: make([]Node, 0, len(spec.Branches)), Perspectives: make([]PerspectiveRef, 0)}
	if host := findDocumentPanel(doc.Panels, spec.HostPanelID); host != nil {
		root.Frame = host.Frame
		// The host panel's focus mode is a property of the whole exploration:
		// the React runtime reads it from the drill ROOT level (isFocusCanvas),
		// so a panel-level `.FocusCanvas()` must reach the root or the chrome
		// never activates.
		if host.Presentation != nil && host.Presentation.Focus != "" {
			root.Presentation = &Presentation{Focus: host.Presentation.Focus}
		}
	}
	for _, branch := range spec.Branches {
		branchKey := qualifiedKey(spec.ID, branch.Key)
		branchPath := appendPath(rootPath, branchKey)
		branchLevel := Level{
			Path: branchPath, Label: branch.Label, Children: make([]Node, 0),
			Perspectives:       make([]PerspectiveRef, 0, len(branch.Perspectives)),
			DefaultPerspective: string(qualifiedKey(spec.ID, branch.Key, branch.DefaultPerspective)),
		}
		root.Children = append(root.Children, Node{Key: branchKey, Path: branchPath, Label: branch.Label, Target: branchKey})
		for _, view := range branch.Perspectives {
			perspectiveID := string(qualifiedKey(spec.ID, branch.Key, view.Key))
			rootNode := qualifiedKey(spec.ID, branch.Key, view.Key, view.RootNode)
			depths := explorationDepths(view)
			doc.Perspectives = append(doc.Perspectives, Perspective{
				ID: perspectiveID, ExplorerID: spec.ID, BranchKey: branchKey, Key: view.Key,
				Label: view.Label, Semantics: Semantics(view.Semantics), Root: rootNode,
			})
			branchLevel.Perspectives = append(branchLevel.Perspectives, PerspectiveRef{ID: perspectiveID})
			for _, nodeSpec := range view.Nodes {
				nodeKey := qualifiedKey(spec.ID, branch.Key, view.Key, nodeSpec.Key)
				nodePath := appendPath(branchPath, qualifiedKey(spec.ID, branch.Key, view.Key), nodeKey)
				level := Level{Path: nodePath, Label: nodeSpec.Label, Children: make([]Node, 0, len(nodeSpec.Edges)), Perspectives: []PerspectiveRef{{ID: perspectiveID}}}
				if nodeSpec.View != "" {
					viewKind, err := panelKind(nodeSpec.View)
					if err != nil {
						return fmt.Errorf("explorer %s node %s view: %w", spec.ID, nodeSpec.Key, err)
					}
					level.View = viewKind
				}
				if nodeSpec.Presentation != nil {
					level.Presentation = convertPresentation(*nodeSpec.Presentation)
				}
				level.Status = convertStatus(nodeSpec.Status)
				if nodeSpec.DynamicChildren != nil {
					level.DynamicChildren = &DynamicChildren{
						Key:   convertSource(nodeSpec.DynamicChildren.Key),
						Label: convertSource(nodeSpec.DynamicChildren.Label),
					}
					if nodeSpec.DynamicChildren.Target != nil {
						target := convertSource(*nodeSpec.DynamicChildren.Target)
						level.DynamicChildren.Target = &target
					}
					if nodeSpec.DynamicChildren.Action != nil {
						if converted, ok := convertAction(*nodeSpec.DynamicChildren.Action, true); ok {
							level.DynamicChildren.Action = &converted
						}
					}
				}
				if nodeSpec.Panel != nil {
					// Every level with a panel declares its encoding, inlined or
					// not. A lazy level's frame arrives later via a query, and
					// the client renders it with `level.encoding ?? panel.encoding`
					// — without this, the host panel's field names leak in and
					// match nothing in the fetched frame, drawing an empty level.
					encoding := declaredEncoding(nodeSpec.Panel.Fields)
					level.Encoding = &encoding
				}
				if nodeSpec.Panel != nil && depths[nodeSpec.Key] <= doc.Drill.InlineDepth {
					if panelResult := result.Panel(nodeSpec.Panel.ID); panelResult != nil && panelResult.Error == nil && panelResult.Frames.Primary() != nil {
						frameRef := FrameRef("explore:" + perspectiveID + ":" + nodeSpec.Key)
						var wireFrame Frame
						var err error
						if nodeSpec.DynamicChildren != nil {
							dependencies := frameDependencies{sources: []action.ValueSource{nodeSpec.DynamicChildren.Key, nodeSpec.DynamicChildren.Label}}
							if nodeSpec.DynamicChildren.Target != nil {
								dependencies.sources = append(dependencies.sources, *nodeSpec.DynamicChildren.Target)
							}
							if nodeSpec.DynamicChildren.Action != nil {
								dependencies.actions = append(dependencies.actions, nodeSpec.DynamicChildren.Action)
							}
							wireFrame, err = buildPanelFrame(*nodeSpec.Panel, panelResult.Frames.Primary(), dependencies)
						} else {
							wireFrame, err = buildPanelFrame(*nodeSpec.Panel, panelResult.Frames.Primary())
						}
						if err != nil {
							return fmt.Errorf("explorer %s node %s: %w", spec.ID, nodeSpec.Key, err)
						}
						doc.Frames[frameRef] = wireFrame
						level.Frame = frameRef
						encoding := buildEncoding(nodeSpec.Panel.Fields, wireFrame)
						level.Encoding = &encoding
						if level.DynamicChildren != nil {
							if err := ResolveDynamicChildren(&wireFrame, level); err != nil {
								return fmt.Errorf("explorer %s node %s: %w", spec.ID, nodeSpec.Key, err)
							}
						}
					}
				}
				// A declared audit table ships whenever the runtime result carries
				// its panel's execution, mirroring the inline-frame guard above: a
				// missing result drops the declaration instead of failing the build.
				if nodeSpec.SourceData != nil {
					if panelResult := result.Panel(nodeSpec.SourceData.Panel.ID); panelResult != nil && panelResult.Error == nil && panelResult.Frames.Primary() != nil {
						sourceFrame, err := buildPanelFrame(nodeSpec.SourceData.Panel, panelResult.Frames.Primary())
						if err != nil {
							return fmt.Errorf("explorer %s node %s source data: %w", spec.ID, nodeSpec.Key, err)
						}
						sourceRef := FrameRef("source:" + perspectiveID + ":" + nodeSpec.Key)
						doc.Frames[sourceRef] = sourceFrame
						level.Source = &LevelSource{
							Label:   nodeSpec.SourceData.Label,
							Frame:   sourceRef,
							Columns: buildTableColumns(nodeSpec.SourceData.Panel),
							Format:  buildFormats(nodeSpec.SourceData.Panel),
						}
					}
				}
				for _, edge := range nodeSpec.Edges {
					pointKey := qualifiedKey(spec.ID, branch.Key, view.Key, nodeSpec.Key, edge.PointKey)
					childPath := appendPath(nodePath, pointKey)
					child := Node{Key: pointKey, Path: childPath, Label: ""}
					if edge.ToNode != "" {
						child.Target = qualifiedKey(spec.ID, branch.Key, view.Key, edge.ToNode)
					}
					if edge.Action != nil {
						if converted, ok := convertAction(*edge.Action, true); ok {
							child.Action = &converted
						}
					}
					level.Children = append(level.Children, child)
				}
				doc.Drill.Edges[nodeKey] = level
			}
		}
		doc.Drill.Edges[branchKey] = branchLevel
	}
	doc.Drill.Edges[rootKey] = root
	return nil
}

func findDocumentPanel(panels []Panel, id string) *Panel {
	for index := range panels {
		if panels[index].ID == id {
			return &panels[index]
		}
	}
	return nil
}

func explorationDepths(view explore.Perspective) map[string]int {
	const unseen = int(^uint(0) >> 1)
	depths := make(map[string]int, len(view.Nodes))
	for _, node := range view.Nodes {
		depths[node.Key] = unseen
	}
	depths[view.RootNode] = 0
	changed := true
	for changed {
		changed = false
		for _, node := range view.Nodes {
			depth := depths[node.Key]
			if depth == unseen {
				continue
			}
			for _, edge := range node.Edges {
				if edge.ToNode != "" && depth+1 < depths[edge.ToNode] {
					depths[edge.ToNode] = depth + 1
					changed = true
				}
			}
			for _, target := range node.DynamicTargets {
				if depth+1 < depths[target] {
					depths[target] = depth + 1
					changed = true
				}
			}
		}
	}
	return depths
}

func explorerRootKey(explorerID string) NodeKey { return qualifiedKey(explorerID) }

func qualifiedKey(parts ...string) NodeKey {
	escaped := make([]string, len(parts))
	for index, part := range parts {
		escaped[index] = url.PathEscape(part)
	}
	return NodeKey(strings.Join(escaped, "/"))
}

func appendPath(path NodePath, keys ...NodeKey) NodePath {
	result := append(NodePath(nil), path...)
	return append(result, keys...)
}

func newSnapshotID() (string, error) {
	var raw [18]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate snapshot id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}
