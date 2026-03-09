package format

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestApplyParsesNumericStrings(t *testing.T) {
	t.Parallel()

	count := Count()
	money := MoneyCompact("UZS")
	percent := Percent(1)

	require.Equal(t, "42", Apply(&count, "42", "", ""))
	require.Equal(t, "12.50K UZS", Apply(&money, "12500", "", ""))
	require.Equal(t, "7.5%", Apply(&percent, "7.5", "", ""))
}

func TestApplyFallsBackForInvalidNumericStrings(t *testing.T) {
	t.Parallel()

	count := Count()
	require.Equal(t, "nope", Apply(&count, "nope", "", ""))
}

func TestApplyFormatsDatesInTimezone(t *testing.T) {
	t.Parallel()

	spec := Date("2006-01-02 15:04")
	value := time.Date(2026, time.March, 9, 0, 30, 0, 0, time.UTC)

	require.Equal(t, "2026-03-09 05:30", Apply(&spec, value, "", "Asia/Tashkent"))
}

func TestApplySupportsMonthLabelDurationAndLocalizedString(t *testing.T) {
	t.Parallel()

	monthSpec := Spec{Kind: KindMonthLabel}
	durationSpec := Spec{Kind: KindDuration}
	localizedSpec := Spec{Kind: KindLocalizedString, Dictionary: map[string]string{"pending": "Pending"}}

	require.Equal(t, "Jan 2026", Apply(&monthSpec, "2026-01-03", "", ""))
	require.Equal(t, "2m0s", Apply(&durationSpec, 120, "", ""))
	require.Equal(t, "Pending", Apply(&localizedSpec, "pending", "", ""))
}
