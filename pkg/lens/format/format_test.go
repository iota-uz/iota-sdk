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
		{name: "money_compact_below_floor", spec: MoneyCompact("UZS"), input: "12500", expected: "12 500 UZS"},
		{name: "money_compact_thousands", spec: MoneyCompact("UZS"), input: "125000", expected: "125.00K UZS"},
		{name: "money_compact_trillions", spec: MoneyCompact("UZS"), input: "1417670000000", expected: "1.42T UZS"},
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

func TestApplyFormatsAbbreviatedMoneyByLocale(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		locale   string
		input    any
		expected string
	}{
		{name: "en_default_below_floor", locale: "", input: 12500.0, expected: "12 500 UZS"},
		{name: "en_explicit_below_floor", locale: "en-US", input: 12500.0, expected: "12,500 UZS"},
		{name: "en_thousand", locale: "en-US", input: 125_000.0, expected: "125.00K UZS"},
		{name: "ru_below_floor", locale: "ru", input: 12500.0, expected: "12 500 UZS"},
		{name: "ru_below_floor_negative", locale: "ru", input: -12500.4, expected: "-12 500 UZS"},
		{name: "ru_thousand", locale: "ru", input: 125_000.0, expected: "125.00 тыс UZS"},
		{name: "ru_million", locale: "ru", input: 3_400_000.0, expected: "3.40 млн UZS"},
		{name: "uz_thousand", locale: "uz", input: 125_000.0, expected: "125.00 ming UZS"},
		{name: "uz_billion", locale: "uz", input: 2_100_000_000.0, expected: "2.10 mlrd UZS"},
		{name: "uz_cyrl_thousand", locale: "uz-Cyrl", input: 125_000.0, expected: "125.00 минг UZS"},
		{name: "uz_cyrl_trillion", locale: "uz-Cyrl", input: 1_417_670_000_000.0, expected: "1.42 трлн UZS"},
	}

	spec := MoneyCompact("UZS")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, Apply(&spec, tc.input, tc.locale, ""))
		})
	}
}

func TestMoneyExact(t *testing.T) {
	t.Parallel()

	require.Equal(t, "66 064 767 694 UZS", MoneyExact(66_064_767_693.59, "UZS", "ru"))
	require.Equal(t, "66,064,767,694 UZS", MoneyExact(66_064_767_693.59, "UZS", "en-US"))
	require.Equal(t, "-9 533 816 944 UZS", MoneyExact(-9_533_816_944.2, "UZS", "uz"))
	require.Equal(t, "1 250", MoneyExact(1250, "", "ru"))
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
