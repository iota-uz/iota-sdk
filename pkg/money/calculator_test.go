package money

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func setBigInt(s string) *big.Int {
	n, _ := new(big.Int).SetString(s, 10)
	return n
}

func TestCalculator_Add(t *testing.T) {
	c := &calculator{}
	tests := []struct {
		name     string
		a, b     *big.Int
		expected *big.Int
	}{
		{"small values", big.NewInt(5), big.NewInt(3), big.NewInt(8)},
		{"big values", setBigInt("99999999999999999999"), big.NewInt(1), setBigInt("100000000000000000000")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.add(tt.a, tt.b)
			assert.Equal(t, 0, result.Cmp(tt.expected), "expected %s, got %s", tt.expected, result)
		})
	}
}

func TestCalculator_Subtract(t *testing.T) {
	c := &calculator{}
	tests := []struct {
		name     string
		a, b     *big.Int
		expected *big.Int
	}{
		{"small values", big.NewInt(10), big.NewInt(3), big.NewInt(7)},
		{"big values", setBigInt("100000000000000000000"), big.NewInt(1), setBigInt("99999999999999999999")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.subtract(tt.a, tt.b)
			assert.Equal(t, 0, result.Cmp(tt.expected), "expected %s, got %s", tt.expected, result)
		})
	}
}

func TestCalculator_Multiply(t *testing.T) {
	c := &calculator{}
	tests := []struct {
		name     string
		a        *big.Int
		m        int64
		expected *big.Int
	}{
		{"small values", big.NewInt(5), 3, big.NewInt(15)},
		{"overflow values", setBigInt("99999999999999999999"), 2, setBigInt("199999999999999999998")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.multiply(tt.a, tt.m)
			assert.Equal(t, 0, result.Cmp(tt.expected), "expected %s, got %s", tt.expected, result)
		})
	}
}

func TestCalculator_Divide(t *testing.T) {
	c := &calculator{}
	tests := []struct {
		name     string
		a        *big.Int
		d        int64
		expected *big.Int
	}{
		{"small values", big.NewInt(10), 3, big.NewInt(3)},
		{"big values", setBigInt("100000000000000000000"), 10, setBigInt("10000000000000000000")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.divide(tt.a, tt.d)
			assert.Equal(t, 0, result.Cmp(tt.expected), "expected %s, got %s", tt.expected, result)
		})
	}
}

func TestCalculator_Modulus(t *testing.T) {
	c := &calculator{}
	tests := []struct {
		name     string
		a        *big.Int
		d        int64
		expected *big.Int
	}{
		{"small values", big.NewInt(10), 3, big.NewInt(1)},
		{"big values", setBigInt("100000000000000000001"), 3, big.NewInt(2)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.modulus(tt.a, tt.d)
			assert.Equal(t, 0, result.Cmp(tt.expected), "expected %s, got %s", tt.expected, result)
		})
	}
}

func TestCalculator_Allocate(t *testing.T) {
	c := &calculator{}
	tests := []struct {
		name     string
		a        *big.Int
		r, s     int64
		expected *big.Int
	}{
		{"normal allocation", big.NewInt(100), 50, 100, big.NewInt(50)},
		{"big values", setBigInt("100000000000000000000"), 50, 100, setBigInt("50000000000000000000")},
		{"zero amount", big.NewInt(0), 50, 100, big.NewInt(0)},
		{"zero sum", big.NewInt(100), 50, 0, big.NewInt(0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.allocate(tt.a, tt.r, tt.s)
			assert.Equal(t, 0, result.Cmp(tt.expected), "expected %s, got %s", tt.expected, result)
		})
	}
}

func TestCalculator_Absolute(t *testing.T) {
	c := &calculator{}
	tests := []struct {
		name     string
		a        *big.Int
		expected *big.Int
	}{
		{"negative to positive", big.NewInt(-5), big.NewInt(5)},
		{"positive stays positive", big.NewInt(5), big.NewInt(5)},
		{"big negative", setBigInt("-99999999999999999999"), setBigInt("99999999999999999999")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.absolute(tt.a)
			assert.Equal(t, 0, result.Cmp(tt.expected), "expected %s, got %s", tt.expected, result)
		})
	}
}

func TestCalculator_Negative(t *testing.T) {
	c := &calculator{}
	tests := []struct {
		name     string
		a        *big.Int
		expected *big.Int
	}{
		{"positive to negative", big.NewInt(5), big.NewInt(-5)},
		{"already negative stays negative", big.NewInt(-5), big.NewInt(-5)},
		{"zero stays zero", big.NewInt(0), big.NewInt(0)},
		{"big positive", setBigInt("99999999999999999999"), setBigInt("-99999999999999999999")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.negative(tt.a)
			assert.Equal(t, 0, result.Cmp(tt.expected), "expected %s, got %s", tt.expected, result)
		})
	}
}

func TestCalculator_Round(t *testing.T) {
	c := &calculator{}
	tests := []struct {
		name     string
		a        *big.Int
		exp      int
		expected *big.Int
	}{
		{"rounds up when remainder > half", big.NewInt(175), 2, big.NewInt(200)},
		{"rounds down when remainder < half", big.NewInt(125), 2, big.NewInt(100)},
		{"zero stays zero", big.NewInt(0), 2, big.NewInt(0)},
		{"big value rounds up", setBigInt("99999999999999999975"), 2, setBigInt("100000000000000000000")},
		{"negative rounds", big.NewInt(-175), 2, big.NewInt(-200)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.round(tt.a, tt.exp)
			assert.Equal(t, 0, result.Cmp(tt.expected), "expected %s, got %s", tt.expected, result)
		})
	}
}
