package document

import (
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
