// Package apex renders Lens panel results into ApexCharts options.
package apex

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/components/charts"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

func Options(panelSpec panel.Spec, panelResult *runtime.PanelResult) charts.ChartOptions {
	options := charts.ChartOptions{
		Chart: charts.ChartConfig{
			Type:    chartType(panelSpec.Kind),
			Height:  panelHeight(panelSpec),
			Toolbar: charts.Toolbar{Show: false},
			Stacked: panelSpec.Kind == panel.KindStackedBar,
		},
		DataLabels: &charts.DataLabels{Enabled: false},
		Colors:     panelColors(panelSpec),
	}
	if panelResult == nil || panelResult.Frames == nil || panelResult.Frames.Primary() == nil {
		return options
	}
	fr := panelResult.Frames.Primary()
	rows := fr.Rows()
	fields := panelSpec.Fields
	if fields.Label == "" {
		fields.Label = "label"
	}
	if fields.Value == "" {
		fields.Value = "value"
	}
	if fields.Series == "" {
		fields.Series = "series"
	}
	if fields.Category == "" {
		fields.Category = "category"
	}

	switch panelSpec.Kind {
	case panel.KindPie, panel.KindDonut, panel.KindGauge:
		labels := make([]string, 0, len(rows))
		values := make([]any, 0, len(rows))
		for _, row := range rows {
			labels = append(labels, fmt.Sprint(row[fields.Label]))
			values = append(values, numericValue(row[fields.Value]))
		}
		options.Labels = labels
		options.Series = values
		if panelSpec.Kind == panel.KindDonut {
			size := "70%"
			position := charts.LegendPositionBottom
			options.Legend = &charts.LegendConfig{Position: &position}
			options.PlotOptions = &charts.PlotOptions{Pie: &charts.PieDonutConfig{Donut: &charts.DonutSpecifics{Size: &size}}}
		}
		if panelSpec.Kind == panel.KindGauge {
			startAngle := -135
			endAngle := 225
			hollowSize := "70%"
			options.PlotOptions = &charts.PlotOptions{
				RadialBar: &charts.RadialBarConfig{
					StartAngle: &startAngle,
					EndAngle:   &endAngle,
					Hollow:     &charts.RadialBarHollow{Size: &hollowSize},
				},
			}
		}
	case panel.KindStat, panel.KindTimeSeries, panel.KindBar, panel.KindHorizontalBar, panel.KindStackedBar, panel.KindTable, panel.KindTabs, panel.KindGrid, panel.KindSplit, panel.KindRepeat:
		if hasSeries(rows, fields.Series) {
			categories, series := groupedSeries(rows, fields)
			options.Series = series
			options.XAxis = charts.XAxisConfig{Categories: categories}
		} else {
			categories := make([]string, 0, len(rows))
			values := make([]any, 0, len(rows))
			for _, row := range rows {
				categories = append(categories, displayValue(row[fields.Label]))
				values = append(values, numericValue(row[fields.Value]))
			}
			options.Series = []charts.Series{{Name: panelSpec.Title, Data: values}}
			options.XAxis = charts.XAxisConfig{Categories: categories}
		}
	}

	if panelSpec.Kind == panel.KindHorizontalBar {
		horizontal := true
		options.PlotOptions = &charts.PlotOptions{Bar: &charts.BarConfig{Horizontal: &horizontal, BorderRadius: 8}}
	}
	if panelSpec.Kind == panel.KindBar || panelSpec.Kind == panel.KindStackedBar || panelSpec.Kind == panel.KindTimeSeries {
		if options.PlotOptions == nil {
			options.PlotOptions = &charts.PlotOptions{Bar: &charts.BarConfig{BorderRadius: 4}}
		}
	}
	if panelSpec.ShowLegend {
		position := charts.LegendPositionBottom
		options.Legend = &charts.LegendConfig{Position: &position, Show: mapping.Pointer(true)}
	}
	if panelSpec.Action != nil {
		options.Chart.Events = &charts.ChartEvents{DataPointSelection: buildActionJS(panelSpec.Action, fr, fields, panelResult.Variables)}
	}
	return options
}

func buildActionJS(spec *action.Spec, fr *frame.Frame, fields panel.FieldMapping, variables map[string]any) templ.JSExpression {
	rowsJSON := rowsToJSON(fr.Rows())
	urlJSON := fmt.Sprintf("%q", spec.URL)
	method := spec.Method
	if method == "" {
		method = "GET"
	}
	var actionJS string
	switch spec.Kind {
	case action.KindNavigate:
		actionJS = "window.location.href = nextURL;"
	case action.KindHtmxSwap:
		target := spec.Target
		actionJS = fmt.Sprintf("htmx.ajax(%q, nextURL, {target: %q, swap: 'innerHTML'});", method, target)
	case action.KindEmitEvent:
		actionJS = fmt.Sprintf("document.dispatchEvent(new CustomEvent(%q, {detail: payload}));", spec.Event)
	}
	js := fmt.Sprintf(`function(event, chartContext, opts) {
		const rows = %s;
		const config = chartContext.w.config;
		const categories = (config.xaxis && config.xaxis.categories) ? config.xaxis.categories : [];
		const seriesName = config.series && config.series[opts.seriesIndex] ? config.series[opts.seriesIndex].name : '';
		const categoryName = categories[opts.dataPointIndex] || '';
		const normalizeCategoryValue = function(value) {
			if (value === undefined || value === null || value === '') {
				return '';
			}
			const stringValue = String(value);
			if (/^\d{4}-\d{2}-\d{2}$/.test(stringValue)) {
				return stringValue;
			}
			const parsed = new Date(stringValue);
			if (!Number.isNaN(parsed.getTime())) {
				return parsed.toISOString().slice(0, 10);
			}
			return stringValue;
		};
		let row = rows[opts.dataPointIndex] || {};
		const groupedMatch = rows.find(function(item) {
			const categoryValue = item[%q] || item[%q] || item[%q];
			const seriesValue = item[%q] || '';
			return normalizeCategoryValue(categoryValue) === normalizeCategoryValue(categoryName) && String(seriesValue) === String(seriesName);
		});
		if (groupedMatch) {
			row = groupedMatch;
		}
		let nextURL = %s;
		const payload = {};
		const params = new URLSearchParams();
	`, rowsJSON, fields.Category, fields.Label, fields.StartTime, fields.Series, urlJSON)
	for _, param := range spec.Params {
		switch param.Source.Kind {
		case action.SourceField:
			js += fmt.Sprintf("if (row[%q] !== undefined && row[%q] !== null) { params.append(%q, row[%q]); payload[%q] = row[%q]; }\n", param.Source.Name, param.Source.Name, param.Name, param.Source.Name, param.Name, param.Source.Name)
		case action.SourcePoint:
			switch param.Source.Name {
			case "label":
				js += fmt.Sprintf("if ((row[%q] || row[%q] || categoryName) !== undefined && (row[%q] || row[%q] || categoryName) !== null && (row[%q] || row[%q] || categoryName) !== '') { params.append(%q, row[%q] || row[%q] || categoryName); payload[%q] = row[%q] || row[%q] || categoryName; }\n", fields.Label, fields.Category, fields.Label, fields.Category, fields.Label, fields.Category, param.Name, fields.Label, fields.Category, param.Name, fields.Label, fields.Category)
			case "value":
				js += fmt.Sprintf("if (row[%q] !== undefined && row[%q] !== null) { params.append(%q, row[%q]); payload[%q] = row[%q]; }\n", fields.Value, fields.Value, param.Name, fields.Value, param.Name, fields.Value)
			case "series":
				js += fmt.Sprintf("if (seriesName) { params.append(%q, seriesName); payload[%q] = seriesName; }\n", param.Name, param.Name)
			case "category":
				js += fmt.Sprintf("if (categoryName) { params.append(%q, categoryName); payload[%q] = categoryName; }\n", param.Name, param.Name)
			}
		case action.SourceVariable:
			if value, ok := variables[param.Source.Name]; ok && value != nil && fmt.Sprint(value) != "" {
				js += fmt.Sprintf("params.append(%q, %q); payload[%q] = %q;\n", param.Name, fmt.Sprint(value), param.Name, fmt.Sprint(value))
			}
		case action.SourceLiteral:
			js += fmt.Sprintf("params.append(%q, %q); payload[%q] = %q;\n", param.Name, fmt.Sprint(param.Source.Value), param.Name, fmt.Sprint(param.Source.Value))
		}
	}
	for key, source := range spec.Payload {
		switch source.Kind {
		case action.SourceField:
			js += fmt.Sprintf("if (row[%q] !== undefined && row[%q] !== null) { payload[%q] = row[%q]; }\n", source.Name, source.Name, key, source.Name)
		case action.SourcePoint:
			switch source.Name {
			case "label":
				js += fmt.Sprintf("payload[%q] = row[%q] || row[%q] || categoryName;\n", key, fields.Label, fields.Category)
			case "value":
				js += fmt.Sprintf("payload[%q] = row[%q];\n", key, fields.Value)
			case "series":
				js += fmt.Sprintf("payload[%q] = seriesName;\n", key)
			case "category":
				js += fmt.Sprintf("payload[%q] = categoryName;\n", key)
			}
		case action.SourceVariable:
			if value, ok := variables[source.Name]; ok && value != nil {
				js += fmt.Sprintf("payload[%q] = %q;\n", key, fmt.Sprint(value))
			}
		case action.SourceLiteral:
			js += fmt.Sprintf("payload[%q] = %q;\n", key, fmt.Sprint(source.Value))
		}
	}
	js += `const query = params.toString();
		if (query) {
			nextURL = nextURL + (nextURL.includes('?') ? '&' : '?') + query;
		}
	` + actionJS + `}`
	return templ.JSExpression(js)
}

func rowsToJSON(rows []map[string]any) string {
	payload, err := json.Marshal(rows)
	if err != nil {
		return "[]"
	}
	return string(payload)
}

func chartType(kind panel.Kind) charts.ChartType {
	switch kind {
	case panel.KindTimeSeries:
		return charts.LineChartType
	case panel.KindPie:
		return charts.PieChartType
	case panel.KindDonut:
		return charts.DonutChartType
	case panel.KindGauge:
		return charts.RadialBarChartType
	case panel.KindStat, panel.KindBar, panel.KindHorizontalBar, panel.KindStackedBar, panel.KindTable, panel.KindTabs, panel.KindGrid, panel.KindSplit, panel.KindRepeat:
		return charts.BarChartType
	}
	return charts.BarChartType
}

func panelHeight(panelSpec panel.Spec) string {
	if panelSpec.Height != "" {
		return panelSpec.Height
	}
	return "320px"
}

func panelColors(panelSpec panel.Spec) []string {
	if len(panelSpec.Colors) > 0 {
		return panelSpec.Colors
	}
	switch panelSpec.Kind {
	case panel.KindTimeSeries:
		return []string{"#0f766e"}
	case panel.KindStackedBar:
		return []string{"#2563eb", "#10b981", "#f59e0b", "#8b5cf6", "#ef4444"}
	case panel.KindPie, panel.KindDonut:
		return []string{"#2563eb", "#10b981", "#f59e0b", "#8b5cf6", "#ef4444"}
	case panel.KindGauge:
		return []string{"#f59e0b"}
	case panel.KindStat, panel.KindBar, panel.KindHorizontalBar, panel.KindTable, panel.KindTabs, panel.KindGrid, panel.KindSplit, panel.KindRepeat:
		return []string{"#2563eb"}
	}
	return []string{"#2563eb"}
}

func hasSeries(rows []map[string]any, field string) bool {
	if field == "" {
		return false
	}
	for _, row := range rows {
		if value, ok := row[field]; ok && fmt.Sprint(value) != "" {
			return true
		}
	}
	return false
}

func groupedSeries(rows []map[string]any, fields panel.FieldMapping) ([]string, []charts.Series) {
	categoryOrder := make([]string, 0)
	categorySeen := map[string]bool{}
	seriesOrder := make([]string, 0)
	seriesSeen := map[string]bool{}
	index := map[string]map[string]float64{}
	for _, row := range rows {
		category := displayValue(firstNonEmpty(row[fields.Category], row[fields.Label]))
		series := displayValue(row[fields.Series])
		if !categorySeen[category] {
			categorySeen[category] = true
			categoryOrder = append(categoryOrder, category)
		}
		if !seriesSeen[series] {
			seriesSeen[series] = true
			seriesOrder = append(seriesOrder, series)
		}
		if _, ok := index[series]; !ok {
			index[series] = make(map[string]float64)
		}
		index[series][category] = numericValue(row[fields.Value])
	}
	series := make([]charts.Series, 0, len(seriesOrder))
	for _, name := range seriesOrder {
		values := make([]any, len(categoryOrder))
		for i, category := range categoryOrder {
			values[i] = index[name][category]
		}
		series = append(series, charts.Series{Name: name, Data: values})
	}
	return categoryOrder, series
}

func firstNonEmpty(values ...any) any {
	for _, value := range values {
		if value == nil {
			continue
		}
		if strings.TrimSpace(displayValue(value)) != "" {
			return value
		}
	}
	return ""
}

func numericValue(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	case uint:
		return float64(v)
	case uint64:
		return float64(v)
	case uint32:
		return float64(v)
	case uint16:
		return float64(v)
	case uint8:
		return float64(v)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err == nil {
			return parsed
		}
		return 0
	default:
		return 0
	}
}

func displayValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case *string:
		if v == nil {
			return ""
		}
		return *v
	case time.Time:
		return v.Format("2006-01-02")
	case *time.Time:
		if v == nil {
			return ""
		}
		return v.Format("2006-01-02")
	default:
		return fmt.Sprint(v)
	}
}
