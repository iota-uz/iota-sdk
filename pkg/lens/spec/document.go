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
}

type VariableSpec struct {
	Name            string            `json:"name"`
	Label           Text              `json:"label"`
	Kind            lens.VariableKind `json:"kind"`
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
}

type RowSpec struct {
	Panels []PanelSpec `json:"panels"`
	Class  string      `json:"class,omitempty"`
}

type PanelSpec struct {
	ID          string            `json:"id"`
	Title       Text              `json:"title"`
	Description Text              `json:"description"`
	Info        Text              `json:"info"`
	Kind        panel.Kind        `json:"kind"`
	Dataset     string            `json:"dataset,omitempty"`
	Span        int               `json:"span,omitempty"`
	Height      string            `json:"height,omitempty"`
	Colors      []string          `json:"colors,omitempty"`
	ShowLegend  bool              `json:"showLegend,omitempty"`
	Fields      FieldMappingSpec  `json:"fields,omitempty"`
	Formatter   *format.Spec      `json:"formatter,omitempty"`
	Columns     []TableColumnSpec `json:"columns,omitempty"`
	Transforms  []transform.Spec  `json:"transforms,omitempty"`
	Action      *action.Spec      `json:"action,omitempty"`
	Children    []PanelSpec       `json:"children,omitempty"`
	ClassName   string            `json:"className,omitempty"`
	Chrome      chrome.Spec       `json:"-"`
	ChromeIcon  string            `json:"icon,omitempty"`
	AccentColor string            `json:"accentColor,omitempty"`
	ValueAxis   panel.ValueAxis   `json:"valueAxis,omitempty"`
	Distributed bool              `json:"distributed,omitempty"`
	ColorField  string            `json:"colorField,omitempty"`
	ColorScale  string            `json:"colorScale,omitempty"`
}

type TableColumnSpec struct {
	Field     string       `json:"field,omitempty"`
	Label     Text         `json:"label"`
	Formatter *format.Spec `json:"formatter,omitempty"`
	Action    *action.Spec `json:"action,omitempty"`
	Text      Text         `json:"text"`
}

type FieldMappingSpec struct {
	Label     string `json:"label,omitempty"`
	Value     string `json:"value,omitempty"`
	Series    string `json:"series,omitempty"`
	Category  string `json:"category,omitempty"`
	ID        string `json:"id,omitempty"`
	StartTime string `json:"startTime,omitempty"`
	EndTime   string `json:"endTime,omitempty"`
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
