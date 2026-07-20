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
		}
	}
	if err := d.validateDrill(); err != nil {
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
	for _, action := range panel.Actions {
		if err := validateAction(panel.ID, action); err != nil {
			return err
		}
		if err := validateActionFields(panel.ID, action, d.Frames[panel.Frame]); err != nil {
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

func validateTableColumns(panel Panel, frame Frame) error {
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
		switch column.Cell.Kind {
		case TableCellPlain, TableCellBar:
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
	case ActionNavigate, ActionNavigateToLeaf:
		if strings.TrimSpace(action.URLTemplate) == "" && action.URLSource == nil {
			return fmt.Errorf("%s navigate action requires url", owner)
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

func validateActionFields(owner string, action Action, frame Frame) error {
	validate := func(source Source) error {
		if source.Kind == ValueSourceField && !frameHasColumn(frame, source.Name) {
			return fmt.Errorf("%s action references missing field %q", owner, source.Name)
		}
		return nil
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
		if action.Kind == ActionNavigateToLeaf {
			return true
		}
	}
	return false
}

func hasLeafTableColumnAction(columns []TableColumn) bool {
	for _, column := range columns {
		if column.Action != nil && column.Action.Kind == ActionNavigateToLeaf {
			return true
		}
	}
	return false
}

func validPanelKind(kind PanelKind) bool {
	switch kind {
	case PanelKindStat, PanelKindPie, PanelKindDonut, PanelKindBar, PanelKindHBar,
		PanelKindLine, PanelKindArea, PanelKindCascade, PanelKindTable:
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
