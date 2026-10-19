package resource

import (
	"encoding/json"
	"math"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestDecimalNativeValuesHaveBoundedPreflight(t *testing.T) {
	t.Parallel()
	amount := decimal.RequireFromString("12345678901234567890.1234567890123456789")
	nullable := decimal.NewNullDecimal(amount)
	for _, value := range []any{decimal.Decimal{}, amount, &amount, nullable, &nullable, decimal.NullDecimal{Decimal: amount}} {
		_, err := Estimate(value, 10000)
		require.NoError(t, err)
		_, err = EstimateJSON(value, 10000)
		require.NoError(t, err)
	}
}

func TestDecimalRejectsHugeExpansionAndCoefficientBeforeEncoding(t *testing.T) {
	t.Parallel()
	// String/NumDigits/Coefficient allocate before they can report the size; none
	// may run to discover these bounds, including a zero with a giant exponent.
	for _, exp := range []int32{math.MaxInt32, math.MinInt32} {
		for _, coefficient := range []int64{0, 1, -1} {
			value := decimal.New(coefficient, exp)
			_, err := EstimateJSON(value, 1024)
			require.ErrorIs(t, err, ErrLimit)
			_, err = Estimate(&value, 1024)
			require.ErrorIs(t, err, ErrLimit)
		}
	}
	huge := decimal.NewFromBigInt(new(big.Int).Lsh(big.NewInt(1), 1<<20), 0)
	_, err := Estimate(huge, 1024)
	require.ErrorIs(t, err, ErrLimit)
	// Invalid nullable values still retain their coefficient in an owned object.
	_, err = Estimate(decimal.NullDecimal{Decimal: huge}, 1024)
	require.ErrorIs(t, err, ErrLimit)
	_, err = EstimateJSON(hostile{}, 1024)
	require.Error(t, err)
}

func TestBytePayloadsUseEncodedLengthInsteadOfPerByteNodes(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`"` + strings.Repeat("x", 1<<20) + `"`)
	size, err := EstimateJSON(raw, DefaultEntryLimit)
	require.NoError(t, err)
	require.Less(t, size, int64(7<<20))
	data := make([]byte, 1<<20)
	size, err = EstimateJSON(data, DefaultEntryLimit)
	require.NoError(t, err)
	require.Less(t, size, int64(3<<20))
	// RawMessage HTML escaping can expand a character into a six-byte escape.
	_, err = EstimateJSON(json.RawMessage(`"`+strings.Repeat("<", 1024)+`"`), 1024)
	require.ErrorIs(t, err, ErrLimit)
	_, err = EstimateJSON(make([]byte, 1024), 1024)
	require.ErrorIs(t, err, ErrLimit)
}

func TestProjectedOwnershipStillCountsJSONIgnoredState(t *testing.T) {
	t.Parallel()
	type renderer struct{ opaque []string }
	type metadata struct {
		Hidden   []byte `json:"-"`
		Renderer renderer
	}
	value := metadata{Hidden: make([]byte, 10000), Renderer: renderer{opaque: []string{"owned elsewhere"}}}
	_, err := EstimateProjected(value, 1024, reflect.TypeFor[renderer]())
	require.ErrorIs(t, err, ErrLimit)
	value.Hidden = nil
	_, err = EstimateProjected(value, 1024, reflect.TypeFor[renderer]())
	require.NoError(t, err)
	_, err = EstimateProjected(hostile{}, 1024, reflect.TypeFor[renderer]())
	require.Error(t, err)
}

type hostileByte uint8

func (*hostileByte) MarshalJSON() ([]byte, error) { panic("named-byte encoder must not run") }

type hostileTextByte uint8

func (hostileTextByte) MarshalText() ([]byte, error) { panic("named-byte text encoder must not run") }
func TestBytePreflightRejectsNamedElementEncoders(t *testing.T) {
	t.Parallel()
	_, err := EstimateJSON([]hostileByte{1}, 1000)
	require.Error(t, err)
	_, err = EstimateJSON([]hostileTextByte{1}, 1000)
	require.Error(t, err)
}
