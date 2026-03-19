// Package cube builds drillable Lens dashboards from SQL or in-memory datasets.
package cube

import (
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
)

type DataMode string

const (
	DataModeSQL     DataMode = "sql"
	DataModeDataset DataMode = "dataset"
)

type DimensionType string

const (
	DimensionTypeCategory DimensionType = "category"
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
	Data             *frame.FrameSet
	Variables        []lens.VariableSpec
	Params           map[string]lens.ParamValue
	Where            []string
	Joins            []JoinSpec
	Dimensions       []DimensionSpec
	Measures         []MeasureSpec
	DefaultDimension string
	Leaf             LeafSpec
}

type JoinSpec struct {
	Name string
	SQL  string
}

type DimensionSpec struct {
	Name         string
	Label        string
	Type         DimensionType
	Column       string
	LabelColumn  string
	ColorColumn  string
	Field        string
	LabelField   string
	ColorField   string
	PanelKind    panel.Kind
	Height       string
	Description  string
	RequiresJoin []string
	Override     *lens.DatasetSpec
	Transforms   []transform.Spec
	Colors       []string
	ValueAxis    panel.ValueAxis
	ColorScale   string
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
	RequiresJoin []string
	Action       *action.Spec
}

type LeafSpec struct {
	URL string
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
		switch s.DataMode {
		case DataModeSQL:
			if dim.Override == nil && strings.TrimSpace(dim.Column) == "" {
				return fmt.Errorf("cube %q dimension %q requires column in sql mode", s.ID, dim.Name)
			}
		case DataModeDataset:
			if dim.Override == nil && strings.TrimSpace(dim.Field) == "" {
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
		case DataModeDataset:
			if measure.Aggregation != AggregationCount && strings.TrimSpace(measure.Field) == "" {
				return fmt.Errorf("cube %q measure %q requires field in dataset mode", s.ID, measure.Name)
			}
		}
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
