// Package metrics renders the KPI band of the IOTA design system: totals of the
// current selection above a table, one card per metric, one row per currency.
//
// Invariants: every card shows the same currencies in the caller's order (a
// missing value is "—"), so the band is one grid and every card has one
// height; the record count is never a card, it lives in the scope line.
package metrics

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/iota-uz/go-i18n/v2/i18n"

	"github.com/iota-uz/iota-sdk/pkg/intl"
)

// Kind decides the sign and the icon of a metric's values.
type Kind string

const (
	KindIn    Kind = "in"
	KindOut   Kind = "out"
	KindNet   Kind = "net"
	KindCount Kind = "count"
	KindPlain Kind = "plain"
)

// Amount is a value in minor units: 12345 with Decimals 2 is 123,45.
type Amount struct {
	Minor    int64
	Decimals int
}

// Money is an amount in hundredths, the common case.
func Money(minor int64) Amount { return Amount{Minor: minor, Decimals: 2} }

// Metric is one card. Values are keyed by currency code; a KindCount metric
// keys its single value by "".
type Metric struct {
	Label  string
	Kind   Kind
	Values map[string]Amount
	Hint   string
}

// Scope describes the selection the totals are computed over.
type Scope struct {
	// Period is a ready label such as "1–30 ноября 2026"; empty hides it.
	Period string
	// Filters is the number of applied filters; zero hides it.
	Filters int
	// Total is the number of records in the unfiltered list.
	Total int64
	// Matched is the number of records left after search; nil when the list
	// is not narrowed by search.
	Matched *int64
	// CountMessageID overrides the plural message for the record count
	// ("Metrics.Records" by default), e.g. "Debts.Count".
	CountMessageID string
}

// State is the band's lifecycle.
type State string

const (
	StateReady State = "ready"
	StateEmpty State = "empty"
)

// Attach places the band: the top band of a table panel or a framed block.
type Attach string

const (
	AttachTable      Attach = "table"
	AttachStandalone Attach = "standalone"
)

// VisibleCurrencies is how many currency rows a card shows before the rest
// fold behind "N more currencies".
const VisibleCurrencies = 3

type Props struct {
	ID    string
	Scope Scope
	Items []Metric
	// Currencies is the one order for the whole band: base currency first.
	// Use Order to build it.
	Currencies []string
	State      State
	Attach     Attach
	// EmptyNote replaces the default hint shown when the selection is empty.
	EmptyNote string
	// ResetAttrs wires "Show all" when the list is narrowed (hx-get etc.);
	// nil hides the link.
	ResetAttrs templ.Attributes
	Class      string
}

// SkeletonProps sizes the placeholder to the band it stands for.
type SkeletonProps struct {
	Items      int
	Currencies int
	Attach     Attach
}

// Order returns one currency order for a band: base first, then the others
// in the given preference order, then any remaining codes alphabetically.
func Order(base string, preferred []string, items ...Metric) []string {
	seen := map[string]bool{}
	present := map[string]bool{}
	for _, item := range items {
		for code := range item.Values {
			if code != "" {
				present[code] = true
			}
		}
	}
	out := []string{}
	add := func(code string) {
		if code != "" && present[code] && !seen[code] {
			seen[code] = true
			out = append(out, code)
		}
	}
	add(base)
	for _, code := range preferred {
		add(code)
	}
	rest := []string{}
	for code := range present {
		if !seen[code] {
			rest = append(rest, code)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

const (
	nbsp  = " "
	minus = "−"
)

// FormatAmount is the full value: digits grouped by spaces, comma decimals,
// no sign (see Sign).
func FormatAmount(a Amount) string {
	minor := a.Minor
	if minor < 0 {
		minor = -minor
	}
	digits := strconv.FormatInt(minor, 10)
	if a.Decimals <= 0 {
		return group(digits)
	}
	for len(digits) <= a.Decimals {
		digits = "0" + digits
	}
	cut := len(digits) - a.Decimals
	return group(digits[:cut]) + "," + digits[cut:]
}

// FormatCount groups an integer count by spaces.
func FormatCount(n int64) string {
	if n < 0 {
		return minus + group(strconv.FormatInt(-n, 10))
	}
	return group(strconv.FormatInt(n, 10))
}

func group(digits string) string {
	var b strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteString(nbsp)
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Sign is the prefix a value of this kind carries: "+" for income, "−" for
// outflow, by sign for a balance, and only a true minus otherwise.
func Sign(kind Kind, minor int64) string {
	switch {
	case minor == 0:
		return ""
	case kind == KindIn:
		return "+"
	case kind == KindOut:
		return minus
	case kind == KindNet && minor > 0:
		return "+"
	case minor < 0:
		return minus
	}
	return ""
}

func valueText(kind Kind, a Amount) string {
	return Sign(kind, a.Minor) + FormatAmount(a)
}

func negative(kind Kind, a Amount) bool { return kind == KindNet && a.Minor < 0 }

func (p Props) visible() []string {
	if len(p.Currencies) <= VisibleCurrencies {
		return p.Currencies
	}
	return p.Currencies[:VisibleCurrencies]
}

func (p Props) hidden() []string {
	if len(p.Currencies) <= VisibleCurrencies {
		return nil
	}
	return p.Currencies[VisibleCurrencies:]
}

func (p Props) attach() Attach { return skeletonAttach(p.Attach) }

func skeletonAttach(a Attach) Attach {
	if a == "" {
		return AttachTable
	}
	return a
}

func (m Metric) value(code string) (Amount, bool) {
	if m.Kind == KindCount {
		code = ""
	}
	a, ok := m.Values[code]
	return a, ok
}

func gridStyle(n int) string {
	if n < 1 {
		n = 1
	}
	return "--mx-n:" + strconv.Itoa(n)
}

func localize(ctx context.Context, id, fallback string, data map[string]any, count any) string {
	if localizer, ok := intl.UseLocalizer(ctx); ok {
		cfg := &i18n.LocalizeConfig{MessageID: id, TemplateData: data}
		if count != nil {
			cfg.PluralCount = count
		}
		if text, err := localizer.Localize(cfg); err == nil && text != "" {
			return text
		}
	}
	return fallback
}

func countLabel(ctx context.Context, s Scope) string {
	id := s.CountMessageID
	if id == "" {
		id = "Metrics.Records"
	}
	n := FormatCount(s.Total)
	return localize(ctx, id, n, map[string]any{"Count": n}, s.Total)
}

func matchedLabel(ctx context.Context, s Scope) string {
	matched, total := FormatCount(*s.Matched), FormatCount(s.Total)
	return localize(ctx, "Metrics.Matched", matched+" / "+total,
		map[string]any{"Matched": matched, "Total": total}, nil)
}

func filtersLabel(ctx context.Context, n int) string {
	return localize(ctx, "Metrics.Filters", strconv.Itoa(n), map[string]any{"Count": n}, n)
}

func moreLabel(ctx context.Context, n int) string {
	return localize(ctx, "Metrics.MoreCurrencies", "+"+strconv.Itoa(n), map[string]any{"Count": n}, n)
}

func text(ctx context.Context, id, fallback string) string {
	return localize(ctx, id, fallback, nil, nil)
}
