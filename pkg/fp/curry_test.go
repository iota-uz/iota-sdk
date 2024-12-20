package fp

import (
	"testing"
)

func TestCurry2_Example(t *testing.T) {
	function := func(t1, t2 int) int { return t1 + t2 }
	curriedF := Curry2(function)

	res := curriedF(1)(2)

	if res != 3 {
		t.Error("Should allow to curry a sum. Received:", res)
	}
}

func TestCurry3_Example(t *testing.T) {
	function := func(t1, t2, t3 int) int { return t1 + t2 + t3 }
	curriedF := Curry3(function)

	res := curriedF(1)(2)(3)

	if res != 6 {
		t.Error("Should allow to curry a sum. Received:", res)
	}
}

func TestCurry4_Example(t *testing.T) {
	function := func(t1, t2, t3, t4 int) int { return t1 + t2 + t3 + t4 }
	curriedF := Curry4(function)

	res := curriedF(1)(2)(3)(4)

	if res != 10 {
		t.Error("Should allow to curry a sum. Received:", res)
	}
}

func TestCurry5_Example(t *testing.T) {
	function := func(t1, t2, t3, t4, t5 int) int { return t1 + t2 + t3 + t4 + t5 }
	curriedF := Curry5(function)

	res := curriedF(1)(2)(3)(4)(5)

	if res != 15 {
		t.Error("Should allow to curry a sum. Received:", res)
	}
}

func TestCurry6_Example(t *testing.T) {
	function := func(t1, t2, t3, t4, t5, t6 int) int { return t1 + t2 + t3 + t4 + t5 + t6 }
	curriedF := Curry6(function)

	res := curriedF(1)(2)(3)(4)(5)(6)

	if res != 21 {
		t.Error("Should allow to curry a sum. Received:", res)
	}
}

func TestCurry7_Example(t *testing.T) {
	function := func(t1, t2, t3, t4, t5, t6, t7 int) int { return t1 + t2 + t3 + t4 + t5 + t6 + t7 }
	curriedF := Curry7(function)

	res := curriedF(1)(2)(3)(4)(5)(6)(7)

	if res != 28 {
		t.Error("Should allow to curry a sum. Received:", res)
	}
}

func TestCurry8_Example(t *testing.T) {
	function := func(t1, t2, t3, t4, t5, t6, t7, t8 int) int { return t1 + t2 + t3 + t4 + t5 + t6 + t7 + t8 }
	curriedF := Curry8(function)

	res := curriedF(1)(2)(3)(4)(5)(6)(7)(8)

	if res != 36 {
		t.Error("Should allow to curry a sum. Received:", res)
	}
}

func TestCurry9_Example(t *testing.T) {
	function := func(t1, t2, t3, t4, t5, t6, t7, t8, t9 int) int { return t1 + t2 + t3 + t4 + t5 + t6 + t7 + t8 + t9 }
	curriedF := Curry9(function)

	res := curriedF(1)(2)(3)(4)(5)(6)(7)(8)(9)

	if res != 45 {
		t.Error("Should allow to curry a sum. Received:", res)
	}
}

func TestCurry10_Example(t *testing.T) {
	function := func(t1, t2, t3, t4, t5, t6, t7, t8, t9, t10 int) int {
		return t1 + t2 + t3 + t4 + t5 + t6 + t7 + t8 + t9 + t10
	}
	curriedF := Curry10(function)

	res := curriedF(1)(2)(3)(4)(5)(6)(7)(8)(9)(10)

	if res != 55 {
		t.Error("Should allow to curry a sum. Received:", res)
	}
}

func TestCurry11_Example(t *testing.T) {
	function := func(t1, t2, t3, t4, t5, t6, t7, t8, t9, t10, t11 int) int {
		return t1 + t2 + t3 + t4 + t5 + t6 + t7 + t8 + t9 + t10 + t11
	}
	curriedF := Curry11(function)

	res := curriedF(1)(2)(3)(4)(5)(6)(7)(8)(9)(10)(11)

	if res != 66 {
		t.Error("Should allow to curry a sum. Received:", res)
	}
}

func TestCurry12_Example(t *testing.T) {
	function := func(t1, t2, t3, t4, t5, t6, t7, t8, t9, t10, t11, t12 int) int {
		return t1 + t2 + t3 + t4 + t5 + t6 + t7 + t8 + t9 + t10 + t11 + t12
	}
	curriedF := Curry12(function)

	res := curriedF(1)(2)(3)(4)(5)(6)(7)(8)(9)(10)(11)(12)

	if res != 78 {
		t.Error("Should allow to curry a sum. Received:", res)
	}
}

func TestCurry13_Example(t *testing.T) {
	function := func(t1, t2, t3, t4, t5, t6, t7, t8, t9, t10, t11, t12, t13 int) int {
		return t1 + t2 + t3 + t4 + t5 + t6 + t7 + t8 + t9 + t10 + t11 + t12 + t13
	}
	curriedF := Curry13(function)

	res := curriedF(1)(2)(3)(4)(5)(6)(7)(8)(9)(10)(11)(12)(13)

	if res != 91 {
		t.Error("Should allow to curry a sum. Received:", res)
	}
}

func TestCurry14_Example(t *testing.T) {
	function := func(t1, t2, t3, t4, t5, t6, t7, t8, t9, t10, t11, t12, t13, t14 int) int {
		return t1 + t2 + t3 + t4 + t5 + t6 + t7 + t8 + t9 + t10 + t11 + t12 + t13 + t14
	}
	curriedF := Curry14(function)

	res := curriedF(1)(2)(3)(4)(5)(6)(7)(8)(9)(10)(11)(12)(13)(14)

	if res != 105 {
		t.Error("Should allow to curry a sum. Received:", res)
	}
}

func TestCurry15_Example(t *testing.T) {
	function := func(t1, t2, t3, t4, t5, t6, t7, t8, t9, t10, t11, t12, t13, t14, t15 int) int {
		return t1 + t2 + t3 + t4 + t5 + t6 + t7 + t8 + t9 + t10 + t11 + t12 + t13 + t14 + t15
	}
	curriedF := Curry15(function)

	res := curriedF(1)(2)(3)(4)(5)(6)(7)(8)(9)(10)(11)(12)(13)(14)(15)

	if res != 120 {
		t.Error("Should allow to curry a sum. Received:", res)
	}
}

func TestCurry16_Example(t *testing.T) {
	function := func(t1, t2, t3, t4, t5, t6, t7, t8, t9, t10, t11, t12, t13, t14, t15, t16 int) int {
		return t1 + t2 + t3 + t4 + t5 + t6 + t7 + t8 + t9 + t10 + t11 + t12 + t13 + t14 + t15 + t16
	}
	curriedF := Curry16(function)

	res := curriedF(1)(2)(3)(4)(5)(6)(7)(8)(9)(10)(11)(12)(13)(14)(15)(16)

	if res != 136 {
		t.Error("Should allow to curry a sum. Received:", res)
	}
}
