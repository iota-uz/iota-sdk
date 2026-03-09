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
	Kind    string
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

func Dashboard(id, title string, rows ...RowSpec) DashboardSpec {
	return DashboardSpec{ID: id, Title: title, Rows: rows}
}

func (d DashboardSpec) WithDescription(description string) DashboardSpec {
	d.Description = description
	return d
}

func (d DashboardSpec) WithDatasets(datasets ...DatasetSpec) DashboardSpec {
	d.Datasets = append(d.Datasets, datasets...)
	return d
}

func (d DashboardSpec) WithVariables(variables ...VariableSpec) DashboardSpec {
	d.Variables = append(d.Variables, variables...)
	return d
}

func Row(panels ...panel.Spec) RowSpec {
	return RowSpec{Panels: panels}
}

func QueryDataset(name, source, text string, transforms ...transform.Spec) DatasetSpec {
	return DatasetSpec{
		Name:       name,
		Kind:       DatasetKindQuery,
		Source:     source,
		Query:      &QuerySpec{Text: text},
		Transforms: transforms,
	}
}

func TransformDataset(name string, dependsOn []string, transforms ...transform.Spec) DatasetSpec {
	return DatasetSpec{
		Name:       name,
		Kind:       DatasetKindTransform,
		DependsOn:  dependsOn,
		Transforms: transforms,
	}
}

func StaticDataset(name string, set *frame.FrameSet) DatasetSpec {
	return DatasetSpec{Name: name, Kind: DatasetKindStatic, Static: set}
}

func DateRangeVariable(name, label string, defaultDuration time.Duration) VariableSpec {
	return VariableSpec{
		Name:            name,
		Label:           label,
		Kind:            VariableDateRange,
		RequestKeys:     []string{name, name + "_start", name + "_end"},
		AllowAllTime:    true,
		DefaultDuration: defaultDuration,
		Default:         DateRangeValue{Mode: "default"},
	}
}

func ResolveTimeRange(value any) datasource.TimeRange {
	dateRange, ok := value.(DateRangeValue)
	if !ok {
		return datasource.TimeRange{}
	}
	return datasource.TimeRange{
		Start: dateRange.Start,
		End:   dateRange.End,
		Mode:  dateRange.Mode,
	}
}
