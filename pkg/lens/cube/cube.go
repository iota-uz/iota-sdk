// Package cube builds drillable Lens dashboards from SQL or in-memory datasets.
package cube

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/formula"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
)

type DataMode string

const (
	DataModeSQL     DataMode = "sql"
	DataModeDataset DataMode = "dataset"
	DataModeNamed   DataMode = "named"
)

type DimensionType string

const (
	DimensionTypeCategory   DimensionType = "category"
	DimensionTypeTimeBucket DimensionType = "time_bucket"
)

type Aggregation string

const (
	AggregationCount Aggregation = "count"
	AggregationSum   Aggregation = "sum"
	AggregationAvg   Aggregation = "avg"
)

type CubeSpec struct {
	ID               string
	Title            string
	Description      string
	DataMode         DataMode
	DataSource       string
	FromSQL          string
	NamedQuery       string
	Data             *frame.FrameSet
	Variables        []lens.VariableSpec
	Params           map[string]lens.ParamValue
	Where            []string
	Joins            []JoinSpec
	Dimensions       []DimensionSpec
	Measures         []MeasureSpec
	DefaultDimension string
	Layout           []LayoutRowSpec
	DrillPath        []string
	Leaf             LeafSpec
}

type JoinSpec struct {
	Name string
	SQL  string
}

type DimensionSpec struct {
	Name            string
	Label           string
	Type            DimensionType
	Column          string
	LabelColumn     string
	ColorColumn     string
	Field           string
	LabelField      string
	ColorField      string
	TimeColumn      string
	TimeGranularity string
	Facet           bool
	LabelMap        map[string]string
	PanelKind       panel.Kind
	Height          string
	Description     string
	RequiresJoin    []string
	Override        *lens.DatasetSpec
	Transforms      []transform.Spec
	Colors          []string
	ValueAxis       panel.ValueAxis
	ColorScale      string
	// Presentation carries renderer-level density choices onto the dimension's
	// panel. A cube dimension is a panel like any other, and some of these are
	// statements about the data rather than about density — ColorBySequence
	// says the categories are ranked, which a hand-written palette can only
	// imitate for as many steps as it happens to list.
	Presentation panel.PresentationHints
	Map          *panel.MapSpec
}

type MeasureSpec struct {
	Name         string
	Label        string
	Column       string
	Field        string
	Aggregation  Aggregation
	Formatter    *format.Spec
	AccentColor  string
	Description  string
	Info         string
	RequiresJoin []string
	Override     *lens.DatasetSpec
	Action       *action.Spec
	InvertTrend  bool
	// Formula optionally describes how the measure is calculated.
	Formula *formula.Spec
}

type LeafSpec struct {
	URL          string           `json:"url,omitempty"`
	Table        *LeafTableSpec   `json:"table,omitempty"`
	ExternalLink LeafLinkResolver `json:"-"`
}

func (s CubeSpec) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("cube id is required")
	}
	if strings.TrimSpace(s.Title) == "" {
		return fmt.Errorf("cube title is required")
	}
	if len(s.Dimensions) == 0 {
		return fmt.Errorf("cube %q requires at least one dimension", s.ID)
	}
	if len(s.Measures) == 0 {
		return fmt.Errorf("cube %q requires at least one measure", s.ID)
	}
	for _, dimension := range s.Dimensions {
		if dimension.PanelKind == panel.KindMap && dimension.Map == nil {
			return fmt.Errorf("cube %q map dimension %q requires map config", s.ID, dimension.Name)
		}
		if dimension.PanelKind == panel.KindBoxPlot || dimension.PanelKind == panel.KindHeatmap {
			return fmt.Errorf("cube %q dimension %q cannot use panel kind %q because cube dimensions do not provide its required fields", s.ID, dimension.Name, dimension.PanelKind)
		}
	}
	variableKinds := make(map[string]lens.VariableKind, len(s.Variables))
	compareCount := 0
	for _, variable := range s.Variables {
		variableKinds[variable.Name] = variable.Kind
		if variable.Kind == lens.VariableCompare {
			compareCount++
			if strings.TrimSpace(variable.CompareTo) == "" {
				return fmt.Errorf("cube %q comparison variable %q requires compare target", s.ID, variable.Name)
			}
		}
	}
	if compareCount > 1 {
		return fmt.Errorf("cube %q supports one comparison variable", s.ID)
	}
	for _, variable := range s.Variables {
		if variable.Kind == lens.VariableCompare && variableKinds[variable.CompareTo] != lens.VariableDateRange {
			return fmt.Errorf("cube %q comparison variable %q target %q must be a date range", s.ID, variable.Name, variable.CompareTo)
		}
	}
	switch s.DataMode {
	case DataModeSQL:
		if strings.TrimSpace(s.DataSource) == "" {
			return fmt.Errorf("cube %q requires datasource for sql mode", s.ID)
		}
		if strings.TrimSpace(s.FromSQL) == "" {
			return fmt.Errorf("cube %q requires FROM clause for sql mode", s.ID)
		}
	case DataModeDataset:
		if s.Data == nil {
			return fmt.Errorf("cube %q requires static frames for dataset mode", s.ID)
		}
	case DataModeNamed:
		if strings.TrimSpace(s.DataSource) == "" {
			return fmt.Errorf("cube %q requires datasource for named mode", s.ID)
		}
		if strings.TrimSpace(s.NamedQuery) == "" {
			return fmt.Errorf("cube %q requires query name for named mode", s.ID)
		}
	default:
		return fmt.Errorf("cube %q has unsupported mode %q", s.ID, s.DataMode)
	}
	for _, dim := range s.Dimensions {
		if strings.TrimSpace(dim.Name) == "" {
			return fmt.Errorf("cube %q has dimension without name", s.ID)
		}
		if strings.TrimSpace(dim.Label) == "" {
			return fmt.Errorf("cube %q dimension %q requires label", s.ID, dim.Name)
		}
		if dim.Type == DimensionTypeTimeBucket {
			if strings.TrimSpace(dim.TimeColumn) == "" && strings.TrimSpace(dim.Column) == "" && strings.TrimSpace(dim.Field) == "" {
				return fmt.Errorf("cube %q time-bucket dimension %q requires time column or field", s.ID, dim.Name)
			}
			if !validTimeGranularity(dim.TimeGranularity) {
				return fmt.Errorf("cube %q time-bucket dimension %q has unsupported granularity %q", s.ID, dim.Name, dim.TimeGranularity)
			}
		}
		switch s.DataMode {
		case DataModeSQL:
			if dim.Override == nil && strings.TrimSpace(dim.sqlColumn()) == "" {
				return fmt.Errorf("cube %q dimension %q requires column in sql mode", s.ID, dim.Name)
			}
		case DataModeDataset, DataModeNamed:
			if dim.Override == nil && strings.TrimSpace(dim.datasetSourceField()) == "" {
				return fmt.Errorf("cube %q dimension %q requires field in dataset mode", s.ID, dim.Name)
			}
		}
	}
	for _, measure := range s.Measures {
		if strings.TrimSpace(measure.Name) == "" {
			return fmt.Errorf("cube %q has measure without name", s.ID)
		}
		if strings.TrimSpace(measure.Label) == "" {
			return fmt.Errorf("cube %q measure %q requires label", s.ID, measure.Name)
		}
		switch s.DataMode {
		case DataModeSQL:
			if strings.TrimSpace(measure.Column) == "" {
				return fmt.Errorf("cube %q measure %q requires column in sql mode", s.ID, measure.Name)
			}
		case DataModeDataset, DataModeNamed:
			if measure.Aggregation != AggregationCount && strings.TrimSpace(measure.Field) == "" {
				return fmt.Errorf("cube %q measure %q requires field in dataset mode", s.ID, measure.Name)
			}
		}
	}
	seenDrill := make(map[string]struct{}, len(s.DrillPath))
	for _, dimName := range s.DrillPath {
		if _, ok := s.Dimension(dimName); !ok {
			return fmt.Errorf("cube %q drill path references unknown dimension %q", s.ID, dimName)
		}
		if _, dup := seenDrill[dimName]; dup {
			return fmt.Errorf("cube %q drill path duplicates dimension %q", s.ID, dimName)
		}
		seenDrill[dimName] = struct{}{}
	}
	seenChartIDs := make(map[string]struct{})
	for _, row := range s.Layout {
		switch row.Kind {
		case LayoutRowKPI:
			for _, mName := range row.Measures {
				if _, ok := s.Measure(mName); !ok {
					return fmt.Errorf("cube %q KPI row references unknown measure %q", s.ID, mName)
				}
			}
		case LayoutRowCharts:
			for _, chart := range row.Charts {
				if strings.TrimSpace(chart.ID) == "" {
					return fmt.Errorf("cube %q layout chart requires id", s.ID)
				}
				if _, dup := seenChartIDs[chart.ID]; dup {
					return fmt.Errorf("cube %q layout duplicates chart id %q", s.ID, chart.ID)
				}
				seenChartIDs[chart.ID] = struct{}{}
				if _, ok := s.Dimension(chart.Dimension); !ok {
					return fmt.Errorf("cube %q chart %q references unknown dimension %q", s.ID, chart.ID, chart.Dimension)
				}
				if chart.Series != "" {
					if _, ok := s.Dimension(chart.Series); !ok {
						return fmt.Errorf("cube %q chart %q references unknown series dimension %q", s.ID, chart.ID, chart.Series)
					}
					if chart.Series == chart.Dimension {
						return fmt.Errorf("cube %q chart %q series dimension must differ from primary dimension %q", s.ID, chart.ID, chart.Dimension)
					}
				}
				if chart.Measure != "" {
					if _, ok := s.Measure(chart.Measure); !ok {
						return fmt.Errorf("cube %q chart %q references unknown measure %q", s.ID, chart.ID, chart.Measure)
					}
				}
				if chart.TopN < 0 {
					return fmt.Errorf("cube %q chart %q topN cannot be negative", s.ID, chart.ID)
				}
			}
		}
	}
	if s.Leaf.Table != nil && strings.TrimSpace(s.Leaf.Table.ID) == "" {
		return fmt.Errorf("cube %q leaf table requires id", s.ID)
	}
	return nil
}

func (s CubeSpec) Dimension(name string) (DimensionSpec, bool) {
	for _, dim := range s.Dimensions {
		if dim.Name == name {
			return dim, true
		}
	}
	return DimensionSpec{}, false
}

func (s CubeSpec) Measure(name string) (MeasureSpec, bool) {
	for _, measure := range s.Measures {
		if measure.Name == name {
			return measure, true
		}
	}
	return MeasureSpec{}, false
}

// FacetDimensions returns dimensions declared with AsFacet() in spec order.
func (s CubeSpec) FacetDimensions() []DimensionSpec {
	out := make([]DimensionSpec, 0)
	for _, dim := range s.Dimensions {
		if dim.Facet {
			out = append(out, dim)
		}
	}
	return out
}

// HasLeaf reports whether the cube defines an embedded leaf table, an external
// link resolver, or a static leaf URL.
func (s CubeSpec) HasLeaf() bool {
	return s.Leaf.Table != nil || s.Leaf.ExternalLink != nil || strings.TrimSpace(s.Leaf.URL) != ""
}

// ResolveExternalLeafURL resolves the cube's external leaf link (if configured)
// for the given drill context and params.
func (s CubeSpec) ResolveExternalLeafURL(ctx context.Context, drill DrillContext, params map[string]any) string {
	if s.Leaf.ExternalLink != nil {
		if resolved := strings.TrimSpace(s.Leaf.ExternalLink(ctx, drill, params)); resolved != "" {
			return resolved
		}
	}
	if raw := strings.TrimSpace(s.Leaf.URL); raw != "" {
		return mergeURL(raw, drill.WithValues(drill.Request))
	}
	return ""
}

// NextDrillDimension returns the next unfiltered dimension along DrillPath
// (or ordered dimensions when DrillPath is empty).
func (s CubeSpec) NextDrillDimension(drill DrillContext) string {
	return nextDrillDimensionExcluding(s, drill, "")
}

func nextDrillDimensionExcluding(s CubeSpec, drill DrillContext, exclude string) string {
	if len(s.DrillPath) > 0 {
		for _, name := range s.DrillPath {
			if name == exclude || drill.ContainsDimension(name) {
				continue
			}
			return name
		}
		return ""
	}
	for _, dim := range orderedDimensions(s) {
		if dim.Name == exclude || drill.ContainsDimension(dim.Name) {
			continue
		}
		return dim.Name
	}
	return ""
}

// DisplayLabel resolves a raw dimension filter value through LabelMap if defined.
func (d DimensionSpec) DisplayLabel(value string) string {
	if len(d.LabelMap) > 0 {
		if mapped, ok := d.LabelMap[value]; ok && strings.TrimSpace(mapped) != "" {
			return mapped
		}
	}
	return value
}

func (d DimensionSpec) sqlColumn() string {
	if d.Type == DimensionTypeTimeBucket {
		col := strings.TrimSpace(d.TimeColumn)
		if col == "" {
			col = strings.TrimSpace(d.Column)
		}
		if col == "" {
			return ""
		}
		return timeBucketSQLExpr(col, d.TimeGranularity)
	}
	return strings.TrimSpace(d.Column)
}

func (d DimensionSpec) sqlLabelColumn() string {
	if labelCol := strings.TrimSpace(d.LabelColumn); labelCol != "" {
		return labelCol
	}
	baseExpr := d.sqlColumn()
	if len(d.LabelMap) == 0 || baseExpr == "" {
		return baseExpr
	}
	keys := make([]string, 0, len(d.LabelMap))
	for key := range d.LabelMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("CASE (")
	b.WriteString(baseExpr)
	b.WriteString(")::text")
	for _, key := range keys {
		b.WriteString(" WHEN '")
		b.WriteString(escapeSQLLiteral(key))
		b.WriteString("' THEN '")
		b.WriteString(escapeSQLLiteral(d.LabelMap[key]))
		b.WriteByte('\'')
	}
	b.WriteString(" ELSE (")
	b.WriteString(baseExpr)
	b.WriteString(")::text END")
	return b.String()
}

func escapeSQLLiteral(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func (d DimensionSpec) datasetSourceField() string {
	if d.Type == DimensionTypeTimeBucket {
		if f := strings.TrimSpace(d.Field); f != "" {
			return f
		}
		return strings.TrimSpace(d.TimeColumn)
	}
	if f := strings.TrimSpace(d.Field); f != "" {
		return f
	}
	return strings.TrimSpace(d.LabelField)
}

func (d DimensionSpec) datasetValueField() string {
	if d.Type == DimensionTypeTimeBucket {
		return "__cube_tb_" + strings.ReplaceAll(strings.TrimSpace(d.Name), " ", "_")
	}
	return d.datasetSourceField()
}
