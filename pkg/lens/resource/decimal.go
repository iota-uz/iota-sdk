package resource

import (
	"fmt"
	"math"
	"math/big"
	"reflect"

	"github.com/shopspring/decimal"
)

func decimalEstimate(value reflect.Value, limit int64) (int64, bool, error) {
	var overhead int64 = 128
	switch value.Type() {
	case reflect.TypeFor[decimal.Decimal]():
	case reflect.TypeFor[*decimal.Decimal]():
		if value.IsNil() {
			return 32, true, nil
		}
		value = value.Elem()
		overhead += 32
	case reflect.TypeFor[decimal.NullDecimal]():
		value = value.FieldByName("Decimal")
		overhead += 32
	case reflect.TypeFor[*decimal.NullDecimal]():
		if value.IsNil() {
			return 32, true, nil
		}
		value = value.Elem().FieldByName("Decimal")
		overhead += 64
	default:
		return 0, false, nil
	}
	// shopspring/decimal v1.4 stores an immutable coefficient *big.Int. Its public
	// Coefficient/Copy/NumDigits methods allocate, so inspect only known field
	// shapes and the big.Int word count before any of those methods or MarshalJSON.
	coefficient := value.FieldByName("value")
	exponent := value.FieldByName("exp")
	if !coefficient.IsValid() || coefficient.Type() != reflect.TypeFor[*big.Int]() || !exponent.IsValid() || exponent.Kind() != reflect.Int32 {
		return 0, true, fmt.Errorf("unsupported native decimal layout")
	}
	var words, wordBytes int64
	if !coefficient.IsNil() {
		absolute := coefficient.Elem().FieldByName("abs")
		if !absolute.IsValid() || absolute.Kind() != reflect.Slice || absolute.Type().Elem().Kind() != reflect.Uint {
			return 0, true, fmt.Errorf("unsupported native decimal coefficient layout")
		}
		words = int64(absolute.Len())
		wordBytes = int64(absolute.Type().Elem().Size())
	}
	var size int64
	charge := func(n int64) error {
		if n < 0 || n > limit-size {
			return NewLimitError("decimal bytes", math.MaxInt64, limit)
		}
		size += n
		return nil
	}
	if err := charge(overhead); err != nil {
		return 0, true, err
	}
	if words > 0 {
		if words > limit/wordBytes {
			return 0, true, NewLimitError("decimal coefficient bytes", math.MaxInt64, limit)
		}
		if err := charge(words*wordBytes + 4*wordBytes); err != nil {
			return 0, true, err
		}
	}
	// One binary word has fewer than wordBits/3+1 decimal digits. This upper
	// bound avoids NumDigits' potentially huge 10^digits error-correction value.
	digitsPerWord := wordBytes*8/3 + 1
	if digitsPerWord == 1 {
		digitsPerWord = 22
	}
	if words > (limit-size)/digitsPerWord {
		return 0, true, NewLimitError("decimal digits", math.MaxInt64, limit)
	}
	digits := words*digitsPerWord + 1
	if err := charge(digits); err != nil {
		return 0, true, err
	}
	// Even zero uses rescale(0) for positive exponents and allocates padding before
	// trimming negative exponents. Widen before negating to handle MinInt32.
	exp := exponent.Int()
	if exp < 0 {
		exp = -exp
	}
	if err := charge(exp); err != nil {
		return 0, true, err
	}
	if err := charge(digits + exp + 4); err != nil {
		return 0, true, err
	}
	return size, true, nil
}
