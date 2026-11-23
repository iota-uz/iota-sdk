package metrics

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatAmount(t *testing.T) {
	assert.Equal(t, "19 296 256 923,67", FormatAmount(Money(1929625692367)))
	assert.Equal(t, "0,05", FormatAmount(Money(5)))
	assert.Equal(t, "1 000", FormatAmount(Amount{Minor: 1000}))
	assert.Equal(t, "12,50", FormatAmount(Money(-1250)), "the sign is Sign's job")
	assert.Equal(t, "2 751", FormatCount(2751))
}

func TestSign(t *testing.T) {
	assert.Equal(t, "+", Sign(KindIn, 1))
	assert.Equal(t, "−", Sign(KindOut, 1))
	assert.Equal(t, "+", Sign(KindNet, 1))
	assert.Equal(t, "−", Sign(KindNet, -1))
	assert.Equal(t, "", Sign(KindNet, 0))
	assert.Equal(t, "", Sign(KindPlain, 5))
	assert.Equal(t, "−", Sign(KindPlain, -5))
}

func TestOrder(t *testing.T) {
	in := Metric{Values: map[string]Amount{"USD": Money(1), "UZS": Money(1)}}
	out := Metric{Values: map[string]Amount{"RUB": Money(1), "EUR": Money(1), "UZS": Money(1)}}
	assert.Equal(t, []string{"UZS", "USD", "EUR", "RUB"}, Order("UZS", []string{"USD"}, in, out))
	assert.Equal(t, []string{"EUR", "RUB", "USD", "UZS"}, Order("", nil, in, out))
	assert.Equal(t, []string{"USD", "UZS"}, Order("KZT", nil, in), "a base currency without values is skipped")
}

func render(t *testing.T, p Props) string {
	t.Helper()
	var b strings.Builder
	require.NoError(t, Metrics(p).Render(context.Background(), &b))
	return b.String()
}

func TestMetricsEveryCardHasEveryCurrency(t *testing.T) {
	in := Metric{Label: "Поступления", Kind: KindIn, Values: map[string]Amount{"UZS": Money(100), "USD": Money(200)}}
	out := Metric{Label: "Списания", Kind: KindOut, Values: map[string]Amount{"UZS": Money(50), "RUB": Money(10)}}
	net := Metric{Label: "Сальдо", Kind: KindNet, Values: map[string]Amount{"UZS": Money(-300)}}
	html := render(t, Props{
		Scope:      Scope{Total: 2751},
		Items:      []Metric{in, out, net},
		Currencies: Order("UZS", nil, in, out, net),
	})
	assert.Equal(t, 3, strings.Count(html, `class="iota-mx-card`))
	assert.Contains(t, html, "--mx-n:3")
	// Every card carries every currency row: USD and RUB, filled or "—".
	assert.Equal(t, 3, strings.Count(html, "<dt>USD</dt>"))
	assert.Equal(t, 3, strings.Count(html, "<dt>RUB</dt>"))
	assert.Equal(t, 4, strings.Count(html, ">—</span>"))
	assert.Contains(t, html, "+1,00")
	assert.Contains(t, html, "−0,50")
	assert.Contains(t, html, "iota-mx-v--neg", "a negative balance is marked")
	assert.Contains(t, html, "2 751", "the record count lives in the scope line")
	assert.NotContains(t, html, "x-data", "no fold below four currencies")
}

func TestMetricsFoldsExtraCurrencies(t *testing.T) {
	m := Metric{Label: "Сумма", Kind: KindPlain, Values: map[string]Amount{
		"UZS": Money(1), "USD": Money(1), "EUR": Money(1), "RUB": Money(1), "KZT": Money(1)}}
	html := render(t, Props{Items: []Metric{m}, Currencies: Order("UZS", nil, m)})
	assert.Contains(t, html, `x-data="{ all: false }"`)
	assert.Equal(t, 2, strings.Count(html, `x-show="all"`)-1, "two folded rows plus the toggle label")
	assert.Contains(t, html, "<dt>RUB</dt>", "folded values stay in the markup, never dropped")
}

func TestMetricsNarrowedAndEmpty(t *testing.T) {
	matched := int64(12)
	html := render(t, Props{
		Scope:      Scope{Total: 2751, Matched: &matched, Filters: 2, Period: "Ноябрь 2026"},
		Items:      []Metric{{Label: "Сумма", Kind: KindPlain, Values: map[string]Amount{}}},
		State:      StateEmpty,
		EmptyNote:  "Нет операций",
		ResetAttrs: map[string]any{"hx-get": "/list"},
	})
	assert.Contains(t, html, `hx-get="/list"`)
	assert.Contains(t, html, "Ноябрь 2026")
	assert.Contains(t, html, "Нет операций")
	assert.Contains(t, html, "iota-mx-card--empty")
}

func TestSkeletonMatchesBand(t *testing.T) {
	var b strings.Builder
	require.NoError(t, Skeleton(SkeletonProps{Items: 3, Currencies: 2}).Render(context.Background(), &b))
	html := b.String()
	assert.Equal(t, 3, strings.Count(html, `class="iota-mx-card"`))
	assert.Equal(t, 3, strings.Count(html, "iota-mx-sk--row"))
	assert.Contains(t, html, `aria-busy="true"`)
}

func TestFormatMinInt64(t *testing.T) {
	assert.Equal(t, "−9 223 372 036 854 775 808", FormatCount(-9223372036854775808))
	assert.Equal(t, "92 233 720 368 547 758,08", FormatAmount(Money(-9223372036854775808)))
}

func TestFallbackLabels(t *testing.T) {
	html := render(t, Props{Scope: Scope{Total: 2}, ResetAttrs: map[string]any{"hx-get": "/x"}})
	assert.Contains(t, html, "Show all", "reset stays labelled without a localizer")
	assert.Contains(t, html, "Totals for selection")
}

func TestSkeletonReservesCurrencyToggle(t *testing.T) {
	var b strings.Builder
	require.NoError(t, Skeleton(SkeletonProps{Items: 1, Currencies: 4}).Render(context.Background(), &b))
	assert.Contains(t, b.String(), "iota-mx-note")
}
