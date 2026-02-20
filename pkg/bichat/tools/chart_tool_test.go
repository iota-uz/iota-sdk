package tools

import (
	"context"
	"strings"
	"testing"
)

func TestDrawChartTool_Name(t *testing.T) {
	t.Parallel()

	tool := NewDrawChartTool()
	if got := tool.Name(); got != "draw_chart" {
		t.Errorf("Name() = %q, want %q", got, "draw_chart")
	}
}

func TestDrawChartTool_Call(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		wantErr        bool // true if we expect non-nil error (currently all should be false)
		wantErrCode    string
		wantErrContain string
		wantSuccess    bool // true if result should NOT contain "error"
	}{
		// === Valid inputs (expect nil error, no "error" in result) ===
		{
			name:        "valid line chart",
			input:       `{"chartType":"line","title":"Sales","series":[{"name":"Q1","data":[1,2,3]}]}`,
			wantErr:     false,
			wantSuccess: true,
		},
		{
			name:        "valid bar chart with labels",
			input:       `{"chartType":"bar","title":"Revenue","series":[{"name":"2024","data":[100,200,300]}],"labels":["Jan","Feb","Mar"]}`,
			wantErr:     false,
			wantSuccess: true,
		},
		{
			name:        "valid pie chart",
			input:       `{"chartType":"pie","title":"Market Share","series":[{"name":"Products","data":[30,45,25]}]}`,
			wantErr:     false,
			wantSuccess: true,
		},
		{
			name:        "valid donut chart",
			input:       `{"chartType":"donut","title":"Distribution","series":[{"name":"Categories","data":[10,20,30,40]}]}`,
			wantErr:     false,
			wantSuccess: true,
		},
		{
			name:        "valid area chart with colors",
			input:       `{"chartType":"area","title":"Trends","series":[{"name":"Data","data":[1.5,2.3,4.7]}],"colors":["#FF0000","#00FF00"]}`,
			wantErr:     false,
			wantSuccess: true,
		},
		{
			name:        "valid chart with custom height",
			input:       `{"chartType":"line","title":"Custom Height","series":[{"name":"Test","data":[1,2]}],"height":500}`,
			wantErr:     false,
			wantSuccess: true,
		},
		{
			name:        "default height when omitted",
			input:       `{"chartType":"bar","title":"Default","series":[{"name":"Data","data":[5,10]}]}`,
			wantErr:     false,
			wantSuccess: true,
		},

		// === Missing required fields (expect nil error, result contains INVALID_REQUEST) ===
		{
			name:           "missing chartType",
			input:          `{"title":"Missing Type","series":[{"name":"Data","data":[1,2,3]}]}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "chartType",
			wantSuccess:    false,
		},
		{
			name:           "missing title",
			input:          `{"chartType":"line","series":[{"name":"Data","data":[1,2,3]}]}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "title",
			wantSuccess:    false,
		},
		{
			name:           "missing series",
			input:          `{"chartType":"bar","title":"No Series"}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "series",
			wantSuccess:    false,
		},
		{
			name:           "empty series array",
			input:          `{"chartType":"line","title":"Empty Series","series":[]}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "series",
			wantSuccess:    false,
		},
		{
			name:           "empty string chartType",
			input:          `{"chartType":"","title":"Empty Type","series":[{"name":"Data","data":[1,2,3]}]}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "chartType",
			wantSuccess:    false,
		},
		{
			name:           "empty string title",
			input:          `{"chartType":"line","title":"","series":[{"name":"Data","data":[1,2,3]}]}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "title",
			wantSuccess:    false,
		},

		// === Invalid chart type ===
		{
			name:           "invalid chart type",
			input:          `{"chartType":"scatter","title":"Invalid Type","series":[{"name":"Data","data":[1,2,3]}]}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "unsupported chart type",
			wantSuccess:    false,
		},

		// === Data validation ===
		{
			name:           "non-numeric data string",
			input:          `{"chartType":"line","title":"Invalid Data","series":[{"name":"Data","data":["a","b","c"]}]}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "not a number",
			wantSuccess:    false,
		},
		{
			name:           "NaN value in data",
			input:          `{"chartType":"bar","title":"NaN Data","series":[{"name":"Data","data":[1.0,"NaN",3.0]}]}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "not a number",
			wantSuccess:    false,
		},
		{
			name:           "null in data",
			input:          `{"chartType":"bar","title":"Null Data","series":[{"name":"Data","data":[1,null,3]}]}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "not a number",
			wantSuccess:    false,
		},
		{
			name:           "boolean in data",
			input:          `{"chartType":"line","title":"Boolean Data","series":[{"name":"Data","data":[true,false]}]}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "not a number",
			wantSuccess:    false,
		},
		{
			name:           "empty data array",
			input:          `{"chartType":"bar","title":"Empty Data","series":[{"name":"Data","data":[]}]}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "no data points",
			wantSuccess:    false,
		},
		{
			name:           "mixed valid and invalid data",
			input:          `{"chartType":"line","title":"Mixed Data","series":[{"name":"Data","data":[1,2,"invalid",4]}]}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "not a number",
			wantSuccess:    false,
		},
		{
			name:        "floating point numbers",
			input:       `{"chartType":"area","title":"Float Data","series":[{"name":"Data","data":[1.5,2.7,3.14,4.0]}]}`,
			wantErr:     false,
			wantSuccess: true,
		},
		{
			name:        "negative numbers in line chart",
			input:       `{"chartType":"line","title":"Negative Line","series":[{"name":"Data","data":[-10,-5,0,5,10]}]}`,
			wantErr:     false,
			wantSuccess: true,
		},
		{
			name:        "large numbers",
			input:       `{"chartType":"bar","title":"Large Numbers","series":[{"name":"Data","data":[1000000,2000000,3000000]}]}`,
			wantErr:     false,
			wantSuccess: true,
		},
		{
			name: "exceeds max data points",
			// Generate 1001 data points
			input: func() string {
				data := `{"chartType":"line","title":"Too Many Points","series":[{"name":"Data","data":[`
				for i := 0; i < 1001; i++ {
					if i > 0 {
						data += ","
					}
					data += "1"
				}
				data += `]}]}`
				return data
			}(),
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "exceeds maximum 1000 data points",
			wantSuccess:    false,
		},

		// === Pie/donut specific ===
		{
			name:           "pie with multiple series",
			input:          `{"chartType":"pie","title":"Multi Series","series":[{"name":"S1","data":[1,2]},{"name":"S2","data":[3,4]}]}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "exactly one series",
			wantSuccess:    false,
		},
		{
			name:           "donut with multiple series",
			input:          `{"chartType":"donut","title":"Multi Series","series":[{"name":"S1","data":[5,10]},{"name":"S2","data":[15,20]}]}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "exactly one series",
			wantSuccess:    false,
		},
		{
			name:           "pie with negative values",
			input:          `{"chartType":"pie","title":"Negative Pie","series":[{"name":"Data","data":[-1,2,3]}]}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "negative",
			wantSuccess:    false,
		},
		{
			name:           "donut with negative values",
			input:          `{"chartType":"donut","title":"Negative Donut","series":[{"name":"Data","data":[-5,10]}]}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "negative",
			wantSuccess:    false,
		},
		{
			name:        "pie with zero values",
			input:       `{"chartType":"pie","title":"Zero Pie","series":[{"name":"Data","data":[0,5,10]}]}`,
			wantErr:     false,
			wantSuccess: true,
		},

		// === Labels validation ===
		{
			name:           "labels count mismatch line",
			input:          `{"chartType":"line","title":"Label Mismatch","series":[{"name":"Data","data":[1,2,3]}],"labels":["A","B"]}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "labels count",
			wantSuccess:    false,
		},
		{
			name:           "labels count mismatch pie",
			input:          `{"chartType":"pie","title":"Pie Label Mismatch","series":[{"name":"Data","data":[1,2,3]}],"labels":["A","B"]}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "labels count",
			wantSuccess:    false,
		},
		{
			name:        "labels matching is ok",
			input:       `{"chartType":"bar","title":"Label Match","series":[{"name":"Data","data":[10,20,30]}],"labels":["A","B","C"]}`,
			wantErr:     false,
			wantSuccess: true,
		},

		// === Cross-series validation ===
		{
			name:           "series length mismatch",
			input:          `{"chartType":"line","title":"Series Mismatch","series":[{"name":"S1","data":[1,2,3]},{"name":"S2","data":[4,5]}]}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "data points",
			wantSuccess:    false,
		},
		{
			name:           "three series length mismatch",
			input:          `{"chartType":"bar","title":"Three Series","series":[{"name":"S1","data":[1,2]},{"name":"S2","data":[3,4]},{"name":"S3","data":[5,6,7]}]}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "data points",
			wantSuccess:    false,
		},
		{
			name:        "series length match",
			input:       `{"chartType":"bar","title":"Series Match","series":[{"name":"S1","data":[1,2,3]},{"name":"S2","data":[4,5,6]}]}`,
			wantErr:     false,
			wantSuccess: true,
		},

		// === Color validation ===
		{
			name:           "invalid hex color",
			input:          `{"chartType":"line","title":"Bad Color","series":[{"name":"Data","data":[1,2]}],"colors":["red"]}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "not a valid hex color",
			wantSuccess:    false,
		},
		{
			name:        "valid hex colors",
			input:       `{"chartType":"bar","title":"Good Colors","series":[{"name":"Data","data":[1,2]}],"colors":["#FFF","#AABBCC"]}`,
			wantErr:     false,
			wantSuccess: true,
		},
		{
			name:        "valid hex colors with alpha",
			input:       `{"chartType":"area","title":"Alpha Colors","series":[{"name":"Data","data":[1,2]}],"colors":["#FF0000AA","#00FF00FF"]}`,
			wantErr:     false,
			wantSuccess: true,
		},
		{
			name:        "valid short hex colors",
			input:       `{"chartType":"line","title":"Short Hex","series":[{"name":"Data","data":[1,2]}],"colors":["#F00","#0F0","#00F"]}`,
			wantErr:     false,
			wantSuccess: true,
		},
		{
			name:           "hex without hash",
			input:          `{"chartType":"bar","title":"No Hash","series":[{"name":"Data","data":[1,2]}],"colors":["FF0000"]}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "not a valid hex color",
			wantSuccess:    false,
		},
		{
			name:           "invalid hex characters",
			input:          `{"chartType":"line","title":"Bad Hex","series":[{"name":"Data","data":[1,2]}],"colors":["#GGHHII"]}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "not a valid hex color",
			wantSuccess:    false,
		},

		// === Height validation ===
		{
			name:           "height too small",
			input:          `{"chartType":"line","title":"Small Height","series":[{"name":"Data","data":[1,2]}],"height":50}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "height must be between 100-1000",
			wantSuccess:    false,
		},
		{
			name:           "height too large",
			input:          `{"chartType":"bar","title":"Large Height","series":[{"name":"Data","data":[1,2]}],"height":2000}`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "height must be between 100-1000",
			wantSuccess:    false,
		},
		{
			name:        "height at min boundary",
			input:       `{"chartType":"line","title":"Min Height","series":[{"name":"Data","data":[1,2]}],"height":100}`,
			wantErr:     false,
			wantSuccess: true,
		},
		{
			name:        "height at max boundary",
			input:       `{"chartType":"area","title":"Max Height","series":[{"name":"Data","data":[1,2]}],"height":1000}`,
			wantErr:     false,
			wantSuccess: true,
		},

		// === Malformed input ===
		{
			name:           "invalid JSON",
			input:          `not json`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "failed to parse input",
			wantSuccess:    false,
		},
		{
			name:           "completely empty input",
			input:          ``,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "failed to parse input",
			wantSuccess:    false,
		},
		{
			name:           "malformed JSON object",
			input:          `{"chartType":"line","title":"Test"`,
			wantErr:        false,
			wantErrCode:    "INVALID_REQUEST",
			wantErrContain: "failed to parse input",
			wantSuccess:    false,
		},

		// === Edge cases ===
		{
			name:        "single data point",
			input:       `{"chartType":"bar","title":"Single Point","series":[{"name":"Data","data":[42]}]}`,
			wantErr:     false,
			wantSuccess: true,
		},
		{
			name:        "many series",
			input:       `{"chartType":"line","title":"Many Series","series":[{"name":"S1","data":[1,2]},{"name":"S2","data":[3,4]},{"name":"S3","data":[5,6]},{"name":"S4","data":[7,8]}]}`,
			wantErr:     false,
			wantSuccess: true,
		},
		{
			name:        "zero values",
			input:       `{"chartType":"bar","title":"Zeros","series":[{"name":"Data","data":[0,0,0]}]}`,
			wantErr:     false,
			wantSuccess: true,
		},
		{
			name:        "mixed positive and negative",
			input:       `{"chartType":"area","title":"Mixed Signs","series":[{"name":"Data","data":[-10,5,-3,8,0]}]}`,
			wantErr:     false,
			wantSuccess: true,
		},
		{
			name:        "donut with single value",
			input:       `{"chartType":"donut","title":"Single Donut","series":[{"name":"Data","data":[100]}]}`,
			wantErr:     false,
			wantSuccess: true,
		},
		{
			name: "exact 1000 data points",
			input: func() string {
				data := `{"chartType":"line","title":"Exactly 1000","series":[{"name":"Data","data":[`
				for i := 0; i < 1000; i++ {
					if i > 0 {
						data += ","
					}
					data += "1"
				}
				data += `]}]}`
				return data
			}(),
			wantErr:     false,
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tool := NewDrawChartTool()
			result, err := tool.Call(context.Background(), tt.input)

			// Check error expectation
			if tt.wantErr && err == nil {
				t.Error("expected non-nil error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected nil error, got: %v", err)
			}

			// For success cases, verify result does NOT contain "error"
			if tt.wantSuccess {
				if strings.Contains(result, `"error"`) {
					t.Errorf("expected success but result contains error: %s", result)
				}
				// Also verify it contains the input title and chartType
				// (these should be in the returned JSON spec)
				return
			}

			// For error cases, verify error code and message
			if tt.wantErrCode != "" {
				if !strings.Contains(result, tt.wantErrCode) {
					t.Errorf("expected error code %q in result, got: %s", tt.wantErrCode, result)
				}
			}

			if tt.wantErrContain != "" {
				if !strings.Contains(result, tt.wantErrContain) {
					t.Errorf("expected result to contain %q, got: %s", tt.wantErrContain, result)
				}
			}
		})
	}
}

func TestDrawChartTool_CallStructured_EmitsArtifact(t *testing.T) {
	t.Parallel()

	tool := NewDrawChartTool().(*DrawChartTool)
	result, err := tool.CallStructured(context.Background(), `{"chartType":"line","title":"Monthly Sales","series":[{"name":"Q1","data":[1,2,3]}]}`)
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
}
