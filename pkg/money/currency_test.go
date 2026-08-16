package money

import (
	"reflect"
	"testing"
)

func TestCurrency_Get(t *testing.T) {
	tcs := []struct {
		code     string
		expected string
	}{
		{EUR, "EUR"},
		{"EUR", "EUR"},
		{"Eur", "EUR"},
	}

	for _, tc := range tcs {
		c := newCurrency(tc.code).get()

		if c.Code != tc.expected {
			t.Errorf("Expected %s got %s", tc.expected, c.Code)
		}
	}
}

func TestCurrency_Get1(t *testing.T) {
	code := "RANDOM"
	c := newCurrency(code).get()

	if c.Grapheme != code {
		t.Errorf("Expected %s got %s", c.Grapheme, code)
	}
}

func TestCurrency_Equals(t *testing.T) {
	tcs := []struct {
		code  string
		other string
	}{
		{EUR, "EUR"},
		{"EUR", "EUR"},
		{"Eur", "EUR"},
		{"usd", "USD"},
	}

	for _, tc := range tcs {
		c := newCurrency(tc.code).get()
		oc := newCurrency(tc.other).get()

		if !c.equals(oc) {
			t.Errorf("Expected that %v is not equal %v", c, oc)
		}
	}
}

func TestCurrency_AddCurrency(t *testing.T) {
	tcs := []struct {
		code     string
		template string
	}{
		{"GOLD", "1$"},
	}

	for _, tc := range tcs {
		AddCurrency(tc.code, "", tc.template, "", "", 0)
		c := newCurrency(tc.code).get()

		if c.Template != tc.template {
			t.Errorf("Expected currency template %v got %v", tc.template, c.Template)
		}
	}
}

func TestCurrency_GetCurrency(t *testing.T) {
	code := "KLINGONDOLLAR"
	desired := Currency{Decimal: ".", Thousand: ",", Code: code, Fraction: 2, Grapheme: "$", Template: "$1"}
	AddCurrency(desired.Code, desired.Grapheme, desired.Template, desired.Decimal, desired.Thousand, desired.Fraction)
	currency := GetCurrency(code)
	if !reflect.DeepEqual(currency, &desired) {
		t.Errorf("Currencies do not match %+v got %+v", desired, currency)
	}
}

func TestCurrency_GetNonExistingCurrency(t *testing.T) {
	currency := GetCurrency("I*am*Not*a*Currency")
	if currency != nil {
		t.Errorf("Unexpected currency returned %+v", currency)
	}
}

func TestCurrencies(t *testing.T) {
	const currencyFooCode = "FOO"
	const currencyFooNumericCode = "1234"
	curFoo := &Currency{
		Code:        currencyFooCode,
		NumericCode: currencyFooNumericCode,
		Fraction:    10,
		Grapheme:    "1",
		Template:    "2",
		Decimal:     "3",
		Thousand:    "4",
	}
	var cs = Currencies{
		currencyFooCode: curFoo,
	}
	const currencyBarCode = "BAR"
	const currencyBarNumericCode = "4321"
	curBar := &Currency{
		Code:        currencyBarCode,
		NumericCode: currencyBarNumericCode,
		Fraction:    1,
		Grapheme:    "2",
		Template:    "3",
		Decimal:     "4",
		Thousand:    "5",
	}
	cs = cs.Add(curBar)

	ac := cs.CurrencyByCode(currencyFooCode)
	if !curFoo.equals(ac) {
		t.Errorf("unexpected currency returned. expected: %v, got %v", curFoo, ac)
	}

	ac = cs.CurrencyByNumericCode(currencyFooNumericCode)
	if !curFoo.equals(ac) {
		t.Errorf("unexpected currency returned. expected: %v, got %v", curFoo, ac)
	}

	ac = cs.CurrencyByCode(currencyBarCode)
	if !curBar.equals(ac) {
		t.Errorf("unexpected currency returned. expected: %v, got %v", curBar, ac)
	}

	ac = cs.CurrencyByNumericCode(currencyBarNumericCode)
	if !curBar.equals(ac) {
		t.Errorf("unexpected currency returned. expected: %v, got %v", curBar, ac)
	}
}

func TestCurrency_GetCurrencyByNumericCode(t *testing.T) {
	code := "986"
	expected := GetCurrency(BRL)
	got := GetCurrencyByNumericCode(code)

	if !expected.equals(got) {
		t.Errorf("unexpected currency returned. expected: %v, got %v", expected, got)
	}
}

func TestCurrency_GetCurrencyByNumericCodeNonExistingCurrency(t *testing.T) {
	currency := GetCurrencyByNumericCode("I*am*Not*a*Valid*Numeric*Code")
	if currency != nil {
		t.Errorf("Unexpected currency returned %+v", currency)
	}
}

// "840" is declared by USD and by the USDSPACE formatting variant. The lookup
// used to return whichever the map's randomised iteration reached first, so a
// dollar receipt was rendered - and stored - as "USDSPACE" on some runs.
func TestCurrency_GetCurrencyByNumericCodePrefersTheISOCodeOverAVariant(t *testing.T) {
	for i := 0; i < 200; i++ {
		got := GetCurrencyByNumericCode("840")
		if got == nil || got.Code != USD {
			t.Fatalf("iteration %d: expected USD for 840, got %+v", i, got)
		}
	}
}

// "532" is a genuine collision in the standard rather than a variant: XCG
// succeeded ANG and inherited its numeric code. Both are ISO-shaped, so the
// tie-break is the smaller code, and it must answer that one every time.
func TestCurrency_GetCurrencyByNumericCodeIsStableOnACollision(t *testing.T) {
	for i := 0; i < 200; i++ {
		got := GetCurrencyByNumericCode("532")
		if got == nil || got.Code != ANG {
			t.Fatalf("iteration %d: expected %s for 532, got %+v", i, ANG, got)
		}
	}
}

// The rule holds for currencies a caller adds, not only for the built-in table.
func TestCurrencies_CurrencyByNumericCodePrefersTheISOCodeAmongAdded(t *testing.T) {
	const numericCode = "999"
	iso := &Currency{Code: "ZZZ", NumericCode: numericCode, Fraction: 2, Grapheme: "z", Template: "$1", Decimal: ".", Thousand: ","}
	variant := &Currency{Code: "AAASPACE", NumericCode: numericCode, Fraction: 2, Grapheme: "z", Template: "$1", Decimal: ".", Thousand: " "}

	cs := Currencies{}.Add(variant).Add(iso)
	for i := 0; i < 200; i++ {
		got := cs.CurrencyByNumericCode(numericCode)
		if got == nil || got.Code != iso.Code {
			t.Fatalf("iteration %d: expected the ISO code %s, got %+v", i, iso.Code, got)
		}
	}
}
