// Package lens defines dashboard specs, datasets, and variable models for Lens.
package lens

import (
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
)

type DatasetKind string

const (
	DatasetKindQuery     DatasetKind = "query"
	DatasetKindTransform DatasetKind = "transform"
	DatasetKindJoin      DatasetKind = "join"
	DatasetKindUnion     DatasetKind = "union"
	DatasetKindFormula   DatasetKind = "formula"
	DatasetKindStatic    DatasetKind = "static"
)

type VariableKind string

const (
	VariableDateRange    VariableKind = "date_range"
	VariableSingleSelect VariableKind = "single_select"
	VariableMultiSelect  VariableKind = "multi_select"
	VariableText         VariableKind = "text"
	VariableNumber       VariableKind = "number"
	VariableToggle       VariableKind = "toggle"
)

type DashboardSpec struct {
	ID          string
	Title       string
	Description string
	Rows        []RowSpec
	Variables   []VariableSpec
	Datasets    []DatasetSpec
}

type RowSpec struct {
	Panels []panel.Spec
	Class  string
}

type VariableOption struct {
	Label string
	Value string
}

type VariableSpec struct {
	Name            string
	Label           string
	Kind            VariableKind
	RequestKeys     []string
	Default         any
	Required        bool
	Description     string
	Options         []VariableOption
	AllowAllTime    bool
	DefaultDuration time.Duration
}

type DateRangeValue struct {
	Mode  string
	Start *time.Time
	End   *time.Time
}

type ParamValue struct {
	Literal  any
	Variable string
}

type QuerySpec struct {
	Text    string
	Params  map[string]ParamValue
	Kind    datasource.QueryKind
	MaxRows int
}

type DatasetSpec struct {
	Name        string
	Title       string
	Kind        DatasetKind
	Source      string
	DependsOn   []string
	Query       *QuerySpec
	Transforms  []transform.Spec
	Static      *frame.FrameSet
	Description string
}

func ResolveTimeRange(value any) datasource.TimeRange {
	dateRange, ok := value.(DateRangeValue)
	if !ok {
		return datasource.TimeRange{}
	}
	return datasource.TimeRange{
		Start: dateRange.Start,
		End:   dateRange.End,
		Mode:  datasource.TimeRangeMode(dateRange.Mode),
	}
}
