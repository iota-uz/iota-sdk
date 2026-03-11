package money

import (
	"math/big"
	"testing"
)

func TestCalculator_Add(t *testing.T) {
	c := &calculator{}
	result := c.add(big.NewInt(5), big.NewInt(3))
	if result.Int64() != 8 {
		t.Errorf("Expected 8, got %d", result.Int64())
	}
}

func TestCalculator_Add_BigValues(t *testing.T) {
	c := &calculator{}
	a := new(big.Int)
	a.SetString("99999999999999999999", 10)
	b := big.NewInt(1)
	result := c.add(a, b)

	expected := new(big.Int)
	expected.SetString("100000000000000000000", 10)
	if result.Cmp(expected) != 0 {
		t.Errorf("Expected %s, got %s", expected.String(), result.String())
	}
}

func TestCalculator_Subtract(t *testing.T) {
	c := &calculator{}
	result := c.subtract(big.NewInt(10), big.NewInt(3))
	if result.Int64() != 7 {
		t.Errorf("Expected 7, got %d", result.Int64())
	}
}

func TestCalculator_Subtract_BigValues(t *testing.T) {
	c := &calculator{}
	a := new(big.Int)
	a.SetString("100000000000000000000", 10)
	b := big.NewInt(1)
	result := c.subtract(a, b)

	expected := new(big.Int)
	expected.SetString("99999999999999999999", 10)
	if result.Cmp(expected) != 0 {
		t.Errorf("Expected %s, got %s", expected.String(), result.String())
	}
}

func TestCalculator_Multiply(t *testing.T) {
	c := &calculator{}
	result := c.multiply(big.NewInt(5), 3)
	if result.Int64() != 15 {
		t.Errorf("Expected 15, got %d", result.Int64())
	}
}

func TestCalculator_Multiply_Overflow(t *testing.T) {
	c := &calculator{}
	a := new(big.Int)
	a.SetString("99999999999999999999", 10)
	result := c.multiply(a, 2)

	expected := new(big.Int)
	expected.SetString("199999999999999999998", 10)
	if result.Cmp(expected) != 0 {
		t.Errorf("Expected %s, got %s", expected.String(), result.String())
	}
}

func TestCalculator_Divide(t *testing.T) {
	c := &calculator{}
	result := c.divide(big.NewInt(10), 3)
	if result.Int64() != 3 {
		t.Errorf("Expected 3, got %d", result.Int64())
	}
}

func TestCalculator_Divide_BigValues(t *testing.T) {
	c := &calculator{}
	a := new(big.Int)
	a.SetString("100000000000000000000", 10)
	result := c.divide(a, 10)

	expected := new(big.Int)
	expected.SetString("10000000000000000000", 10)
	if result.Cmp(expected) != 0 {
		t.Errorf("Expected %s, got %s", expected.String(), result.String())
	}
}

func TestCalculator_Modulus(t *testing.T) {
	c := &calculator{}
	result := c.modulus(big.NewInt(10), 3)
	if result.Int64() != 1 {
		t.Errorf("Expected 1, got %d", result.Int64())
	}
}

func TestCalculator_Modulus_BigValues(t *testing.T) {
	c := &calculator{}
	a := new(big.Int)
	a.SetString("100000000000000000001", 10)
	result := c.modulus(a, 3)
	if result.Int64() != 2 {
		t.Errorf("Expected 2, got %d", result.Int64())
	}
}

func TestCalculator_Allocate(t *testing.T) {
	c := &calculator{}
	result := c.allocate(big.NewInt(100), 50, 100)
	if result.Int64() != 50 {
		t.Errorf("Expected 50, got %d", result.Int64())
	}
}

func TestCalculator_Allocate_BigValues(t *testing.T) {
	c := &calculator{}
	a := new(big.Int)
	a.SetString("100000000000000000000", 10)
	result := c.allocate(a, 50, 100)

	expected := new(big.Int)
	expected.SetString("50000000000000000000", 10)
	if result.Cmp(expected) != 0 {
		t.Errorf("Expected %s, got %s", expected.String(), result.String())
	}
}

func TestCalculator_Allocate_ZeroAmount(t *testing.T) {
	c := &calculator{}
	result := c.allocate(big.NewInt(0), 50, 100)
	if result.Int64() != 0 {
		t.Errorf("Expected 0, got %d", result.Int64())
	}
}

func TestCalculator_Allocate_ZeroSum(t *testing.T) {
	c := &calculator{}
	result := c.allocate(big.NewInt(100), 50, 0)
	if result.Int64() != 0 {
		t.Errorf("Expected 0, got %d", result.Int64())
	}
}

func TestCalculator_Absolute(t *testing.T) {
	c := &calculator{}
	result := c.absolute(big.NewInt(-5))
	if result.Int64() != 5 {
		t.Errorf("Expected 5, got %d", result.Int64())
	}

	result = c.absolute(big.NewInt(5))
	if result.Int64() != 5 {
		t.Errorf("Expected 5, got %d", result.Int64())
	}
}

func TestCalculator_Absolute_BigValues(t *testing.T) {
	c := &calculator{}
	a := new(big.Int)
	a.SetString("-99999999999999999999", 10)
	result := c.absolute(a)

	expected := new(big.Int)
	expected.SetString("99999999999999999999", 10)
	if result.Cmp(expected) != 0 {
		t.Errorf("Expected %s, got %s", expected.String(), result.String())
	}
}

func TestCalculator_Negative(t *testing.T) {
	c := &calculator{}
	result := c.negative(big.NewInt(5))
	if result.Int64() != -5 {
		t.Errorf("Expected -5, got %d", result.Int64())
	}

	// Already negative - should stay negative
	result = c.negative(big.NewInt(-5))
	if result.Int64() != -5 {
		t.Errorf("Expected -5, got %d", result.Int64())
	}

	// Zero stays zero
	result = c.negative(big.NewInt(0))
	if result.Int64() != 0 {
		t.Errorf("Expected 0, got %d", result.Int64())
	}
}

func TestCalculator_Negative_BigValues(t *testing.T) {
	c := &calculator{}
	a := new(big.Int)
	a.SetString("99999999999999999999", 10)
	result := c.negative(a)

	expected := new(big.Int)
	expected.SetString("-99999999999999999999", 10)
	if result.Cmp(expected) != 0 {
		t.Errorf("Expected %s, got %s", expected.String(), result.String())
	}
}

func TestCalculator_Round(t *testing.T) {
	c := &calculator{}
	// 175 rounded to exp=2 (100): 75 > 50, so rounds up to 200
	result := c.round(big.NewInt(175), 2)
	if result.Int64() != 200 {
		t.Errorf("Expected 200, got %d", result.Int64())
	}

	// 125 rounded to exp=2: 25 < 50, rounds down to 100
	result = c.round(big.NewInt(125), 2)
	if result.Int64() != 100 {
		t.Errorf("Expected 100, got %d", result.Int64())
	}

	// Zero stays zero
	result = c.round(big.NewInt(0), 2)
	if result.Int64() != 0 {
		t.Errorf("Expected 0, got %d", result.Int64())
	}
}

func TestCalculator_Round_BigValues(t *testing.T) {
	c := &calculator{}
	a := new(big.Int)
	a.SetString("99999999999999999975", 10) // last 2 digits: 75 > 50
	result := c.round(a, 2)

	expected := new(big.Int)
	expected.SetString("100000000000000000000", 10)
	if result.Cmp(expected) != 0 {
		t.Errorf("Expected %s, got %s", expected.String(), result.String())
	}
}

func TestCalculator_Round_NegativeBigValue(t *testing.T) {
	c := &calculator{}
	a := new(big.Int)
	a.SetString("-175", 10)
	result := c.round(a, 2)

	if result.Int64() != -200 {
		t.Errorf("Expected -200, got %d", result.Int64())
	}
}
