// Package build provides ergonomic builders for Lens dashboard specs.
package build

import (
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
)

type DashboardBuilder struct {
	spec lens.DashboardSpec
}

func Dashboard(id, title string, rows ...lens.RowSpec) *DashboardBuilder {
	return &DashboardBuilder{
		spec: lens.DashboardSpec{
			ID:    id,
			Title: title,
			Rows:  append([]lens.RowSpec(nil), rows...),
		},
	}
}

func (b *DashboardBuilder) Description(description string) *DashboardBuilder {
	b.spec.Description = description
	return b
}

func (b *DashboardBuilder) Rows(rows ...lens.RowSpec) *DashboardBuilder {
	b.spec.Rows = append(b.spec.Rows, rows...)
	return b
}

func (b *DashboardBuilder) Datasets(datasets ...lens.DatasetSpec) *DashboardBuilder {
	b.spec.Datasets = append(b.spec.Datasets, datasets...)
	return b
}

func (b *DashboardBuilder) Variables(variables ...lens.VariableSpec) *DashboardBuilder {
	b.spec.Variables = append(b.spec.Variables, variables...)
	return b
}

func (b *DashboardBuilder) Build() lens.DashboardSpec {
	return b.spec
}

func Row(panels ...panel.Spec) lens.RowSpec {
	return lens.RowSpec{Panels: append([]panel.Spec(nil), panels...)}
}

func QueryDataset(name, source, text string, transforms ...transform.Spec) lens.DatasetSpec {
	return lens.DatasetSpec{
		Name:       name,
		Kind:       lens.DatasetKindQuery,
		Source:     source,
		Query:      &lens.QuerySpec{Text: text, Kind: datasource.QueryKindRaw},
		Transforms: transforms,
	}
}

func TransformDataset(name string, dependsOn []string, transforms ...transform.Spec) lens.DatasetSpec {
	return lens.DatasetSpec{
		Name:       name,
		Kind:       lens.DatasetKindTransform,
		DependsOn:  append([]string(nil), dependsOn...),
		Transforms: append([]transform.Spec(nil), transforms...),
	}
}

func StaticDataset(name string, set *frame.FrameSet) lens.DatasetSpec {
	if set == nil {
		empty, _ := frame.NewFrameSet()
		return lens.DatasetSpec{Name: name, Kind: lens.DatasetKindStatic, Static: empty}
	}
	return lens.DatasetSpec{Name: name, Kind: lens.DatasetKindStatic, Static: set.Clone()}
}

func DateRangeVariable(name, label string, defaultDuration time.Duration) lens.VariableSpec {
	return lens.VariableSpec{
		Name:            name,
		Label:           label,
		Kind:            lens.VariableDateRange,
		RequestKeys:     []string{name, name + "_start", name + "_end"},
		AllowAllTime:    true,
		DefaultDuration: defaultDuration,
		Default:         lens.DateRangeValue{Mode: "default"},
	}
}
