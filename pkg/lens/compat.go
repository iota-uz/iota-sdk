package lens

import (
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
)

// Deprecated: prefer github.com/iota-uz/iota-sdk/pkg/lens/build.Dashboard.
func Dashboard(id, title string, rows ...RowSpec) DashboardSpec {
	return DashboardSpec{ID: id, Title: title, Rows: rows}
}

// Deprecated: prefer github.com/iota-uz/iota-sdk/pkg/lens/build.Dashboard(...).Description(...).Build().
func (d DashboardSpec) WithDescription(description string) DashboardSpec {
	d.Description = description
	return d
}

// Deprecated: prefer github.com/iota-uz/iota-sdk/pkg/lens/build.Dashboard(...).Datasets(...).Build().
func (d DashboardSpec) WithDatasets(datasets ...DatasetSpec) DashboardSpec {
	d.Datasets = append(d.Datasets, datasets...)
	return d
}

// Deprecated: prefer github.com/iota-uz/iota-sdk/pkg/lens/build.Dashboard(...).Variables(...).Build().
func (d DashboardSpec) WithVariables(variables ...VariableSpec) DashboardSpec {
	d.Variables = append(d.Variables, variables...)
	return d
}

// Deprecated: prefer github.com/iota-uz/iota-sdk/pkg/lens/build.Row.
func Row(panels ...panel.Spec) RowSpec {
	return RowSpec{Panels: panels}
}

// Deprecated: prefer github.com/iota-uz/iota-sdk/pkg/lens/build.QueryDataset.
func QueryDataset(name, source, text string, transforms ...transform.Spec) DatasetSpec {
	return DatasetSpec{
		Name:       name,
		Kind:       DatasetKindQuery,
		Source:     source,
		Query:      &QuerySpec{Text: text, Kind: datasource.QueryKindRaw},
		Transforms: transforms,
	}
}

// Deprecated: prefer github.com/iota-uz/iota-sdk/pkg/lens/build.TransformDataset.
func TransformDataset(name string, dependsOn []string, transforms ...transform.Spec) DatasetSpec {
	return DatasetSpec{
		Name:       name,
		Kind:       DatasetKindTransform,
		DependsOn:  dependsOn,
		Transforms: transforms,
	}
}

// Deprecated: prefer github.com/iota-uz/iota-sdk/pkg/lens/build.StaticDataset.
func StaticDataset(name string, set *frame.FrameSet) DatasetSpec {
	if set == nil {
		empty, _ := frame.NewFrameSet()
		return DatasetSpec{Name: name, Kind: DatasetKindStatic, Static: empty}
	}
	return DatasetSpec{Name: name, Kind: DatasetKindStatic, Static: set.Clone()}
}

// Deprecated: prefer github.com/iota-uz/iota-sdk/pkg/lens/build.DateRangeVariable.
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
