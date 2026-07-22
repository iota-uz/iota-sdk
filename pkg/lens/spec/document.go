// Package spec defines the JSON-serializable Lens document model and helpers.
package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/chrome"
	"github.com/iota-uz/iota-sdk/pkg/lens/cube"
	"github.com/iota-uz/iota-sdk/pkg/lens/exportmeta"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const DocumentVersion = 2

type BodyPosition string

const (
	BodyPositionAppend  BodyPosition = "append"
	BodyPositionPrepend BodyPosition = "prepend"
)

type Document struct {
	Version          int                        `json:"version"`
	ID               string                     `json:"id"`
	Title            Text                       `json:"title"`
	Description      Text                       `json:"description"`
	Variables        []VariableSpec             `json:"variables,omitempty"`
	BodyPosition     BodyPosition               `json:"bodyPosition,omitempty"`
	Datasets         []DatasetSpec              `json:"datasets,omitempty"`
	Rows             []RowSpec                  `json:"rows,omitempty"`
	Drill            *lens.DrillMeta            `json:"drill,omitempty"`
	DataMode         cube.DataMode              `json:"dataMode,omitempty"`
	DataSource       string                     `json:"dataSource,omitempty"`
	FromSQL          string                     `json:"fromSQL,omitempty"`
	DataRef          string                     `json:"dataRef,omitempty"`
	Params           map[string]lens.ParamValue `json:"params,omitempty"`
	Where            []string                   `json:"where,omitempty"`
	Joins            []cube.JoinSpec            `json:"joins,omitempty"`
	Dimensions       []DimensionSpec            `json:"dimensions,omitempty"`
	Measures         []MeasureSpec              `json:"measures,omitempty"`
	DefaultDimension string                     `json:"defaultDimension,omitempty"`
	Leaf             *cube.LeafSpec             `json:"leaf,omitempty"`
	Cache            CachePolicy                `json:"cache,omitempty"`
	Export           exportmeta.Spec            `json:"export,omitempty"`
	Explorers        []ExplorerSpec             `json:"explorers,omitempty"`
}

type CachePolicy struct {
	Mode lens.CacheMode `json:"mode,omitempty"`
	TTL  Duration       `json:"ttl,omitempty"`
}

type VariableSpec struct {
	Name            string            `json:"name"`
	Label           Text              `json:"label"`
	Kind            lens.VariableKind `json:"kind"`
	Component       string            `json:"component,omitempty"`
	RequestKeys     []string          `json:"requestKeys,omitempty"`
	Default         any               `json:"default,omitempty"`
	Required        bool              `json:"required,omitempty"`
	Description     Text              `json:"description"`
	Options         []VariableOption  `json:"options,omitempty"`
	AllowAllTime    bool              `json:"allowAllTime,omitempty"`
	DefaultDuration Duration          `json:"defaultDuration,omitempty"`
}

type VariableOption struct {
	Label Text   `json:"label"`
	Value string `json:"value"`
}

type DimensionSpec struct {
	Name         string             `json:"name"`
	Label        Text               `json:"label"`
	Type         cube.DimensionType `json:"type,omitempty"`
	Column       string             `json:"column,omitempty"`
	LabelColumn  string             `json:"labelColumn,omitempty"`
	ColorColumn  string             `json:"colorColumn,omitempty"`
	Field        string             `json:"field,omitempty"`
	LabelField   string             `json:"labelField,omitempty"`
	ColorField   string             `json:"colorField,omitempty"`
	PanelKind    panel.Kind         `json:"panelKind,omitempty"`
	Height       string             `json:"height,omitempty"`
	Description  Text               `json:"description"`
	RequiresJoin []string           `json:"requiresJoin,omitempty"`
	Override     *DatasetSpec       `json:"override,omitempty"`
	Transforms   []transform.Spec   `json:"transforms,omitempty"`
	Colors       []string           `json:"colors,omitempty"`
	ValueAxis    panel.ValueAxis    `json:"valueAxis,omitempty"`
	ColorScale   string             `json:"colorScale,omitempty"`
}

type MeasureSpec struct {
	Name         string           `json:"name"`
	Label        Text             `json:"label"`
	Column       string           `json:"column,omitempty"`
	Field        string           `json:"field,omitempty"`
	Aggregation  cube.Aggregation `json:"aggregation"`
	Formatter    *format.Spec     `json:"formatter,omitempty"`
	AccentColor  string           `json:"accentColor,omitempty"`
	Description  Text             `json:"description"`
	Info         Text             `json:"info"`
	RequiresJoin []string         `json:"requiresJoin,omitempty"`
	Override     *DatasetSpec     `json:"override,omitempty"`
	Action       *action.Spec     `json:"action,omitempty"`
}

type DatasetSpec struct {
	Name        string           `json:"name"`
	Title       Text             `json:"title"`
	Kind        lens.DatasetKind `json:"kind"`
	Source      string           `json:"source,omitempty"`
	DependsOn   []string         `json:"dependsOn,omitempty"`
	Query       *lens.QuerySpec  `json:"query,omitempty"`
	Transforms  []transform.Spec `json:"transforms,omitempty"`
	StaticRef   string           `json:"staticRef,omitempty"`
	Static      *frame.FrameSet  `json:"-"`
	Description Text             `json:"description"`
	Cache       CachePolicy      `json:"cache,omitempty"`
	Export      exportmeta.Spec  `json:"export,omitempty"`
}

type RowSpec struct {
	Panels []PanelSpec `json:"panels"`
	Class  string      `json:"class,omitempty"`
	// Heading, when set, renders the row as a section header band instead of
	// a panel grid. Used to group panels into labeled sections.
	Heading Text `json:"heading,omitempty,omitzero"`
}

type PanelSpec struct {
	ID              string                `json:"id"`
	Title           Text                  `json:"title"`
	Description     Text                  `json:"description"`
	Info            Text                  `json:"info"`
	Kind            panel.Kind            `json:"kind"`
	Dataset         string                `json:"dataset,omitempty"`
	Span            int                   `json:"span,omitempty"`
	Height          string                `json:"height,omitempty"`
	Colors          []string              `json:"colors,omitempty"`
	ShowLegend      bool                  `json:"showLegend,omitempty"`
	LegendPosition  panel.LegendPosition  `json:"legendPosition,omitempty"`
	LegendWidthPx   int                   `json:"legendWidth,omitempty"`
	LegendOffsetY   int                   `json:"legendOffsetY,omitempty"`
	LegendFloating  bool                  `json:"legendFloating,omitempty"`
	CircularScale   float64               `json:"circularScale,omitempty"`
	CircularOffsetX int                   `json:"circularOffsetX,omitempty"`
	ShowTotalBadge  bool                  `json:"showTotalBadge,omitempty"`
	TotalBadgeValue *float64              `json:"totalBadgeValue,omitempty"`
	HeadlineValue   *float64              `json:"headlineValue,omitempty"`
	DrillHierarchy  *panel.DrillHierarchy `json:"drillHierarchy,omitempty"`
	DrillTree       *panel.DrillTree      `json:"drillTree,omitempty"`
	Trend           *panel.TrendSpec      `json:"trend,omitempty"`
	Status          *panel.StatusSpec     `json:"status,omitempty"`
	Sparkline       *panel.SparklineSpec  `json:"sparkline,omitempty"`
	GroupLayout     panel.GroupLayout     `json:"groupLayout,omitempty"`
	// Presentation carries opt-in renderer density hints (legend placement,
	// in-slice labels, total-badge placement, bar width, per-category color).
	// The zero value keeps today's rendering.
	Presentation panel.PresentationHints `json:"presentation,omitzero"`
	Fields       FieldMappingSpec        `json:"fields,omitempty"`
	Formatter    *format.Spec            `json:"formatter,omitempty"`
	Columns      []TableColumnSpec       `json:"columns,omitempty"`
	Transforms   []transform.Spec        `json:"transforms,omitempty"`
	Action       *action.Spec            `json:"action,omitempty"`
	Children     []PanelSpec             `json:"children,omitempty"`
	ClassName    string                  `json:"className,omitempty"`
	Chrome       chrome.Spec             `json:"-"`
	ChromeIcon   string                  `json:"icon,omitempty"`
	AccentColor  string                  `json:"accentColor,omitempty"`
	ValueAxis    panel.ValueAxis         `json:"valueAxis,omitempty"`
	Distributed  bool                    `json:"distributed,omitempty"`
	ColorField   string                  `json:"colorField,omitempty"`
	ColorScale   string                  `json:"colorScale,omitempty"`
	Export       exportmeta.Spec         `json:"export,omitempty"`
}

type TableColumnSpec struct {
	Field     string               `json:"field,omitempty"`
	Label     Text                 `json:"label"`
	Formatter *format.Spec         `json:"formatter,omitempty"`
	Action    *action.Spec         `json:"action,omitempty"`
	Text      Text                 `json:"text"`
	Align     string               `json:"align,omitempty"`
	Cell      *panel.TableCellSpec `json:"cell,omitempty"`
	// WidthPx, when > 0, sets a min-width (px) on the column's cells.
	WidthPx int `json:"width,omitempty"`
	// ClampLines, when > 0, limits the cell text to that many rendered lines.
	ClampLines int `json:"clamp,omitempty"`
	// Affordance selects how an actionable cell advertises its action; "pill"
	// renders a compact pill with a drill arrow.
	Affordance string `json:"affordance,omitempty"`
}

type FieldMappingSpec struct {
	Label     string `json:"label,omitempty"`
	Value     string `json:"value,omitempty"`
	Series    string `json:"series,omitempty"`
	Category  string `json:"category,omitempty"`
	ID        string `json:"id,omitempty"`
	StartTime string `json:"startTime,omitempty"`
	EndTime   string `json:"endTime,omitempty"`
	Cut       string `json:"cut,omitempty"`
	CutLabel  string `json:"cutLabel,omitempty"`
	Final     string `json:"final,omitempty"`
}

type ExplorerSpec struct {
	ID           string           `json:"id"`
	HostPanelID  string           `json:"hostPanelId"`
	ExpandedSpan int              `json:"expandedSpan,omitempty"`
	Branches     []ExplorerBranch `json:"branches"`
}

type ExplorerBranch struct {
	Key                string                `json:"key"`
	Label              Text                  `json:"label"`
	DefaultPerspective string                `json:"defaultPerspective"`
	Perspectives       []ExplorerPerspective `json:"perspectives"`
}

type ExplorerPerspective struct {
	Key       string          `json:"key"`
	Label     Text            `json:"label"`
	Semantics string          `json:"semantics"`
	RootNode  string          `json:"rootNode"`
	Nodes     []ExplorerNode  `json:"nodes"`
	Export    exportmeta.Spec `json:"export,omitempty"`
}

type ExplorerNode struct {
	Key             string                   `json:"key"`
	Label           Text                     `json:"label"`
	Panel           *PanelSpec               `json:"panel,omitempty"`
	Load            *ExplorerLoadSpec        `json:"load,omitempty"`
	Edges           []ExplorerEdge           `json:"edges,omitempty"`
	DynamicEdges    bool                     `json:"dynamicEdges,omitempty"`
	DynamicTargets  []string                 `json:"dynamicTargets,omitempty"`
	DynamicChildren *ExplorerDynamicChildren `json:"dynamicChildren,omitempty"`
	Check           *ExplorerBalanceCheck    `json:"check,omitempty"`
}

type ExplorerDynamicChildren struct {
	Key    action.ValueSource  `json:"key"`
	Label  action.ValueSource  `json:"label"`
	Target *action.ValueSource `json:"target,omitempty"`
	Action *action.Spec        `json:"action,omitempty"`
}

type ExplorerLoadSpec struct {
	URL           string `json:"url"`
	Method        string `json:"method,omitempty"`
	PreserveQuery bool   `json:"preserveQuery,omitempty"`
}

type ExplorerEdge struct {
	PointKey string       `json:"pointKey"`
	ToNode   string       `json:"toNode,omitempty"`
	Action   *action.Spec `json:"action,omitempty"`
}

type ExplorerBalanceCheck struct {
	Expected  float64 `json:"expected"`
	Actual    float64 `json:"actual"`
	Tolerance float64 `json:"tolerance,omitempty"`
}

func Load(data []byte) (Document, error) {
	const op serrors.Op = "lens.spec.Load"

	var doc Document
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	//nolint:musttag // Document is the canonical Lens JSON payload owned by this package.
	if err := decoder.Decode(&doc); err != nil {
		return Document{}, serrors.E(op, err)
	}
	if err := doc.Validate(); err != nil {
		return Document{}, serrors.E(op, err)
	}
	return doc, nil
}

func LoadFS(fsys fs.FS, name string) (Document, error) {
	const op serrors.Op = "lens.spec.LoadFS"

	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return Document{}, serrors.E(op, err)
	}
	doc, err := Load(data)
	if err != nil {
		return Document{}, serrors.E(op, err)
	}
	return doc, nil
}

func (d Document) EffectiveVersion() int {
	if d.Version == 0 {
		return DocumentVersion
	}
	return d.Version
}

func (d Document) Validate() error {
	if version := d.EffectiveVersion(); version != DocumentVersion {
		return fmt.Errorf("unsupported lens document version %d", version)
	}
	if strings.TrimSpace(d.ID) == "" {
		return fmt.Errorf("document id is required")
	}
	if strings.TrimSpace(d.Title.Resolve("")) == "" {
		return fmt.Errorf("document title is required")
	}
	switch d.BodyPosition {
	case "", BodyPositionAppend, BodyPositionPrepend:
	default:
		return fmt.Errorf("unsupported bodyPosition %q", d.BodyPosition)
	}
	return nil
}

func (d Document) HasSemantic() bool {
	return d.DataMode != "" ||
		strings.TrimSpace(d.DataSource) != "" ||
		strings.TrimSpace(d.FromSQL) != "" ||
		strings.TrimSpace(d.DataRef) != "" ||
		len(d.Dimensions) > 0 ||
		len(d.Measures) > 0
}
