package spec

import (
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
)

func FromLegacyDashboard(legacy lens.DashboardSpec) Document {
	return Document{
		Version:     DocumentVersion,
		ID:          legacy.ID,
		Title:       LiteralText(legacy.Title),
		Description: LiteralText(legacy.Description),
		Variables:   fromLegacyVariables(legacy.Variables),
		Datasets:    FromLegacyDatasets(legacy.Datasets),
		Rows:        FromLegacyRows(legacy.Rows),
		Drill:       legacy.Drill,
	}
}

func FromLegacyDatasets(datasets []lens.DatasetSpec) []DatasetSpec {
	if len(datasets) == 0 {
		return nil
	}
	out := make([]DatasetSpec, 0, len(datasets))
	for _, dataset := range datasets {
		out = append(out, DatasetSpec{
			Name:        dataset.Name,
			Title:       LiteralText(dataset.Title),
			Kind:        dataset.Kind,
			Source:      dataset.Source,
			DependsOn:   append([]string(nil), dataset.DependsOn...),
			Query:       dataset.Query,
			Transforms:  append([]transform.Spec(nil), dataset.Transforms...),
			Static:      dataset.Static,
			Description: LiteralText(dataset.Description),
		})
	}
	return out
}

func FromLegacyRows(rows []lens.RowSpec) []RowSpec {
	if len(rows) == 0 {
		return nil
	}
	out := make([]RowSpec, 0, len(rows))
	for _, row := range rows {
		out = append(out, RowSpec{
			Panels: fromLegacyPanels(row.Panels),
			Class:  row.Class,
		})
	}
	return out
}

func fromLegacyVariables(variables []lens.VariableSpec) []VariableSpec {
	if len(variables) == 0 {
		return nil
	}
	out := make([]VariableSpec, 0, len(variables))
	for _, variable := range variables {
		options := make([]VariableOption, 0, len(variable.Options))
		for _, option := range variable.Options {
			options = append(options, VariableOption{
				Label: LiteralText(option.Label),
				Value: option.Value,
			})
		}
		out = append(out, VariableSpec{
			Name:            variable.Name,
			Label:           LiteralText(variable.Label),
			Kind:            variable.Kind,
			RequestKeys:     append([]string(nil), variable.RequestKeys...),
			Default:         variable.Default,
			Required:        variable.Required,
			Description:     LiteralText(variable.Description),
			Options:         options,
			AllowAllTime:    variable.AllowAllTime,
			DefaultDuration: Duration(variable.DefaultDuration),
		})
	}
	return out
}

func fromLegacyPanels(panelsIn []panel.Spec) []PanelSpec {
	if len(panelsIn) == 0 {
		return nil
	}
	out := make([]PanelSpec, 0, len(panelsIn))
	for _, item := range panelsIn {
		columns := make([]TableColumnSpec, 0, len(item.Columns))
		for _, column := range item.Columns {
			columns = append(columns, TableColumnSpec{
				Field:     column.Field.Name(),
				Label:     LiteralText(column.Label),
				Formatter: column.Formatter,
				Action:    column.Action,
				Text:      LiteralText(column.Text),
			})
		}
		out = append(out, PanelSpec{
			ID:          item.ID,
			Title:       LiteralText(item.Title),
			Description: LiteralText(item.Description),
			Info:        LiteralText(item.Info),
			Kind:        item.Kind,
			Dataset:     item.Dataset,
			Span:        item.Span,
			Height:      item.Height,
			Colors:      append([]string(nil), item.Colors...),
			ShowLegend:  item.ShowLegend,
			Fields: FieldMappingSpec{
				Label:     item.Fields.Label.Name(),
				Value:     item.Fields.Value.Name(),
				Series:    item.Fields.Series.Name(),
				Category:  item.Fields.Category.Name(),
				ID:        item.Fields.ID.Name(),
				StartTime: item.Fields.StartTime.Name(),
				EndTime:   item.Fields.EndTime.Name(),
			},
			Formatter:   item.Formatter,
			Columns:     columns,
			Transforms:  append([]transform.Spec(nil), item.Transforms...),
			Action:      item.Action,
			Children:    fromLegacyPanels(item.Children),
			ClassName:   item.ClassName,
			Chrome:      item.Chrome,
			ValueAxis:   item.ValueAxis,
			Distributed: item.Distributed,
			ColorField:  item.ColorField.Name(),
			ColorScale:  item.ColorScale,
		})
	}
	return out
}
