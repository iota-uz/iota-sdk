package document

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRuntimeI18nDefaultsOwnNewChromeAndPlaceholders(t *testing.T) {
	t.Parallel()
	defaults := RuntimeI18nDefaults()
	for _, key := range []string{
		I18nChartGaugeValue, I18nChartGaugeRange, I18nChartData, I18nChartOpenMark,
		I18nPanelError, I18nPanelTrendNew, I18nPanelTrendNotAvailable,
		I18nViewsConfirmDelete, I18nViewsConfirmDeleteSched, I18nViewsNextRun,
		I18nViewsScheduleDisabled, I18nViewsScheduleReenable, I18nViewsReenable,
		I18nViewsScheduleReenabled, I18nViewsScheduleReenableErr,
		I18nTableBooleanYes, I18nTableBooleanNo, I18nTableScrollLeft, I18nTableScrollRight,
		I18nChartBoxplotMin, I18nChartBoxplotQ1, I18nChartBoxplotMedian, I18nChartBoxplotQ3, I18nChartBoxplotMax,
		I18nChartSeriesPrevious, I18nChartSeriesTrend, I18nChartSeriesMovingAverage,
		I18nChartSeriesEstimate, I18nChartSeriesYTD, I18nChartSeriesForecast,
		I18nChartSeriesForecastLower, I18nChartSeriesForecastConfidence,
	} {
		require.NotEmptyf(t, defaults[key], "default for %s", key)
	}
	require.Contains(t, defaults[I18nChartGaugeValue], "{name}")
	require.Contains(t, defaults[I18nChartGaugeRange], "{maximum}")
	require.Contains(t, defaults[I18nChartData], "{name}")
	require.Contains(t, defaults[I18nChartSeriesMovingAverage], "{window}")
	require.Contains(t, defaults[I18nChartSeriesForecastLower], "{forecast}")
	require.Contains(t, defaults[I18nViewsNextRun], "{time}")
	require.Contains(t, defaults[I18nViewsScheduleDisabled], "{count}")
	require.Contains(t, defaults[I18nViewsScheduleReenable], "{name}")
	require.Contains(t, defaults[I18nViewsScheduleReenabled], "{name}")

	defaults[I18nPanelError] = "mutated"
	require.NotEqual(t, "mutated", RuntimeI18nDefaults()[I18nPanelError])
}
