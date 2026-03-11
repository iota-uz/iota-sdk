package money

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"testing"
)

func TestNew(t *testing.T) {
	m := New(1, EUR)

	if m.Amount() != 1 {
		t.Errorf("Expected %d got %d", 1, m.Amount())
	}

	if m.currency.Code != EUR {
		t.Errorf("Expected currency %s got %s", EUR, m.currency.Code)
	}

	m = New(-100, EUR)

	if m.Amount() != -100 {
		t.Errorf("Expected %d got %d", -100, m.Amount())
	}
}

func TestNew_WithUnregisteredCurrency(t *testing.T) {
	const currencyFooCode = "FOO"
	const expectedAmount = 100
	const expectedDisplay = "1.00FOO"

	m := New(100, currencyFooCode)

	if m.Amount() != expectedAmount {
		t.Errorf("Expected amount %d got %d", expectedAmount, m.Amount())
	}

	if m.currency.Code != currencyFooCode {
		t.Errorf("Expected currency code %s got %s", currencyFooCode, m.currency.Code)
	}

	if m.Display() != expectedDisplay {
		t.Errorf("Expected display %s got %s", expectedDisplay, m.Display())
	}
}

func TestCurrency(t *testing.T) {
	code := "MOCK"
	decimals := 5
	AddCurrency(code, "M$", "1 $", ".", ",", decimals)
	m := New(1, code)
	c := m.Currency().Code
	if c != code {
		t.Errorf("Expected %s got %s", code, c)
	}
	f := m.Currency().Fraction
	if f != decimals {
		t.Errorf("Expected %d got %d", decimals, f)
	}
}

func TestMoney_SameCurrency(t *testing.T) {
	m := New(0, EUR)
	om := New(0, USD)

	if m.SameCurrency(om) {
		t.Errorf("Expected %s not to be same as %s", m.currency.Code, om.currency.Code)
	}

	om = New(0, EUR)

	if !m.SameCurrency(om) {
		t.Errorf("Expected %s to be same as %s", m.currency.Code, om.currency.Code)
	}
}

func TestMoney_Equals(t *testing.T) {
	m := New(0, EUR)
	tcs := []struct {
		amount   int64
		expected bool
	}{
		{-1, false},
		{0, true},
		{1, false},
	}

	for _, tc := range tcs {
		om := New(tc.amount, EUR)
		r, err := m.Equals(om)

		if err != nil || r != tc.expected {
			t.Errorf("Expected %d Equals %d == %t got %t", m.Amount(),
				om.Amount(), tc.expected, r)
		}
	}
}

func TestMoney_Equals_DifferentCurrencies(t *testing.T) {
	t.Parallel()

	eur := New(0, EUR)
	usd := New(0, USD)

	_, err := eur.Equals(usd)
	if err == nil || !errors.Is(ErrCurrencyMismatch, err) {
		t.Errorf("Expected Equals to return %q, got %v", ErrCurrencyMismatch.Error(), err)
	}
}

func TestMoney_GreaterThan(t *testing.T) {
	m := New(0, EUR)
	tcs := []struct {
		amount   int64
		expected bool
	}{
		{-1, true},
		{0, false},
		{1, false},
	}

	for _, tc := range tcs {
		om := New(tc.amount, EUR)
		r, err := m.GreaterThan(om)

		if err != nil || r != tc.expected {
			t.Errorf("Expected %d Greater Than %d == %t got %t", m.Amount(),
				om.Amount(), tc.expected, r)
		}
	}
}

func TestMoney_GreaterThanOrEqual(t *testing.T) {
	m := New(0, EUR)
	tcs := []struct {
		amount   int64
		expected bool
	}{
		{-1, true},
		{0, true},
		{1, false},
	}

	for _, tc := range tcs {
		om := New(tc.amount, EUR)
		r, err := m.GreaterThanOrEqual(om)

		if err != nil || r != tc.expected {
			t.Errorf("Expected %d Equals Or Greater Than %d == %t got %t", m.Amount(),
				om.Amount(), tc.expected, r)
		}
	}
}

func TestMoney_LessThan(t *testing.T) {
	m := New(0, EUR)
	tcs := []struct {
		amount   int64
		expected bool
	}{
		{-1, false},
		{0, false},
		{1, true},
	}

	for _, tc := range tcs {
		om := New(tc.amount, EUR)
		r, err := m.LessThan(om)

		if err != nil || r != tc.expected {
			t.Errorf("Expected %d Less Than %d == %t got %t", m.Amount(),
				om.Amount(), tc.expected, r)
		}
	}
}

func TestMoney_LessThanOrEqual(t *testing.T) {
	m := New(0, EUR)
	tcs := []struct {
		amount   int64
		expected bool
	}{
		{-1, false},
		{0, true},
		{1, true},
	}

	for _, tc := range tcs {
		om := New(tc.amount, EUR)
		r, err := m.LessThanOrEqual(om)

		if err != nil || r != tc.expected {
			t.Errorf("Expected %d Equal Or Less Than %d == %t got %t", m.Amount(),
				om.Amount(), tc.expected, r)
		}
	}
}

func TestMoney_IsZero(t *testing.T) {
	tcs := []struct {
		amount   int64
		expected bool
	}{
		{-1, false},
		{0, true},
		{1, false},
	}

	for _, tc := range tcs {
		m := New(tc.amount, EUR)
		r := m.IsZero()

		if r != tc.expected {
			t.Errorf("Expected %d to be zero == %t got %t", m.Amount(), tc.expected, r)
		}
	}
}

func TestMoney_IsNegative(t *testing.T) {
	tcs := []struct {
		amount   int64
		expected bool
	}{
		{-1, true},
		{0, false},
		{1, false},
	}

	for _, tc := range tcs {
		m := New(tc.amount, EUR)
		r := m.IsNegative()

		if r != tc.expected {
			t.Errorf("Expected %d to be negative == %t got %t", m.Amount(),
				tc.expected, r)
		}
	}
}

func TestMoney_IsPositive(t *testing.T) {
	tcs := []struct {
		amount   int64
		expected bool
	}{
		{-1, false},
		{0, false},
		{1, true},
	}

	for _, tc := range tcs {
		m := New(tc.amount, EUR)
		r := m.IsPositive()

		if r != tc.expected {
			t.Errorf("Expected %d to be positive == %t got %t", m.Amount(),
				tc.expected, r)
		}
	}
}

func TestMoney_Absolute(t *testing.T) {
	tcs := []struct {
		amount   int64
		expected int64
	}{
		{-1, 1},
		{0, 0},
		{1, 1},
	}

	for _, tc := range tcs {
		m := New(tc.amount, EUR)
		r := m.Absolute().Amount()

		if r != tc.expected {
			t.Errorf("Expected absolute %d to be %d got %d", m.Amount(),
				tc.expected, r)
		}
	}
}

func TestMoney_Negative(t *testing.T) {
	tcs := []struct {
		amount   int64
		expected int64
	}{
		{-1, -1},
		{0, -0},
		{1, -1},
	}

	for _, tc := range tcs {
		m := New(tc.amount, EUR)
		r := m.Negative().Amount()

		if r != tc.expected {
			t.Errorf("Expected absolute %d to be %d got %d", m.Amount(),
				tc.expected, r)
		}
	}
}

func TestMoney_Add(t *testing.T) {
	tcs := []struct {
		amount1  int64
		amount2  int64
		expected int64
	}{
		{5, 5, 10},
		{10, 5, 15},
		{1, -1, 0},
	}

	for _, tc := range tcs {
		m := New(tc.amount1, EUR)
		om := New(tc.amount2, EUR)
		r, err := m.Add(om)
		if err != nil {
			t.Error(err)
		}

		if r.Amount() != tc.expected {
			t.Errorf("Expected %d + %d = %d got %d", tc.amount1, tc.amount2,
				tc.expected, r.Amount())
		}
	}
}

func TestMoney_Add2(t *testing.T) {
	m := New(100, EUR)
	dm := New(100, GBP)
	r, err := m.Add(dm)

	if r != nil || err == nil {
		t.Error("Expected err")
	}
}

func TestMoney_Add3(t *testing.T) {
	tcs := []struct {
		amount1  int64
		amount2  int64
		amount3  int64
		expected int64
	}{
		{5, 5, 3, 13},
		{10, 5, 4, 19},
		{1, -1, 2, 2},
		{3, -1, -4, -2},
	}

	for _, tc := range tcs {
		mon1 := New(tc.amount1, EUR)
		mon2 := New(tc.amount2, EUR)
		mon3 := New(tc.amount3, EUR)
		r, err := mon1.Add(mon2, mon3)

		if err != nil {
			t.Error(err)
		}

		if r.Amount() != tc.expected {
			t.Errorf("Expected %d + %d + %d = %d got %d", tc.amount1, tc.amount2, tc.amount3,
				tc.expected, r.Amount())
		}
	}
}

func TestMoney_Add4(t *testing.T) {
	m := New(100, EUR)
	r, err := m.Add()

	if err != nil {
		t.Error(err)
	}

	if r.Amount() != 100 {
		t.Error("Expected amount to be 100")
	}
}

func TestMoney_Subtract(t *testing.T) {
	tcs := []struct {
		amount1  int64
		amount2  int64
		expected int64
	}{
		{5, 5, 0},
		{10, 5, 5},
		{1, -1, 2},
	}

	for _, tc := range tcs {
		m := New(tc.amount1, EUR)
		om := New(tc.amount2, EUR)
		r, err := m.Subtract(om)
		if err != nil {
			t.Error(err)
		}

		if r.Amount() != tc.expected {
			t.Errorf("Expected %d - %d = %d got %d", tc.amount1, tc.amount2,
				tc.expected, r.Amount())
		}
	}
}

func TestMoney_Subtract2(t *testing.T) {
	m := New(100, EUR)
	dm := New(100, GBP)
	r, err := m.Subtract(dm)

	if r != nil || err == nil {
		t.Error("Expected err")
	}
}

func TestMoney_Subtract3(t *testing.T) {
	tcs := []struct {
		amount1  int64
		amount2  int64
		amount3  int64
		expected int64
	}{
		{5, 5, 3, -3},
		{10, -5, 4, 11},
		{1, -1, 2, 0},
		{7, 1, -4, 10},
	}

	for _, tc := range tcs {
		mon1 := New(tc.amount1, EUR)
		mon2 := New(tc.amount2, EUR)
		mon3 := New(tc.amount3, EUR)
		r, err := mon1.Subtract(mon2, mon3)

		if err != nil {
			t.Error(err)
		}

		if r.Amount() != tc.expected {
			t.Errorf("Expected (%d) - (%d) - (%d) = %d got %d", tc.amount1, tc.amount2, tc.amount3,
				tc.expected, r.Amount())
		}
	}
}

func TestMoney_Subtract4(t *testing.T) {
	m := New(100, EUR)
	r, err := m.Subtract()

	if err != nil {
		t.Error(err)
	}

	if r.Amount() != 100 {
		t.Error("Expected amount to be 100")
	}
}

func TestMoney_Multiply(t *testing.T) {
	tcs := []struct {
		amount     int64
		multiplier int64
		expected   int64
	}{
		{5, 5, 25},
		{10, 5, 50},
		{1, -1, -1},
		{1, 0, 0},
	}

	for _, tc := range tcs {
		m := New(tc.amount, EUR)
		r := m.Multiply(tc.multiplier).Amount()

		if r != tc.expected {
			t.Errorf("Expected %d * %d = %d got %d", tc.amount, tc.multiplier, tc.expected, r)
		}
	}
}

func TestMoney_Multiply2(t *testing.T) {
	tcs := []struct {
		amount1  int64
		amount2  int64
		amount3  int64
		expected int64
	}{
		{5, 5, 5, 125},
		{10, 5, -3, -150},
		{1, -1, 6, -6},
		{1, 0, 2, 0},
	}

	for _, tc := range tcs {
		mon1 := New(tc.amount1, EUR)
		r := mon1.Multiply(tc.amount2, tc.amount3)

		if r.Amount() != tc.expected {
			t.Errorf("Expected %d * %d * %d = %d got %d", tc.amount1, tc.amount2, tc.amount3, tc.expected, r.Amount())
		}
	}
}

func TestMoney_Round(t *testing.T) {
	tcs := []struct {
		amount   int64
		expected int64
	}{
		{125, 100},
		{175, 200},
		{349, 300},
		{351, 400},
		{0, 0},
		{-1, 0},
		{-75, -100},
	}

	for _, tc := range tcs {
		m := New(tc.amount, EUR)
		r := m.Round().Amount()

		if r != tc.expected {
			t.Errorf("Expected rounded %d to be %d got %d", tc.amount, tc.expected, r)
		}
	}
}

func TestMoney_RoundWithExponential(t *testing.T) {
	tcs := []struct {
		amount   int64
		expected int64
	}{
		{12555, 13000},
	}

	for _, tc := range tcs {
		AddCurrency("CUR", "*", "$1", ".", ",", 3)
		m := New(tc.amount, "CUR")
		r := m.Round().Amount()

		if r != tc.expected {
			t.Errorf("Expected rounded %d to be %d got %d", tc.amount, tc.expected, r)
		}
	}
}

func TestMoney_Split(t *testing.T) {
	tcs := []struct {
		amount   int64
		split    int
		expected []int64
	}{
		{100, 3, []int64{34, 33, 33}},
		{100, 4, []int64{25, 25, 25, 25}},
		{5, 3, []int64{2, 2, 1}},
		{-101, 4, []int64{-26, -25, -25, -25}},
		{-101, 4, []int64{-26, -25, -25, -25}},
		{-2, 3, []int64{-1, -1, 0}},
	}

	for _, tc := range tcs {
		m := New(tc.amount, EUR)
		split, _ := m.Split(tc.split)
		rs := make([]int64, 0, len(tc.expected))

		for _, party := range split {
			rs = append(rs, party.Amount())
		}

		if !reflect.DeepEqual(tc.expected, rs) {
			t.Errorf("Expected split of %d to be %v got %v", tc.amount, tc.expected, rs)
		}
	}
}

func TestMoney_Split2(t *testing.T) {
	m := New(100, EUR)
	r, err := m.Split(-10)

	if r != nil || err == nil {
		t.Error("Expected err")
	}
}

func TestMoney_Allocate(t *testing.T) {
	tcs := []struct {
		amount   int64
		ratios   []int
		expected []int64
	}{
		{100, []int{50, 50}, []int64{50, 50}},
		{100, []int{30, 30, 30}, []int64{34, 33, 33}},
		{200, []int{25, 25, 50}, []int64{50, 50, 100}},
		{5, []int{50, 25, 25}, []int64{3, 1, 1}},
		{0, []int{0, 0, 0, 0}, []int64{0, 0, 0, 0}},
		{0, []int{50, 10}, []int64{0, 0}},
		{10, []int{0, 100}, []int64{0, 10}},
		{10, []int{0, 0}, []int64{0, 0}},
	}

	for _, tc := range tcs {
		m := New(tc.amount, EUR)
		split, _ := m.Allocate(tc.ratios...)
		rs := make([]int64, 0, len(tc.expected))

		for _, party := range split {
			rs = append(rs, party.Amount())
		}

		if !reflect.DeepEqual(tc.expected, rs) {
			t.Errorf("Expected allocation of %d for ratios %v to be %v got %v", tc.amount, tc.ratios,
				tc.expected, rs)
		}
	}
}

func TestMoney_Allocate2(t *testing.T) {
	m := New(100, EUR)
	r, err := m.Allocate()

	if r != nil || err == nil {
		t.Error("Expected err")
	}
}

func TestAllocateOverflow(t *testing.T) {
	m := New(math.MaxInt64, EUR)
	_, err := m.Allocate(math.MaxInt, 1)
	if err == nil {
		t.Fatalf("expected an error, but got nil")
	}

	expectedErrorMessage := "sum of given ratios exceeds max int"
	if err.Error() != expectedErrorMessage {
		t.Fatalf("expected error message %q, but got %q", expectedErrorMessage, err.Error())
	}
}

func TestMoney_Format(t *testing.T) {
	tcs := []struct {
		amount   int64
		code     string
		expected string
	}{
		{100, GBP, "£1.00"},
	}

	for _, tc := range tcs {
		m := New(tc.amount, tc.code)
		r := m.Display()

		if r != tc.expected {
			t.Errorf("Expected formatted %d to be %s got %s", tc.amount, tc.expected, r)
		}
	}
}

func TestMoney_Display(t *testing.T) {
	tcs := []struct {
		amount   int64
		code     string
		expected string
	}{
		{100, AED, "1.00 .\u062f.\u0625"},
		{1, USD, "$0.01"},
	}

	for _, tc := range tcs {
		m := New(tc.amount, tc.code)
		r := m.Display()

		if r != tc.expected {
			t.Errorf("Expected formatted %d to be %s got %s", tc.amount, tc.expected, r)
		}
	}
}

func TestMoney_AsMajorUnits(t *testing.T) {
	tcs := []struct {
		amount   int64
		code     string
		expected float64
	}{
		{100, AED, 1.00},
		{1, USD, 0.01},
	}

	for _, tc := range tcs {
		m := New(tc.amount, tc.code)
		r := m.AsMajorUnits()

		if r != tc.expected {
			t.Errorf("Expected value as major units of %d to be %f got %f", tc.amount, tc.expected, r)
		}
	}
}

func TestMoney_Allocate3(t *testing.T) {
	pound := New(100, GBP)
	parties, err := pound.Allocate(33, 33, 33)
	if err != nil {
		t.Error(err)
	}

	if parties[0].Display() != "£0.34" {
		t.Errorf("Expected %s got %s", "£0.34", parties[0].Display())
	}

	if parties[1].Display() != "£0.33" {
		t.Errorf("Expected %s got %s", "£0.33", parties[1].Display())
	}

	if parties[2].Display() != "£0.33" {
		t.Errorf("Expected %s got %s", "£0.33", parties[2].Display())
	}
}

func TestMoney_Comparison(t *testing.T) {
	pound := New(100, GBP)
	twoPounds := New(200, GBP)
	twoEuros := New(200, EUR)

	if r, err := pound.GreaterThan(twoPounds); err != nil || r {
		t.Errorf("Expected %d Greater Than %d == %t got %t", pound.Amount(),
			twoPounds.Amount(), false, r)
	}

	if r, err := pound.LessThan(twoPounds); err != nil || !r {
		t.Errorf("Expected %d Less Than %d == %t got %t", pound.Amount(),
			twoPounds.Amount(), true, r)
	}

	if r, err := pound.LessThan(twoEuros); err == nil || r {
		t.Error("Expected err")
	}

	if r, err := pound.GreaterThan(twoEuros); err == nil || r {
		t.Error("Expected err")
	}

	if r, err := pound.Equals(twoEuros); err == nil || r {
		t.Error("Expected err")
	}

	if r, err := pound.LessThanOrEqual(twoEuros); err == nil || r {
		t.Error("Expected err")
	}

	if r, err := pound.GreaterThanOrEqual(twoEuros); err == nil || r {
		t.Error("Expected err")
	}

	if r, err := twoPounds.Compare(pound); r != 1 && err != nil {
		t.Errorf("Expected %d Greater Than %d == %d got %d", pound.Amount(),
			twoPounds.Amount(), 1, r)
	}

	if r, err := pound.Compare(twoPounds); r != -1 && err != nil {
		t.Errorf("Expected %d Less Than %d == %d got %d", pound.Amount(),
			twoPounds.Amount(), -1, r)
	}

	if _, err := pound.Compare(twoEuros); err != ErrCurrencyMismatch {
		t.Error("Expected err")
	}

	anotherTwoEuros := New(200, EUR)
	if r, err := twoEuros.Compare(anotherTwoEuros); r != 0 && err != nil {
		t.Errorf("Expected %d Equals to %d == %d got %d", anotherTwoEuros.Amount(),
			twoEuros.Amount(), 0, r)
	}
}

func TestMoney_Currency(t *testing.T) {
	pound := New(100, GBP)

	if pound.Currency().Code != GBP {
		t.Errorf("Expected %s got %s", GBP, pound.Currency().Code)
	}
}

func TestMoney_Amount(t *testing.T) {
	pound := New(100, GBP)

	if pound.Amount() != 100 {
		t.Errorf("Expected %d got %d", 100, pound.Amount())
	}
}

func TestNewFromFloat(t *testing.T) {
	m := NewFromFloat(12.34, EUR)

	if m.Amount() != 1234 {
		t.Errorf("Expected %d got %d", 1234, m.Amount())
	}

	if m.currency.Code != EUR {
		t.Errorf("Expected currency %s got %s", EUR, m.currency.Code)
	}

	m = NewFromFloat(12.34, "eur")

	if m.Amount() != 1234 {
		t.Errorf("Expected %d got %d", 1234, m.Amount())
	}

	if m.currency.Code != EUR {
		t.Errorf("Expected currency %s got %s", EUR, m.currency.Code)
	}

	m = NewFromFloat(-0.125, EUR)

	if m.Amount() != -12 {
		t.Errorf("Expected %d got %d", -12, m.Amount())
	}
}

func TestNewFromFloat_WithUnregisteredCurrency(t *testing.T) {
	const currencyFooCode = "FOO"
	const expectedAmount = 1234
	const expectedDisplay = "12.34FOO"

	m := NewFromFloat(12.34, currencyFooCode)

	if m.Amount() != expectedAmount {
		t.Errorf("Expected amount %d got %d", expectedAmount, m.Amount())
	}

	if m.currency.Code != currencyFooCode {
		t.Errorf("Expected currency code %s got %s", currencyFooCode, m.currency.Code)
	}

	if m.Display() != expectedDisplay {
		t.Errorf("Expected display %s got %s", expectedDisplay, m.Display())
	}
}

func TestDefaultMarshal(t *testing.T) {
	given := New(12345, IQD)
	expected := `{"amount":12345,"currency":"IQD"}`

	b, err := json.Marshal(given)
	if err != nil {
		t.Error(err)
	}

	if string(b) != expected {
		t.Errorf("Expected %s got %s", expected, string(b))
	}

	given = &Money{}
	expected = `{"amount":0,"currency":""}`

	b, err = json.Marshal(given)
	if err != nil {
		t.Error(err)
	}

	if string(b) != expected {
		t.Errorf("Expected %s got %s", expected, string(b))
	}
}

func TestCustomMarshal(t *testing.T) {
	given := New(12345, IQD)
	expected := `{"amount":12345,"currency_code":"IQD","currency_fraction":3}`
	MarshalJSON = func(m Money) ([]byte, error) {
		buff := bytes.NewBufferString(fmt.Sprintf(`{"amount": %d, "currency_code": "%s", "currency_fraction": %d}`, m.Amount(), m.Currency().Code, m.Currency().Fraction))
		return buff.Bytes(), nil
	}

	b, err := json.Marshal(given)
	if err != nil {
		t.Error(err)
	}

	if string(b) != expected {
		t.Errorf("Expected %s got %s", expected, string(b))
	}
}

func TestDefaultUnmarshal(t *testing.T) {
	// Reset to default after TestCustomMarshal may have changed it
	MarshalJSON = defaultMarshalJSON
	UnmarshalJSON = defaultUnmarshalJSON

	given := `{"amount": 10012, "currency":"USD"}`
	expected := "$100.12"
	var m Money
	err := json.Unmarshal([]byte(given), &m)
	if err != nil {
		t.Error(err)
	}

	if m.Display() != expected {
		t.Errorf("Expected %s got %s", expected, m.Display())
	}

	given = `{"amount": 0, "currency":""}`
	err = json.Unmarshal([]byte(given), &m)
	if err != nil {
		t.Error(err)
	}

	if m != (Money{}) {
		t.Errorf("Expected zero value, got %+v", m)
	}

	given = `{}`
	err = json.Unmarshal([]byte(given), &m)
	if err != nil {
		t.Error(err)
	}

	if m != (Money{}) {
		t.Errorf("Expected zero value, got %+v", m)
	}

	given = `{"amount": "foo", "currency": "USD"}`
	err = json.Unmarshal([]byte(given), &m)
	if !errors.Is(err, ErrInvalidJSONUnmarshal) {
		t.Errorf("Expected ErrInvalidJSONUnmarshal, got %+v", err)
	}

	given = `{"amount": 1234, "currency": 1234}`
	err = json.Unmarshal([]byte(given), &m)
	if !errors.Is(err, ErrInvalidJSONUnmarshal) {
		t.Errorf("Expected ErrInvalidJSONUnmarshal, got %+v", err)
	}
}

func TestCustomUnmarshal(t *testing.T) {
	given := `{"amount": 10012, "currency_code":"USD", "currency_fraction":2}`
	expected := "$100.12"
	UnmarshalJSON = func(m *Money, b []byte) error {
		data := make(map[string]interface{})
		err := json.Unmarshal(b, &data)
		if err != nil {
			return err
		}
		ref := New(int64(data["amount"].(float64)), data["currency_code"].(string))
		*m = *ref
		return nil
	}

	var m Money
	err := json.Unmarshal([]byte(given), &m)
	if err != nil {
		t.Error(err)
	}

	if m.Display() != expected {
		t.Errorf("Expected %s got %s", expected, m.Display())
	}
}

func TestMoney_DisplayWithSpaces(t *testing.T) {
	tcs := []struct {
		amount   int64
		code     string
		expected string
	}{
		{566666, USDSPACE, "$5 666.66"},   // Space-separated thousands
		{100, USDSPACE, "$1.00"},          // No thousands separator needed
		{1000, USDSPACE, "$10.00"},        // No thousands separator needed
		{100000, USDSPACE, "$1 000.00"},   // Space separator
		{1234567, USDSPACE, "$12 345.67"}, // Multiple space separators
		{-566666, USDSPACE, "-$5 666.66"}, // Negative numbers
		{0, USDSPACE, "$0.00"},            // Zero
		{1, USDSPACE, "$0.01"},            // Small amounts
	}

	for _, tc := range tcs {
		m := New(tc.amount, tc.code)
		r := m.Display()

		if r != tc.expected {
			t.Errorf("Expected formatted %d with %s to be %s got %s", tc.amount, tc.code, tc.expected, r)
		}
	}
}

func TestMoney_USDSpace_BackwardCompatibility(t *testing.T) {
	// Test that regular USD still uses comma separators
	usdAmount := int64(566666)
	usdMoney := New(usdAmount, USD)
	expectedUSD := "$5,666.66"
	actualUSD := usdMoney.Display()

	if actualUSD != expectedUSD {
		t.Errorf("Expected USD to use comma separator: %s, got: %s", expectedUSD, actualUSD)
	}

	// Test that USDSPACE uses space separators
	usdSpaceMoney := New(usdAmount, USDSPACE)
	expectedUSDSpace := "$5 666.66"
	actualUSDSpace := usdSpaceMoney.Display()

	if actualUSDSpace != expectedUSDSpace {
		t.Errorf("Expected USDSPACE to use space separator: %s, got: %s", expectedUSDSpace, actualUSDSpace)
	}
}

func TestMoney_USDSpace_FloatConstructor(t *testing.T) {
	tcs := []struct {
		amount   float64
		expected string
	}{
		{5666.66, "$5 666.66"},
		{1.00, "$1.00"},
		{10.50, "$10.50"},
		{1000.75, "$1 000.75"},
		{12345.67, "$12 345.67"},
		{-5666.66, "-$5 666.66"},
		{0.01, "$0.01"},
	}

	for _, tc := range tcs {
		m := NewFromFloat(tc.amount, USDSPACE)
		r := m.Display()

		if r != tc.expected {
			t.Errorf("Expected NewFromFloat(%f, USDSPACE) to be %s got %s", tc.amount, tc.expected, r)
		}
	}
}

func TestMoney_USDSpace_Operations(t *testing.T) {
	// Test arithmetic operations work correctly with USDSPACE
	m1 := NewFromFloat(1000.50, USDSPACE)
	m2 := NewFromFloat(500.25, USDSPACE)

	// Addition
	sum, err := m1.Add(m2)
	if err != nil {
		t.Errorf("Addition failed: %v", err)
	}
	expectedSum := "$1 500.75"
	if sum.Display() != expectedSum {
		t.Errorf("Expected sum to be %s, got %s", expectedSum, sum.Display())
	}

	// Subtraction
	diff, err := m1.Subtract(m2)
	if err != nil {
		t.Errorf("Subtraction failed: %v", err)
	}
	expectedDiff := "$500.25"
	if diff.Display() != expectedDiff {
		t.Errorf("Expected difference to be %s, got %s", expectedDiff, diff.Display())
	}

	// Multiplication
	mult := m1.Multiply(2)
	expectedMult := "$2 001.00"
	if mult.Display() != expectedMult {
		t.Errorf("Expected multiplication result to be %s, got %s", expectedMult, mult.Display())
	}
}

func TestMoney_USDSpace_CompactDisplay(t *testing.T) {
	tcs := []struct {
		amount   int64
		decimals int
		expected string
	}{
		{123456700, 1, "1.2M $"},    // 1,234,567.00 -> 1.2M $
		{123456700, 2, "1.23M $"},   // 1,234,567.00 -> 1.23M $
		{2252423200, 2, "22.52M $"}, // 22,524,232.00 -> 22.52M $
		{123400, 2, "1.23K $"},      // 1,234.00 -> 1.23K $
		{50000, 1, "$500.00"},       // Uses regular format for smaller amounts
	}

	for _, tc := range tcs {
		m := New(tc.amount, USDSPACE)
		r := m.DisplayCompact(tc.decimals)

		if r != tc.expected {
			t.Errorf("Expected compact format of %d to be %s got %s", tc.amount, tc.expected, r)
		}
	}
}

// --- New big.Int tests ---

func TestNewFromBigInt(t *testing.T) {
	bi := new(big.Int)
	bi.SetString("38499843389614000000", 10) // > MaxInt64
	m := NewFromBigInt(bi, UZS)

	if m.BigAmount().Cmp(bi) != 0 {
		t.Errorf("Expected BigAmount to equal %s, got %s", bi.String(), m.BigAmount().String())
	}

	// Verify it's a copy
	bi.SetInt64(0)
	if m.BigAmount().Sign() == 0 {
		t.Error("Expected NewFromBigInt to store a copy, not a reference")
	}
}

func TestNewFromFloat_LargeValue_NoOverflow(t *testing.T) {
	// 384998433896140.00 UZS -> minor units = 38499843389614000
	m := NewFromFloat(384998433896140.00, UZS)
	expected := new(big.Int)
	expected.SetString("38499843389614000", 10)

	if m.BigAmount().Cmp(expected) != 0 {
		t.Errorf("Expected BigAmount %s, got %s", expected.String(), m.BigAmount().String())
	}
}

func TestNewFromFloat_384Quadrillion_UZS(t *testing.T) {
	// 384_998_433_896_140 UZS (in major units), fraction=2
	// This is the real-world case from QANOT SHARQ
	amount := 3849984338961.40 // in UZS major units
	m := NewFromFloat(amount, UZS)

	// Should not overflow - the value should be positive and large
	if !m.IsPositive() {
		t.Error("Expected positive value for large UZS amount")
	}
}

func TestAmount_ReturnsInt64_WhenFits(t *testing.T) {
	m := New(42, EUR)
	if m.Amount() != 42 {
		t.Errorf("Expected 42, got %d", m.Amount())
	}
}

func TestAmount_ReturnsMaxInt64_WhenOverflow(t *testing.T) {
	bi := new(big.Int)
	bi.SetString("99999999999999999999", 10) // > MaxInt64
	m := NewFromBigInt(bi, EUR)

	if m.Amount() != math.MaxInt64 {
		t.Errorf("Expected MaxInt64, got %d", m.Amount())
	}
}

func TestAmount_ReturnsMinInt64_WhenNegativeOverflow(t *testing.T) {
	bi := new(big.Int)
	bi.SetString("-99999999999999999999", 10) // < MinInt64
	m := NewFromBigInt(bi, EUR)

	if m.Amount() != math.MinInt64 {
		t.Errorf("Expected MinInt64, got %d", m.Amount())
	}
}

func TestBigAmount_ExactValue(t *testing.T) {
	bi := new(big.Int)
	bi.SetString("38499843389614000000", 10)
	m := NewFromBigInt(bi, UZS)

	if m.BigAmount().Cmp(bi) != 0 {
		t.Errorf("Expected %s, got %s", bi.String(), m.BigAmount().String())
	}
}

func TestBigAmount_NilSafety(t *testing.T) {
	m := &Money{}
	result := m.BigAmount()
	if result == nil {
		t.Fatal("BigAmount should not return nil for zero-value Money")
	}
	if result.Sign() != 0 {
		t.Errorf("Expected 0 for nil amount, got %s", result.String())
	}
}

func TestBigAmount_ReturnsCopy(t *testing.T) {
	m := New(100, EUR)
	a := m.BigAmount()
	a.SetInt64(999)
	if m.Amount() != 100 {
		t.Error("BigAmount should return a copy, not a reference")
	}
}

func TestAdd_BigValues(t *testing.T) {
	bi1 := new(big.Int)
	bi1.SetString("99999999999999999999", 10)
	bi2 := new(big.Int)
	bi2.SetString("1", 10)

	m1 := NewFromBigInt(bi1, EUR)
	m2 := NewFromBigInt(bi2, EUR)
	r, err := m1.Add(m2)
	if err != nil {
		t.Fatal(err)
	}

	expected := new(big.Int)
	expected.SetString("100000000000000000000", 10)
	if r.BigAmount().Cmp(expected) != 0 {
		t.Errorf("Expected %s, got %s", expected.String(), r.BigAmount().String())
	}
}

func TestSubtract_BigValues(t *testing.T) {
	bi1 := new(big.Int)
	bi1.SetString("100000000000000000000", 10)
	bi2 := new(big.Int)
	bi2.SetString("1", 10)

	m1 := NewFromBigInt(bi1, EUR)
	m2 := NewFromBigInt(bi2, EUR)
	r, err := m1.Subtract(m2)
	if err != nil {
		t.Fatal(err)
	}

	expected := new(big.Int)
	expected.SetString("99999999999999999999", 10)
	if r.BigAmount().Cmp(expected) != 0 {
		t.Errorf("Expected %s, got %s", expected.String(), r.BigAmount().String())
	}
}

func TestMultiply_BigValues(t *testing.T) {
	bi := new(big.Int)
	bi.SetString("10000000000000000000", 10)
	m := NewFromBigInt(bi, EUR)
	r := m.Multiply(10)

	expected := new(big.Int)
	expected.SetString("100000000000000000000", 10)
	if r.BigAmount().Cmp(expected) != 0 {
		t.Errorf("Expected %s, got %s", expected.String(), r.BigAmount().String())
	}
}

func TestMultiply_OverflowInt64_StillCorrect(t *testing.T) {
	m := New(math.MaxInt64, EUR)
	r := m.Multiply(2)

	expected := new(big.Int).Mul(big.NewInt(math.MaxInt64), big.NewInt(2))
	if r.BigAmount().Cmp(expected) != 0 {
		t.Errorf("Expected %s, got %s", expected.String(), r.BigAmount().String())
	}
}

func TestSplit_BigValues(t *testing.T) {
	bi := new(big.Int)
	bi.SetString("100000000000000000001", 10)
	m := NewFromBigInt(bi, EUR)

	parts, err := m.Split(3)
	if err != nil {
		t.Fatal(err)
	}

	// Sum of parts should equal original
	total := big.NewInt(0)
	for _, p := range parts {
		total.Add(total, p.BigAmount())
	}
	if total.Cmp(bi) != 0 {
		t.Errorf("Sum of split parts %s != original %s", total.String(), bi.String())
	}
}

func TestAllocate_BigValues(t *testing.T) {
	bi := new(big.Int)
	bi.SetString("100000000000000000001", 10)
	m := NewFromBigInt(bi, EUR)

	parts, err := m.Allocate(50, 50)
	if err != nil {
		t.Fatal(err)
	}

	total := big.NewInt(0)
	for _, p := range parts {
		total.Add(total, p.BigAmount())
	}
	if total.Cmp(bi) != 0 {
		t.Errorf("Sum of allocated parts %s != original %s", total.String(), bi.String())
	}
}

func TestCompare_BigValues(t *testing.T) {
	bi1 := new(big.Int)
	bi1.SetString("99999999999999999999", 10)
	bi2 := new(big.Int)
	bi2.SetString("100000000000000000000", 10)

	m1 := NewFromBigInt(bi1, EUR)
	m2 := NewFromBigInt(bi2, EUR)

	r, err := m1.Compare(m2)
	if err != nil {
		t.Fatal(err)
	}
	if r != -1 {
		t.Errorf("Expected -1, got %d", r)
	}
}

func TestEquals_BigValues(t *testing.T) {
	bi := new(big.Int)
	bi.SetString("99999999999999999999", 10)

	m1 := NewFromBigInt(bi, EUR)
	m2 := NewFromBigInt(new(big.Int).Set(bi), EUR)

	eq, err := m1.Equals(m2)
	if err != nil {
		t.Fatal(err)
	}
	if !eq {
		t.Error("Expected equal big values to be equal")
	}
}

func TestGreaterThan_BigValues(t *testing.T) {
	bi1 := new(big.Int)
	bi1.SetString("100000000000000000000", 10)
	bi2 := new(big.Int)
	bi2.SetString("99999999999999999999", 10)

	m1 := NewFromBigInt(bi1, EUR)
	m2 := NewFromBigInt(bi2, EUR)

	gt, err := m1.GreaterThan(m2)
	if err != nil {
		t.Fatal(err)
	}
	if !gt {
		t.Error("Expected m1 > m2")
	}
}

func TestLessThan_BigValues(t *testing.T) {
	bi1 := new(big.Int)
	bi1.SetString("99999999999999999999", 10)
	bi2 := new(big.Int)
	bi2.SetString("100000000000000000000", 10)

	m1 := NewFromBigInt(bi1, EUR)
	m2 := NewFromBigInt(bi2, EUR)

	lt, err := m1.LessThan(m2)
	if err != nil {
		t.Fatal(err)
	}
	if !lt {
		t.Error("Expected m1 < m2")
	}
}

func TestIsZero_NilAmount(t *testing.T) {
	m := &Money{}
	if !m.IsZero() {
		t.Error("Expected nil amount to be zero")
	}
}

func TestIsPositive_BigValue(t *testing.T) {
	bi := new(big.Int)
	bi.SetString("99999999999999999999", 10)
	m := NewFromBigInt(bi, EUR)
	if !m.IsPositive() {
		t.Error("Expected big positive value to be positive")
	}
}

func TestIsNegative_BigValue(t *testing.T) {
	bi := new(big.Int)
	bi.SetString("-99999999999999999999", 10)
	m := NewFromBigInt(bi, EUR)
	if !m.IsNegative() {
		t.Error("Expected big negative value to be negative")
	}
}

func TestAbsolute_BigValue(t *testing.T) {
	bi := new(big.Int)
	bi.SetString("-99999999999999999999", 10)
	m := NewFromBigInt(bi, EUR)
	abs := m.Absolute()

	expected := new(big.Int)
	expected.SetString("99999999999999999999", 10)
	if abs.BigAmount().Cmp(expected) != 0 {
		t.Errorf("Expected %s, got %s", expected.String(), abs.BigAmount().String())
	}
}

func TestNegative_BigValue(t *testing.T) {
	bi := new(big.Int)
	bi.SetString("99999999999999999999", 10)
	m := NewFromBigInt(bi, EUR)
	neg := m.Negative()

	expected := new(big.Int)
	expected.SetString("-99999999999999999999", 10)
	if neg.BigAmount().Cmp(expected) != 0 {
		t.Errorf("Expected %s, got %s", expected.String(), neg.BigAmount().String())
	}
}

func TestRound_BigValue(t *testing.T) {
	bi := new(big.Int)
	bi.SetString("99999999999999999975", 10) // last two digits: 75 > 50
	m := NewFromBigInt(bi, EUR)              // EUR fraction = 2
	rounded := m.Round()

	expected := new(big.Int)
	expected.SetString("100000000000000000000", 10)
	if rounded.BigAmount().Cmp(expected) != 0 {
		t.Errorf("Expected %s, got %s", expected.String(), rounded.BigAmount().String())
	}
}

func TestDisplay_BigValue(t *testing.T) {
	bi := new(big.Int)
	bi.SetString("1234567890123456789", 10)
	m := NewFromBigInt(bi, USD)
	display := m.Display()

	// Should contain some reasonable formatting without panic
	if display == "" {
		t.Error("Display should not return empty string for big value")
	}
}

func TestDisplayCompact_BigValue(t *testing.T) {
	bi := new(big.Int)
	bi.SetString("100000000000000000000", 10)
	m := NewFromBigInt(bi, USD)
	compact := m.DisplayCompact(1)

	if compact == "" {
		t.Error("DisplayCompact should not return empty string for big value")
	}
}

func TestAsMajorUnits_BigValue(t *testing.T) {
	// For values that fit in int64, AsMajorUnits should work fine
	m := New(100, USD)
	if m.AsMajorUnits() != 1.0 {
		t.Errorf("Expected 1.0, got %f", m.AsMajorUnits())
	}
}

func TestMarshalJSON_BigValue(t *testing.T) {
	// Reset to default
	MarshalJSON = defaultMarshalJSON

	bi := new(big.Int)
	bi.SetString("99999999999999999999", 10)
	m := NewFromBigInt(bi, USD)

	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	expected := `{"amount":99999999999999999999,"currency":"USD"}`
	if string(b) != expected {
		t.Errorf("Expected %s, got %s", expected, string(b))
	}
}

func TestUnmarshalJSON_BigValue(t *testing.T) {
	UnmarshalJSON = defaultUnmarshalJSON

	given := `{"amount": 99999999999999999999, "currency":"USD"}`
	var m Money
	err := json.Unmarshal([]byte(given), &m)
	if err != nil {
		t.Fatal(err)
	}

	expected := new(big.Int)
	expected.SetString("99999999999999999999", 10)
	if m.BigAmount().Cmp(expected) != 0 {
		t.Errorf("Expected %s, got %s", expected.String(), m.BigAmount().String())
	}
}

func TestMarshalJSON_BackwardCompatible(t *testing.T) {
	MarshalJSON = defaultMarshalJSON

	m := New(12345, USD)
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	expected := `{"amount":12345,"currency":"USD"}`
	if string(b) != expected {
		t.Errorf("Expected %s, got %s", expected, string(b))
	}
}

func TestUnmarshalJSON_BackwardCompatible(t *testing.T) {
	UnmarshalJSON = defaultUnmarshalJSON

	given := `{"amount": 12345, "currency":"USD"}`
	var m Money
	err := json.Unmarshal([]byte(given), &m)
	if err != nil {
		t.Fatal(err)
	}

	if m.Amount() != 12345 {
		t.Errorf("Expected 12345, got %d", m.Amount())
	}
	if m.Currency().Code != USD {
		t.Errorf("Expected USD, got %s", m.Currency().Code)
	}
}

func TestJSON_RoundTrip_SmallValue(t *testing.T) {
	MarshalJSON = defaultMarshalJSON
	UnmarshalJSON = defaultUnmarshalJSON

	original := New(42, EUR)
	b, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}

	var restored Money
	err = json.Unmarshal(b, &restored)
	if err != nil {
		t.Fatal(err)
	}

	if restored.Amount() != original.Amount() {
		t.Errorf("Round trip failed: expected %d, got %d", original.Amount(), restored.Amount())
	}
}

func TestJSON_RoundTrip_BigValue(t *testing.T) {
	MarshalJSON = defaultMarshalJSON
	UnmarshalJSON = defaultUnmarshalJSON

	bi := new(big.Int)
	bi.SetString("99999999999999999999", 10)
	original := NewFromBigInt(bi, EUR)

	b, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}

	var restored Money
	err = json.Unmarshal(b, &restored)
	if err != nil {
		t.Fatal(err)
	}

	if restored.BigAmount().Cmp(original.BigAmount()) != 0 {
		t.Errorf("Round trip failed: expected %s, got %s", original.BigAmount().String(), restored.BigAmount().String())
	}
}

func TestJSON_RoundTrip_NegativeBigValue(t *testing.T) {
	MarshalJSON = defaultMarshalJSON
	UnmarshalJSON = defaultUnmarshalJSON

	bi := new(big.Int)
	bi.SetString("-99999999999999999999", 10)
	original := NewFromBigInt(bi, EUR)

	b, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}

	var restored Money
	err = json.Unmarshal(b, &restored)
	if err != nil {
		t.Fatal(err)
	}

	if restored.BigAmount().Cmp(original.BigAmount()) != 0 {
		t.Errorf("Round trip failed: expected %s, got %s", original.BigAmount().String(), restored.BigAmount().String())
	}
}

func TestMultiply_PanicOnEmpty(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when calling Multiply with no args")
		}
	}()
	m := New(100, EUR)
	m.Multiply()
}

func TestDisplay_BigValue_UsesFormatBigInt(t *testing.T) {
	// Value that doesn't fit in int64 should use FormatBigInt path
	bi := new(big.Int)
	bi.SetString("12345678901234567890", 10)
	m := NewFromBigInt(bi, USD)
	display := m.Display()
	if display == "" {
		t.Error("Expected non-empty display for big value")
	}
	// Should contain the grapheme
	if !contains(display, "$") {
		t.Errorf("Expected display to contain $, got %s", display)
	}
}

func TestDisplayCompact_BigValue_UsesFormatCompactBigInt(t *testing.T) {
	bi := new(big.Int)
	bi.SetString("12345678901234567890", 10)
	m := NewFromBigInt(bi, USD)
	compact := m.DisplayCompact(2)
	if compact == "" {
		t.Error("Expected non-empty compact display for big value")
	}
}

func TestAmount_NilAmount(t *testing.T) {
	m := &Money{}
	if m.Amount() != 0 {
		t.Errorf("Expected 0 for nil amount, got %d", m.Amount())
	}
}

func TestUnmarshalJSON_FloatAmount(t *testing.T) {
	// Test float64 path in unmarshal (when UseNumber is not available)
	UnmarshalJSON = defaultUnmarshalJSON
	given := `{"amount": 123.0, "currency":"USD"}`
	var m Money
	err := json.Unmarshal([]byte(given), &m)
	if err != nil {
		t.Fatal(err)
	}
	if m.Amount() != 123 {
		t.Errorf("Expected 123, got %d", m.Amount())
	}
}

func TestUnmarshalJSON_FloatNumberString(t *testing.T) {
	// Test json.Number that is a float (not integer parseable by big.Int)
	UnmarshalJSON = defaultUnmarshalJSON
	given := `{"amount": 123.7, "currency":"USD"}`
	var m Money
	err := json.Unmarshal([]byte(given), &m)
	if err != nil {
		t.Fatal(err)
	}
	if m.Amount() != 123 {
		t.Errorf("Expected 123, got %d", m.Amount())
	}
}

func TestAllocate_NegativeRatio(t *testing.T) {
	m := New(100, EUR)
	_, err := m.Allocate(-1)
	if err == nil {
		t.Error("Expected error for negative ratio")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestJSON_RoundTrip_Zero(t *testing.T) {
	MarshalJSON = defaultMarshalJSON
	UnmarshalJSON = defaultUnmarshalJSON

	original := New(0, EUR)

	b, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}

	var restored Money
	err = json.Unmarshal(b, &restored)
	if err != nil {
		t.Fatal(err)
	}

	if restored.Amount() != 0 {
		t.Errorf("Round trip failed: expected 0, got %d", restored.Amount())
	}
}
