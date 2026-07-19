package document

import (
	"encoding/json"
	"time"
)

type FrameRef string

type DashboardDocument struct {
	Version      string             `json:"version"`
	SnapshotID   string             `json:"snapshotId"`
	Meta         Meta               `json:"meta"`
	Layout       Layout             `json:"layout"`
	Panels       []Panel            `json:"panels"`
	Frames       map[FrameRef]Frame `json:"frames"`
	Drill        Drill              `json:"drill"`
	Perspectives []Perspective      `json:"perspectives"`
	Endpoints    Endpoints          `json:"endpoints"`
	I18n         map[string]string  `json:"i18n"`
	Theme        Theme              `json:"theme"`
}

func (d DashboardDocument) MarshalJSON() ([]byte, error) {
	type wireDocument DashboardDocument
	d.Version = ContractVersion
	return json.Marshal(wireDocument(d))
}

type Meta struct {
	DashboardID string    `json:"dashboardId"`
	Title       string    `json:"title"`
	GeneratedAt time.Time `json:"generatedAt"`
	Locale      string    `json:"locale"`
}

type Layout struct {
	Rows []LayoutRow `json:"rows"`
}

type LayoutRow struct {
	Heading string       `json:"heading,omitempty"`
	Class   string       `json:"class,omitempty"`
	Panels  []LayoutItem `json:"panels"`
}

type LayoutItem struct {
	PanelID string `json:"panelId"`
	Span    int    `json:"span"`
}

type PanelKind string

const (
	PanelKindStat    PanelKind = "stat"
	PanelKindPie     PanelKind = "pie"
	PanelKindDonut   PanelKind = "donut"
	PanelKindBar     PanelKind = "bar"
	PanelKindHBar    PanelKind = "hbar"
	PanelKindLine    PanelKind = "line"
	PanelKindArea    PanelKind = "area"
	PanelKindCascade PanelKind = "cascade"
	PanelKindTable   PanelKind = "table"
)

type Semantics string

const (
	SemanticsPartition      Semantics = "partition"
	SemanticsReconciliation Semantics = "reconciliation"
	SemanticsSeries         Semantics = "series"
	SemanticsEvidence       Semantics = "evidence"
)

type Panel struct {
	ID        string                 `json:"id"`
	Kind      PanelKind              `json:"kind"`
	Title     string                 `json:"title"`
	Semantics Semantics              `json:"semantics"`
	Frame     FrameRef               `json:"frame"`
	Encoding  Encoding               `json:"encoding"`
	Format    map[string]FieldFormat `json:"format"`
	DrillRoot *NodeKey               `json:"drillRoot,omitempty"`
	Actions   []Action               `json:"actions"`
}

type Encoding struct {
	Label    string `json:"label,omitempty"`
	Value    string `json:"value,omitempty"`
	ID       string `json:"id,omitempty"`
	Series   string `json:"series,omitempty"`
	Category string `json:"category,omitempty"`
	Cut      string `json:"cut,omitempty"`
	CutLabel string `json:"cutLabel,omitempty"`
	Final    string `json:"final,omitempty"`
}

type FormatKind string

const (
	FormatMoney   FormatKind = "money"
	FormatPercent FormatKind = "percent"
	FormatDate    FormatKind = "date"
	FormatNumber  FormatKind = "number"
	FormatString  FormatKind = "string"
)

type FieldFormat struct {
	Kind       FormatKind `json:"kind"`
	Currency   string     `json:"currency,omitempty"`
	MinorUnits bool       `json:"minorUnits"`
	Precision  int        `json:"precision,omitempty"`
	Layout     string     `json:"layout,omitempty"`
}

type ActionKind string

const (
	ActionNavigate       ActionKind = "navigate"
	ActionNavigateToLeaf ActionKind = "navigate_to_leaf"
	ActionEmitEvent      ActionKind = "emit_event"
)

type ValueSourceKind string

const (
	ValueSourceField    ValueSourceKind = "field"
	ValueSourceVariable ValueSourceKind = "variable"
	ValueSourceLiteral  ValueSourceKind = "literal"
)

type Action struct {
	Kind          ActionKind        `json:"kind"`
	Method        string            `json:"method,omitempty"`
	URLTemplate   string            `json:"urlTemplate,omitempty"`
	Event         string            `json:"event,omitempty"`
	Params        []ActionParam     `json:"params"`
	Payload       map[string]Source `json:"payload"`
	PreserveQuery bool              `json:"preserveQuery,omitempty"`
}

type ActionParam struct {
	Name   string `json:"name"`
	Source Source `json:"source"`
}

type Source struct {
	Kind     ValueSourceKind `json:"kind"`
	Name     string          `json:"name,omitempty"`
	Value    any             `json:"value,omitempty"`
	Fallback any             `json:"fallback,omitempty"`
}

type NodeKey string
type NodePath []NodeKey

type Drill struct {
	Edges       map[NodeKey]Level `json:"edges"`
	InlineDepth int               `json:"inlineDepth"`
}

type Level struct {
	Path         NodePath         `json:"path"`
	Label        string           `json:"label"`
	Children     []Node           `json:"children"`
	Frame        FrameRef         `json:"frame,omitempty"`
	Encoding     *Encoding        `json:"encoding,omitempty"`
	Perspectives []PerspectiveRef `json:"perspectives"`
}

type Node struct {
	Key    NodeKey  `json:"key"`
	Path   NodePath `json:"path"`
	Label  string   `json:"label"`
	Target NodeKey  `json:"target,omitempty"`
	Action *Action  `json:"action,omitempty"`
}

type PerspectiveRef struct {
	ID string `json:"id"`
}

type Perspective struct {
	ID         string    `json:"id"`
	ExplorerID string    `json:"explorerId"`
	BranchKey  NodeKey   `json:"branchKey"`
	Key        string    `json:"key"`
	Label      string    `json:"label"`
	Semantics  Semantics `json:"semantics"`
	Root       NodeKey   `json:"root"`
}

type ColumnType string

const (
	ColumnString ColumnType = "string"
	ColumnNumber ColumnType = "number"
	ColumnBool   ColumnType = "bool"
	ColumnTime   ColumnType = "time"
)

type Column struct {
	Name string     `json:"name"`
	Type ColumnType `json:"type"`
}

type Frame struct {
	Columns []Column `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

type Endpoints struct {
	Query  string `json:"query,omitempty"`
	Export string `json:"export,omitempty"`
}

type Theme struct {
	Palette map[string]string `json:"palette"`
	Series  map[string]string `json:"series"`
}
