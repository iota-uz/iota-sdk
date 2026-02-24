package chart

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

func TestDrawChartTool_Name(t *testing.T) {
	t.Parallel()

	tool := NewDrawChartTool()
	if got := tool.Name(); got != "draw_chart" {
		t.Errorf("Name() = %q, want %q", got, "draw_chart")
	}
}

func TestDrawChartTool_CallStructured_RequiresOptions(t *testing.T) {
	t.Parallel()

	tool := NewDrawChartTool().(*DrawChartTool)
	result, err := tool.CallStructured(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("CallStructured() error = %v", err)
	}

	payload, ok := result.Payload.(types.ToolErrorPayload)
	if !ok {
		t.Fatalf("payload type = %T, want ToolErrorPayload", result.Payload)
	}
	if payload.Code != "INVALID_REQUEST" {
		t.Fatalf("payload.Code = %q, want INVALID_REQUEST", payload.Code)
	}
	if !strings.Contains(payload.Message, "options") {
		t.Fatalf("payload.Message = %q, want mention of options", payload.Message)
	}
}

func TestDrawChartTool_Parameters_RequireCanonicalOptions(t *testing.T) {
	t.Parallel()

	tool := NewDrawChartTool()
	schema := tool.Parameters()

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema.properties type = %T, want map[string]any", schema["properties"])
	}
	if _, ok := props["options"]; !ok {
		t.Fatal("schema.properties should include options")
	}
	required, ok := schema["required"].([]string)
	if !ok || len(required) != 1 || required[0] != "options" {
		t.Fatalf("schema.required = %v, want [options]", schema["required"])
	}
}

func TestDrawChartTool_CallStructured_ValidatesSeriesShape(t *testing.T) {
	t.Parallel()

	tool := NewDrawChartTool().(*DrawChartTool)

	resultLine, err := tool.CallStructured(context.Background(), `{"options":{"chart":{"type":"line"},"series":[1,2,3]}}`)
	if err != nil {
		t.Fatalf("CallStructured(line) error = %v", err)
	}
	payload, ok := resultLine.Payload.(types.ToolErrorPayload)
	if !ok {
		t.Fatalf("line payload type = %T, want ToolErrorPayload", resultLine.Payload)
	}
	if !strings.Contains(payload.Message, "array of objects") {
		t.Fatalf("line payload.Message = %q, want mention of array of objects", payload.Message)
	}

	resultPie, err := tool.CallStructured(context.Background(), `{"options":{"chart":{"type":"pie"},"series":[{"name":"Products","data":[30,45,25]}],"labels":["A","B","C"]}}`)
	if err != nil {
		t.Fatalf("CallStructured(pie) error = %v", err)
	}
	payload, ok = resultPie.Payload.(types.ToolErrorPayload)
	if !ok {
		t.Fatalf("pie payload type = %T, want ToolErrorPayload", resultPie.Payload)
	}
	if !strings.Contains(payload.Message, "numeric options.series") {
		t.Fatalf("pie payload.Message = %q, want mention of numeric options.series", payload.Message)
	}
}

func TestDrawChartTool_CallStructured_AppliesDefaults(t *testing.T) {
	t.Parallel()

	tool := NewDrawChartTool().(*DrawChartTool)
	result, err := tool.CallStructured(context.Background(), `{"options":{"chart":{"type":"bar"},"title":{"text":"Revenue"},"series":[{"name":"S","data":[1,2,3]}],"xaxis":{"categories":["Jan","Feb","Mar"]}}}`)
	if err != nil {
		t.Fatalf("CallStructured() error = %v", err)
	}

	payload, ok := result.Payload.(types.JSONPayload)
	if !ok {
		t.Fatalf("payload type = %T, want JSONPayload", result.Payload)
	}
	options, ok := payload.Output.(map[string]any)
	if !ok {
		t.Fatalf("payload.Output type = %T, want map[string]any", payload.Output)
	}

	chartCfg, ok := options["chart"].(map[string]any)
	if !ok {
		t.Fatalf("options.chart type = %T, want map[string]any", options["chart"])
	}
	if chartCfg["height"] != defaultChartHeight {
		t.Fatalf("chart.height = %v, want %d", chartCfg["height"], defaultChartHeight)
	}
	if chartCfg["type"] != "bar" {
		t.Fatalf("chart.type = %v, want bar", chartCfg["type"])
	}

	if _, ok := options["colors"].([]any); !ok {
		t.Fatalf("options.colors type = %T, want []any", options["colors"])
	}
}

func TestDrawChartTool_CallStructured_AutoEnablesLogScale(t *testing.T) {
	t.Parallel()

	tool := NewDrawChartTool().(*DrawChartTool)
	result, err := tool.CallStructured(context.Background(), `{"options":{"chart":{"type":"line"},"title":{"text":"Spread"},"series":[{"name":"S","data":[1,100,10000]}]}}`)
	if err != nil {
		t.Fatalf("CallStructured() error = %v", err)
	}

	payload := result.Payload.(types.JSONPayload)
	options := payload.Output.(map[string]any)
	yaxis, ok := options["yaxis"].(map[string]any)
	if !ok {
		t.Fatalf("options.yaxis type = %T, want map[string]any", options["yaxis"])
	}
	if logRaw, ok := yaxis["logarithmic"].(bool); !ok || !logRaw {
		t.Fatalf("yaxis.logarithmic = %v, want true", yaxis["logarithmic"])
	}
}

func TestDrawChartTool_CallStructured_DoesNotAutoEnableLogScaleWhenNonPositive(t *testing.T) {
	t.Parallel()

	tool := NewDrawChartTool().(*DrawChartTool)
	result, err := tool.CallStructured(context.Background(), `{"options":{"chart":{"type":"line"},"title":{"text":"Mixed"},"series":[{"name":"S","data":[0,100,10000]}]}}`)
	if err != nil {
		t.Fatalf("CallStructured() error = %v", err)
	}

	payload := result.Payload.(types.JSONPayload)
	options := payload.Output.(map[string]any)
	yaxisRaw, ok := options["yaxis"]
	if !ok {
		return
	}
	yaxis, ok := yaxisRaw.(map[string]any)
	if !ok {
		t.Fatalf("options.yaxis type = %T, want map[string]any", yaxisRaw)
	}
	if logRaw, ok := yaxis["logarithmic"].(bool); ok && logRaw {
		t.Fatalf("yaxis.logarithmic = true, want false/absent")
	}
}

func TestDrawChartTool_CallStructured_RejectsExplicitLogScaleWithNonPositive(t *testing.T) {
	t.Parallel()

	tool := NewDrawChartTool().(*DrawChartTool)
	result, err := tool.CallStructured(context.Background(), `{"options":{"chart":{"type":"line"},"yaxis":{"logarithmic":true},"series":[{"name":"S","data":[0,10,100]}]}}`)
	if err != nil {
		t.Fatalf("CallStructured() error = %v", err)
	}

	payload, ok := result.Payload.(types.ToolErrorPayload)
	if !ok {
		t.Fatalf("payload type = %T, want ToolErrorPayload", result.Payload)
	}
	if payload.Code != "INVALID_REQUEST" {
		t.Fatalf("payload.Code = %q, want INVALID_REQUEST", payload.Code)
	}
	if !strings.Contains(payload.Message, "logarithmic") {
		t.Fatalf("payload.Message = %q, want mention of logarithmic", payload.Message)
	}
}

func TestDrawChartTool_CallStructured_ValidatesTitleHeightAndColors(t *testing.T) {
	t.Parallel()

	tool := NewDrawChartTool().(*DrawChartTool)

	resultTitle, err := tool.CallStructured(context.Background(), `{"options":{"chart":{"type":"line"},"title":"Revenue","series":[{"name":"S","data":[1,2,3]}]}}`)
	if err != nil {
		t.Fatalf("CallStructured(title) error = %v", err)
	}
	errPayload, ok := resultTitle.Payload.(types.ToolErrorPayload)
	if !ok {
		t.Fatalf("title payload type = %T, want ToolErrorPayload", resultTitle.Payload)
	}
	if !strings.Contains(errPayload.Message, "options.title") {
		t.Fatalf("title payload.Message = %q, want mention of options.title", errPayload.Message)
	}

	resultHeight, err := tool.CallStructured(context.Background(), `{"options":{"chart":{"type":"line","height":50},"series":[{"name":"S","data":[1,2,3]}]}}`)
	if err != nil {
		t.Fatalf("CallStructured(height) error = %v", err)
	}
	errPayload, ok = resultHeight.Payload.(types.ToolErrorPayload)
	if !ok {
		t.Fatalf("height payload type = %T, want ToolErrorPayload", resultHeight.Payload)
	}
	if !strings.Contains(errPayload.Message, "height") {
		t.Fatalf("height payload.Message = %q, want mention of height", errPayload.Message)
	}

	resultColor, err := tool.CallStructured(context.Background(), `{"options":{"chart":{"type":"line"},"series":[{"name":"S","data":[1,2,3]}],"colors":["#GGGGGG"]}}`)
	if err != nil {
		t.Fatalf("CallStructured(color) error = %v", err)
	}
	errPayload, ok = resultColor.Payload.(types.ToolErrorPayload)
	if !ok {
		t.Fatalf("color payload type = %T, want ToolErrorPayload", resultColor.Payload)
	}
	if !strings.Contains(errPayload.Message, "hex color") {
		t.Fatalf("color payload.Message = %q, want mention of hex color", errPayload.Message)
	}
}

func TestDrawChartTool_Call_ReturnsFormattedResult(t *testing.T) {
	t.Parallel()

	tool := NewDrawChartTool()
	result, err := tool.Call(context.Background(), `{"options":{"chart":{"type":"line"},"title":{"text":"Sales"},"series":[{"name":"Q1","data":[1,2,3]}]}}`)
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if strings.Contains(result, "\"code\": \"INVALID_REQUEST\"") {
		t.Fatalf("expected success output, got: %s", result)
	}
	if !strings.Contains(result, "\"series\"") {
		t.Fatalf("expected result to contain series, got: %s", result)
	}
}

func TestDrawChartTool_CallStructured_EmitsArtifact(t *testing.T) {
	t.Parallel()

	tool := NewDrawChartTool().(*DrawChartTool)
	result, err := tool.CallStructured(context.Background(), `{"options":{"chart":{"type":"line"},"title":{"text":"Monthly Sales"},"series":[{"name":"Q1","data":[1,2,3]}]}}`)
	if err != nil {
		t.Fatalf("CallStructured() error = %v", err)
	}
	if result == nil {
		t.Fatal("CallStructured() returned nil result")
	}
	if len(result.Artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(result.Artifacts))
	}

	artifact := result.Artifacts[0]
	if artifact.Type != "chart" {
		t.Fatalf("artifact.Type = %q, want %q", artifact.Type, "chart")
	}
	if artifact.Name != "Monthly Sales" {
		t.Fatalf("artifact.Name = %q, want %q", artifact.Name, "Monthly Sales")
	}
	if artifact.Metadata == nil || artifact.Metadata["spec"] == nil {
		t.Fatalf("artifact.Metadata.spec should be present")
	}
}

func TestDrawChartTool_CallStructured_DownsamplesDenseSeriesAndEmitsWarning(t *testing.T) {
	t.Parallel()

	data := make([]any, 0, 1200)
	for i := 0; i < 1200; i++ {
		data = append(data, i+1)
	}

	inputMap := map[string]any{
		"options": map[string]any{
			"chart": map[string]any{
				"type": "line",
			},
			"title": map[string]any{
				"text": "Dense Series",
			},
			"series": []any{
				map[string]any{
					"name": "Revenue",
					"data": data,
				},
			},
		},
	}
	inputBytes, err := json.Marshal(inputMap)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	tool := NewDrawChartTool().(*DrawChartTool)
	result, err := tool.CallStructured(context.Background(), string(inputBytes))
	if err != nil {
		t.Fatalf("CallStructured() error = %v", err)
	}

	payload, ok := result.Payload.(types.JSONPayload)
	if !ok {
		t.Fatalf("payload type = %T, want JSONPayload", result.Payload)
	}
	options, ok := payload.Output.(map[string]any)
	if !ok {
		t.Fatalf("payload.Output type = %T, want map[string]any", payload.Output)
	}
	series, ok := options["series"].([]any)
	if !ok || len(series) == 0 {
		t.Fatalf("options.series type = %T, want non-empty []any", options["series"])
	}
	seriesMap, ok := series[0].(map[string]any)
	if !ok {
		t.Fatalf("series[0] type = %T, want map[string]any", series[0])
	}
	downsampled, ok := seriesMap["data"].([]any)
	if !ok {
		t.Fatalf("series[0].data type = %T, want []any", seriesMap["data"])
	}
	if len(downsampled) != downsampleTargetPoints {
		t.Fatalf("downsampled len = %d, want %d", len(downsampled), downsampleTargetPoints)
	}

	if len(result.Artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(result.Artifacts))
	}
	warningsRaw, ok := result.Artifacts[0].Metadata["warnings"]
	if !ok {
		t.Fatalf("artifact metadata should include warnings")
	}
	warnings, ok := warningsRaw.([]string)
	if !ok {
		t.Fatalf("warnings type = %T, want []string", warningsRaw)
	}
	if len(warnings) == 0 {
		t.Fatalf("expected downsampling warning")
	}
	if !strings.Contains(strings.ToLower(strings.Join(warnings, " ")), "downsampled") {
		t.Fatalf("warnings = %v, want downsampled hint", warnings)
	}
}

func TestDrawChartTool_CallStructured_InferDatetimeAxisAndNormalizeValues(t *testing.T) {
	t.Parallel()

	tool := NewDrawChartTool().(*DrawChartTool)
	result, err := tool.CallStructured(
		context.Background(),
		`{"options":{"chart":{"type":"line"},"series":[{"name":"Revenue","data":[{"x":"2025-01-01","y":1200},{"x":"2025-02-01","y":980},{"x":"2025-03-01","y":1540}]}]}}`,
	)
	if err != nil {
		t.Fatalf("CallStructured() error = %v", err)
	}

	payload, ok := result.Payload.(types.JSONPayload)
	if !ok {
		t.Fatalf("payload type = %T, want JSONPayload", result.Payload)
	}
	options, ok := payload.Output.(map[string]any)
	if !ok {
		t.Fatalf("payload.Output type = %T, want map[string]any", payload.Output)
	}
	xaxis, ok := options["xaxis"].(map[string]any)
	if !ok {
		t.Fatalf("options.xaxis type = %T, want map[string]any", options["xaxis"])
	}
	if xaxis["type"] != "datetime" {
		t.Fatalf("xaxis.type = %v, want datetime", xaxis["type"])
	}

	series, _ := options["series"].([]any)
	seriesMap, _ := series[0].(map[string]any)
	points, _ := seriesMap["data"].([]any)
	firstPoint, _ := points[0].(map[string]any)
	if _, ok := toFloat(firstPoint["x"]); !ok {
		t.Fatalf("expected normalized numeric x, got %T (%v)", firstPoint["x"], firstPoint["x"])
	}
}

func TestDrawChartTool_CallStructured_DoesNotInferDatetimeFromNumericCategories(t *testing.T) {
	t.Parallel()

	tool := NewDrawChartTool().(*DrawChartTool)
	result, err := tool.CallStructured(
		context.Background(),
		`{"options":{"chart":{"type":"line"},"xaxis":{"categories":["1","2","3"]},"series":[{"name":"Revenue","data":[1200,980,1540]}]}}`,
	)
	if err != nil {
		t.Fatalf("CallStructured() error = %v", err)
	}

	payload, ok := result.Payload.(types.JSONPayload)
	if !ok {
		t.Fatalf("payload type = %T, want JSONPayload", result.Payload)
	}
	options, ok := payload.Output.(map[string]any)
	if !ok {
		t.Fatalf("payload.Output type = %T, want map[string]any", payload.Output)
	}
	xaxis, ok := options["xaxis"].(map[string]any)
	if !ok {
		t.Fatalf("options.xaxis type = %T, want map[string]any", options["xaxis"])
	}
	if _, hasType := xaxis["type"]; hasType {
		t.Fatalf("xaxis.type should not be inferred for numeric labels, got %v", xaxis["type"])
	}
}
