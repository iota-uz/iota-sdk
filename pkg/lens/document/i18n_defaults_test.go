package document

import (
	"os"
	"path/filepath"
	"regexp"
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
			strings.HasPrefix(key, "report.editor.") || strings.HasPrefix(key, "report.proposal.") || strings.HasPrefix(key, "report.leave.") ||
			strings.HasPrefix(key, "dashboard.edit") || strings.HasPrefix(key, "dashboard.empty.") || key == I18nFilterMore {
			require.NotEmptyf(t, defaults[key], "default for %s", key)
		}
	}
	require.Contains(t, defaults[I18nReportPickerTitle], "{target}")
	require.Contains(t, defaults[I18nReportPickerAdd], "{count}")
	require.Contains(t, defaults[I18nReportPickerGenerateCreated], "{title}")
	require.Contains(t, defaults[I18nReportPickerOwnershipSharedBy], "{owner}")
	require.Contains(t, defaults[I18nReportDestinationTitle], "{title}")
	require.Contains(t, defaults[I18nReportEditorAdded], "{count}")
	require.Contains(t, defaults[I18nReportEditorImpactCount], "{count}")
	require.Contains(t, defaults[I18nReportEditorMoved], "{position}")
}

func TestRuntimeI18nDefaultsForSkipsLocalesTheRuntimeTranslates(t *testing.T) {
	t.Parallel()
	for _, locale := range []string{"ru", "ru-RU", "ru_RU", "RU", "uz", "uz-Latn", "uz-UZ"} {
		require.Truef(t, HasRuntimeBuiltinCatalogue(locale), locale)
		require.Emptyf(t, RuntimeI18nDefaultsFor(locale), locale)
	}
	for _, locale := range []string{"", "en", "en-US", "zh", "pt-BR", "uz-Cyrl", "uz_cyrl_UZ"} {
		require.Falsef(t, HasRuntimeBuiltinCatalogue(locale), locale)
		require.Equalf(t, RuntimeI18nDefaults(), RuntimeI18nDefaultsFor(locale), locale)
	}
}

// TestRuntimeBuiltinLocalesMatchRuntimeCatalogues keeps the Go list of
// translated languages equal to the catalogues the React runtime registers.
func TestRuntimeBuiltinLocalesMatchRuntimeCatalogues(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile(filepath.Join("..", "..", "..", "web", "lens", "src", "runtime", "messages.ts"))
	if err != nil {
		t.Skipf("runtime sources are unavailable: %v", err)
	}
	match := regexp.MustCompile(`const catalogues[^=]*=\s*\{([^}]*)\}`).FindSubmatch(source)
	require.NotNil(t, match, "messages.ts no longer registers catalogues in one literal")
	registered := make([]string, 0)
	for _, name := range strings.Split(string(match[1]), ",") {
		if name = strings.TrimSpace(name); name != "" {
			registered = append(registered, name)
		}
	}
	require.ElementsMatch(t, registered, RuntimeBuiltinLocales())
}
