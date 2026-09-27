package document

import (
	"math"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/stretchr/testify/require"
)

func TestBuildTemporalCarriesEveryOverlayAndExtendsTargetSpec(t *testing.T) {
	t.Parallel()

	spec := panel.TimeSeries("trend", "Trend", "daily").
		Target(100, "Break-even").
		Regression(panel.FieldRef("regression"), "Trend").
		MovingAverage(7, panel.FieldRef("sma_7"), "SMA 7").
		ReferenceLine(80, "Plan").
		IncompletePeriod("2026", panel.TemporalPeriodYTD, "YTD", panel.FieldRef("annualized")).
		AnnotateTime("2026-03-01", "Launch").
		Forecast("2026-08-01", panel.FieldRef("forecast"), panel.FieldRef("lower"), panel.FieldRef("upper"), "Forecast").
		Build()

	temporal := buildTemporal(spec)
	require.NotNil(t, temporal)
	require.Equal(t, "regression", temporal.Regression.Field)
	require.Equal(t, 7, temporal.MovingAverages[0].Window)
	require.Equal(t, []PanelTarget{{Value: 100, Label: "Break-even"}, {Value: 80, Label: "Plan"}}, temporal.ReferenceLines)
	require.Equal(t, TemporalPeriodYTD, temporal.Period.State)
	require.Equal(t, "Launch", temporal.Annotations[0].Label)
	require.Equal(t, "upper", temporal.Forecast.UpperField)
}

func TestValidateTemporalPanelRejectsInvalidOverlayContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		kind     PanelKind
		temporal *PanelTemporal
		want     string
	}{
		{"unsupported panel", PanelKindBar, &PanelTemporal{Regression: &TemporalSeries{Field: "trend"}}, "only supported on line and area"},
		{"missing regression field", PanelKindLine, &PanelTemporal{Regression: &TemporalSeries{}}, "regression field is required"},
		{"invalid moving average", PanelKindLine, &PanelTemporal{MovingAverages: []TemporalMovingAverage{{Window: 5, Field: "sma"}}}, "must be 3, 7, or 12"},
		{"duplicate moving average", PanelKindLine, &PanelTemporal{MovingAverages: []TemporalMovingAverage{{Window: 3, Field: "a"}, {Window: 3, Field: "b"}}}, "duplicate moving-average"},
		{"non-finite reference", PanelKindLine, &PanelTemporal{ReferenceLines: []PanelTarget{{Value: math.Inf(1)}}}, "must be finite"},
		{"invalid period", PanelKindLine, &PanelTemporal{Period: &TemporalPeriod{Category: "2026", State: "partial"}}, "unsupported incomplete-period state"},
		{"empty annotation", PanelKindLine, &PanelTemporal{Annotations: []TemporalAnnotation{{At: "2026-01-01"}}}, "requires at and label"},
		{"missing forecast start", PanelKindLine, &PanelTemporal{Forecast: &TemporalForecast{ValueField: "v", LowerField: "lo", UpperField: "hi"}}, "forecast start is required"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := validateTemporalPanel(Panel{ID: "trend", Kind: test.kind, Deferred: true, Temporal: test.temporal}, Frame{})
			require.ErrorContains(t, err, test.want)
		})
	}
}

func TestValidateTemporalPanelAllowsIncompletePeriodOnBar(t *testing.T) {
	t.Parallel()
	require.NoError(t, validateTemporalPanel(Panel{
		ID: "premium", Kind: PanelKindBar, Deferred: true,
		Temporal: &PanelTemporal{Period: &TemporalPeriod{Category: "2026 · Q3", State: TemporalPeriodYTD}},
	}, Frame{}))
}
