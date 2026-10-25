package table

import (
	"fmt"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const (
	moneyColumnClass = "text-right [&_.flex]:justify-end"
	moneyCellClass   = "text-right"
)

// MoneyAmount is a monetary figure prepared for display in a table cell.
//
// Value is the exact decimal ("1234.50") exposed as data-value, so tests and
// scripts never parse the localized Text, whose group separator may be a
// no-break space.
type MoneyAmount struct {
	Value string
	Text  string
	// Currency is the code printed beside the figure. When empty, the figure is
	// printed with MissingCurrency instead, never with an assumed currency.
	Currency string
	// MissingCurrency labels a figure whose currency is unknown. Leave empty to
	// print the figure without a currency note.
	MissingCurrency string
	TestID          string
	// MissingCurrencyTestID is the data-testid of the missing-currency note.
	MissingCurrencyTestID string
}

// MoneyFromMinor formats an amount held in minor units. The fraction digits
// come from the currency when it is known and default to two; Value is derived
// with integer arithmetic, only Text goes through a float.
func MoneyFromMinor(minor int64, currency string, locale language.Tag) MoneyAmount {
	fraction := 2
	if c := money.GetCurrency(currency); c != nil && c.Fraction >= 0 {
		fraction = c.Fraction
	}
	return MoneyAmount{
		Value:    minorDecimal(minor, fraction),
		Text:     message.NewPrinter(locale).Sprintf("%.*f", fraction, float64(minor)/pow10(fraction)),
		Currency: currency,
	}
}

func pow10(n int) float64 {
	p := 1.0
	for i := 0; i < n; i++ {
		p *= 10
	}
	return p
}

func minorDecimal(minor int64, fraction int) string {
	sign := ""
	abs := uint64(minor)
	if minor < 0 {
		sign, abs = "-", -abs
	}
	if fraction == 0 {
		return fmt.Sprintf("%s%d", sign, abs)
	}
	unit := uint64(pow10(fraction))
	return fmt.Sprintf("%s%d.%0*d", sign, abs/unit, fraction, abs%unit)
}

// MoneyColumn is a right-aligned money column. The sort indicator of a
// sortable header is aligned with it; options may add sorting, priority and so on.
func MoneyColumn(key, label string, opts ...ColumnOpt) TableColumn {
	return Column(key, label, append([]ColumnOpt{WithClass(moneyColumnClass)}, opts...)...)
}

// MoneyCell is a right-aligned cell holding amount, for use under MoneyColumn.
func MoneyCell(amount MoneyAmount, opts ...CellOpt) TableCell {
	return Cell(MoneyValue(amount), amount.Value, append([]CellOpt{WithCellClasses(templ.Classes(moneyCellClass))}, opts...)...)
}
