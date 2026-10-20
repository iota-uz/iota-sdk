package document

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRuntimeI18nDefaultsOwnNewChromeAndPlaceholders(t *testing.T) {
	t.Parallel()
	defaults := RuntimeI18nDefaults()
	for _, key := range []string{
		I18nChartGaugeValue, I18nChartGaugeRange, I18nChartData, I18nChartOpenMark,
		I18nPanelError, I18nPanelTrendNew, I18nPanelTrendNotAvailable,
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
	defaults[I18nPanelError] = "mutated"
	require.NotEqual(t, "mutated", RuntimeI18nDefaults()[I18nPanelError])
}

func TestRuntimeI18nDefaultsCoverReportPickerAndEditor(t *testing.T) {
	t.Parallel()
	defaults := RuntimeI18nDefaults()
	for _, key := range RuntimeI18nKeys() {
		if strings.HasPrefix(key, "report.picker.") || strings.HasPrefix(key, "report.destination.") ||
			strings.HasPrefix(key, "report.editor.") || strings.HasPrefix(key, "report.proposal.") || strings.HasPrefix(key, "report.leave.") {
			require.NotEmptyf(t, defaults[key], "default for %s", key)
		}
	}
	require.Contains(t, defaults[I18nReportPickerTitle], "{target}")
	require.Contains(t, defaults[I18nReportPickerAdd], "{count}")
	require.Contains(t, defaults[I18nReportPickerOwnershipSharedBy], "{owner}")
	require.Contains(t, defaults[I18nReportDestinationTitle], "{title}")
	require.Contains(t, defaults[I18nReportEditorAdded], "{count}")
	require.Contains(t, defaults[I18nReportEditorImpactCount], "{count}")
	require.Contains(t, defaults[I18nReportEditorMoved], "{position}")
}
