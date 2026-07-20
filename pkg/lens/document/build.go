package document

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/explore"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
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
		Layout:       Layout{Rows: make([]LayoutRow, 0, len(spec.Rows))},
		Panels:       make([]Panel, 0),
		Frames:       make(map[FrameRef]Frame),
		Drill:        Drill{Edges: make(map[NodeKey]Level), InlineDepth: opts.InlineDepth},
		Perspectives: make([]Perspective, 0),
		Endpoints:    opts.Endpoints,
		I18n:         cloneStrings(opts.I18n),
		Theme:        cloneTheme(opts.Theme),
	}
	if doc.I18n == nil {
		doc.I18n = make(map[string]string)
	}

	hosts := explorerHosts(spec.Explorers)
	for _, rowSpec := range spec.Rows {
		layoutRow := LayoutRow{Heading: rowSpec.Heading, Class: rowSpec.Class, Panels: make([]LayoutItem, 0)}
		for _, panelSpec := range rowSpec.Panels {
			if err := appendPanelTree(doc, panelSpec, result, hosts, &layoutRow); err != nil {
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

func appendPanelTree(doc *DashboardDocument, spec panel.Spec, result *runtime.Result, hosts map[string]explore.Spec, row *LayoutRow) error {
	if spec.Kind.IsContainer() {
		for _, child := range spec.Children {
			if err := appendPanelTree(doc, child, result, hosts, row); err != nil {
				return err
			}
		}
		return nil
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
	wireFrame, err := buildFrame(primary)
	if err != nil {
		return fmt.Errorf("panel %s: %w", spec.ID, err)
	}
	doc.Frames[frameRef] = wireFrame
	actions := panelActions(spec)
	semantics := inferSemantics(kind)
	// Evidence is a claim about the panel's rows: each one is a source record
	// with a leaf link. An aggregate table whose interactions live outside the
	// wire contract (e.g. renderer-local HTMX actions) is series-shaped data
	// in a tabular encoding, and must not be forced into the evidence
	// invariant that Validate enforces.
	if semantics == SemanticsEvidence && !hasLeafAction(actions) {
		semantics = SemanticsSeries
	}
	var drillRoot *NodeKey
	if explorerSpec, ok := hosts[spec.ID]; ok {
		semantics = defaultExplorerSemantics(explorerSpec, semantics)
		key := explorerRootKey(explorerSpec.ID)
		drillRoot = &key
	}
	doc.Panels = append(doc.Panels, Panel{
		ID: spec.ID, Kind: kind, Title: spec.Title, Semantics: semantics, Frame: frameRef,
		Encoding: buildEncoding(spec.Fields, wireFrame), Format: buildFormats(spec), DrillRoot: drillRoot, Actions: actions,
	})
	span := spec.Span
	if span == 0 {
		span = 6
	}
	row.Panels = append(row.Panels, LayoutItem{PanelID: spec.ID, Span: span})
	for index, color := range spec.Colors {
		if strings.TrimSpace(color) != "" {
			doc.Theme.Series[fmt.Sprintf("%s:%d", spec.ID, index)] = color
		}
	}
	return nil
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
	case panel.KindBar, panel.KindStackedBar:
		return PanelKindBar, nil
	case panel.KindHorizontalBar, panel.KindSegmentBar:
		return PanelKindHBar, nil
	case panel.KindTimeSeries:
		return PanelKindLine, nil
	case panel.KindCascade:
		return PanelKindCascade, nil
	case panel.KindTable:
		return PanelKindTable, nil
	default:
		return "", fmt.Errorf("unsupported document panel kind %q", kind)
	}
}

func inferSemantics(kind PanelKind) Semantics {
	//nolint:exhaustive // Remaining kinds are series-shaped by default.
	switch kind {
	case PanelKindPie, PanelKindDonut:
		return SemanticsPartition
	case PanelKindCascade:
		return SemanticsReconciliation
	case PanelKindTable:
		return SemanticsEvidence
	default:
		return SemanticsSeries
	}
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
	return formats
}

func convertFormat(spec format.Spec) (FieldFormat, bool) {
	result := FieldFormat{Precision: spec.Precision, Layout: spec.Layout}
	//nolint:exhaustive // Formats without a wire representation are dropped via default.
	switch spec.Kind {
	case format.KindMoney:
		result.Kind = FormatMoney
		result.Currency = spec.Currency
		result.MinorUnits = false
	case format.KindAbbreviatedMoney:
		if strings.TrimSpace(spec.Currency) == "" {
			result.Kind = FormatNumber
		} else {
			result.Kind = FormatMoney
			result.Currency = spec.Currency
			result.MinorUnits = false
		}
	case format.KindPercent:
		result.Kind = FormatPercent
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
	for _, column := range spec.Columns {
		if column.Action == nil {
			continue
		}
		if converted, ok := convertAction(*column.Action, true); ok {
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
	//nolint:exhaustive // HTMX/cube-drill/explore actions are legacy renderer concerns, not wire actions.
	switch spec.Kind {
	case action.KindNavigate:
		if leaf {
			result.Kind = ActionNavigateToLeaf
		} else {
			result.Kind = ActionNavigate
		}
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
	}
	for _, branch := range spec.Branches {
		branchKey := qualifiedKey(spec.ID, branch.Key)
		branchPath := appendPath(rootPath, branchKey)
		branchLevel := Level{Path: branchPath, Label: branch.Label, Children: make([]Node, 0), Perspectives: make([]PerspectiveRef, 0, len(branch.Perspectives))}
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
				if nodeSpec.Panel != nil && depths[nodeSpec.Key] <= doc.Drill.InlineDepth {
					if panelResult := result.Panel(nodeSpec.Panel.ID); panelResult != nil && panelResult.Error == nil && panelResult.Frames.Primary() != nil {
						frameRef := FrameRef("explore:" + perspectiveID + ":" + nodeSpec.Key)
						wireFrame, err := buildFrame(panelResult.Frames.Primary())
						if err != nil {
							return fmt.Errorf("explorer %s node %s: %w", spec.ID, nodeSpec.Key, err)
						}
						doc.Frames[frameRef] = wireFrame
						level.Frame = frameRef
						encoding := buildEncoding(nodeSpec.Panel.Fields, wireFrame)
						level.Encoding = &encoding
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
