package format

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyParsesNumericStrings_Scenarios(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		spec     Spec
		input    any
		expected string
	}{
		{name: "count", spec: Count(), input: "42", expected: "42"},
		{name: "money", spec: Money("UZS", 0), input: "160000", expected: "160 000 so\u2019m"},
		{name: "money_compact", spec: MoneyCompact("UZS"), input: "12500", expected: "12.50K UZS"},
		{name: "percent", spec: Percent(1), input: "7.5", expected: "7.5%"},
		{name: "invalid_count_string", spec: Count(), input: "abc", expected: "abc"},
		{name: "empty_count_string", spec: Count(), input: "", expected: ""},
		{name: "unsupported_kind", spec: Spec{Kind: Kind("unsupported")}, input: "42", expected: "42"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, Apply(&tc.spec, tc.input, "", ""))
		})
	}
}

func TestApplyFormatsDatesInTimezone(t *testing.T) {
	t.Parallel()

	spec := Date("2006-01-02 15:04")
	value := time.Date(2026, time.March, 9, 0, 30, 0, 0, time.UTC)

	require.Equal(t, "2026-03-09 05:30", Apply(&spec, value, "", "Asia/Tashkent"))
}

func TestApplyFormatsMoneyWithLocaleSeparators(t *testing.T) {
	t.Parallel()

	require.Equal(t, "160 000 so\u2019m", Apply(&Spec{Kind: KindMoney, Currency: "UZS", Precision: 0}, 160000.0, "ru", ""))
	require.Equal(t, "160,000 so\u2019m", Apply(&Spec{Kind: KindMoney, Currency: "UZS", Precision: 0}, 160000.0, "en-US", ""))
	require.Equal(t, "160 000.50 so\u2019m", Apply(&Spec{Kind: KindMoney, Currency: "UZS", Precision: 2}, 160000.5, "ru", ""))
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

func TestApplyFormatsNilAsEmptyString(t *testing.T) {
	t.Parallel()

	require.Empty(t, Apply(nil, nil, "", ""))
	require.Empty(t, Apply(&Spec{Kind: KindInteger}, nil, "", ""))
}
