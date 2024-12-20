package fp

import "testing"

func add1(x int) int {
	return x + 1
}

func double(x int) int {
	return x * 2
}

func TestCompose2_Example(t *testing.T) {
	res := Compose2(add1, double)(0)
	if res != 1 {
		t.Error("Should perform right-to-left function composition of 2 functions. Received:", res)
	}
}

func TestCompose3_Example(t *testing.T) {
	res := Compose3(add1, double, add1)(0)
	if res != 3 {
		t.Error("Should perform right-to-left function composition of 3 functions. Received:", res)
	}
}

func TestCompose4_Example(t *testing.T) {
	res := Compose4(add1, double, add1, double)(0)
	if res != 3 {
		t.Error("Should perform right-to-left function composition of 4 functions. Received:", res)
	}
}

func TestCompose5_Example(t *testing.T) {
	res := Compose5(add1, double, add1, double, add1)(0)
	if res != 7 {
		t.Error("Should perform right-to-left function composition of 5 functions. Received:", res)
	}
}

func TestCompose6_Example(t *testing.T) {
	res := Compose6(add1, double, add1, double, add1, double)(0)
	if res != 7 {
		t.Error("Should perform right-to-left function composition of 6 functions. Received:", res)
	}
}

func TestCompose7_Example(t *testing.T) {
	res := Compose7(add1, double, add1, double, add1, double, add1)(0)
	if res != 15 {
		t.Error("Should perform right-to-left function composition of 7 functions. Received:", res)
	}
}

func TestCompose8_Example(t *testing.T) {
	res := Compose8(add1, double, add1, double, add1, double, add1, double)(0)
	if res != 15 {
		t.Error("Should perform right-to-left function composition of 8 functions. Received:", res)
	}
}

func TestCompose9_Example(t *testing.T) {
	res := Compose9(add1, double, add1, double, add1, double, add1, double, add1)(0)
	if res != 31 {
		t.Error("Should perform right-to-left function composition of 9 functions. Received:", res)
	}
}

func TestCompose10_Example(t *testing.T) {
	res := Compose10(add1, double, add1, double, add1, double, add1, double, add1, double)(0)
	if res != 31 {
		t.Error("Should perform right-to-left function composition of 10 functions. Received:", res)
	}
}

func TestCompose11_Example(t *testing.T) {
	res := Compose11(add1, double, add1, double, add1, double, add1, double, add1, double, add1)(0)
	if res != 63 {
		t.Error("Should perform right-to-left function composition of 11 functions. Received:", res)
	}
}

func TestCompose12_Example(t *testing.T) {
	res := Compose12(add1, double, add1, double, add1, double, add1, double, add1, double, add1, double)(0)
	if res != 63 {
		t.Error("Should perform right-to-left function composition of 12 functions. Received:", res)
	}
}

func TestCompose13_Example(t *testing.T) {
	res := Compose13(add1, double, add1, double, add1, double, add1, double, add1, double, add1, double, add1)(0)
	if res != 127 {
		t.Error("Should perform right-to-left function composition of 13 functions. Received:", res)
	}
}

func TestCompose14_Example(t *testing.T) {
	res := Compose14(add1, double, add1, double, add1, double, add1, double, add1, double, add1, double, add1, double)(0)
	if res != 127 {
		t.Error("Should perform right-to-left function composition of 14 functions. Received:", res)
	}
}

func TestCompose15_Example(t *testing.T) {
	res := Compose15(add1, double, add1, double, add1, double, add1, double, add1, double, add1, double, add1, double, add1)(0)
	if res != 255 {
		t.Error("Should perform right-to-left function composition of 15 functions. Received:", res)
	}
}

func TestCompose16_Example(t *testing.T) {
	res := Compose16(add1, double, add1, double, add1, double, add1, double, add1, double, add1, double, add1, double, add1, double)(0)
	if res != 255 {
		t.Error("Should perform right-to-left function composition of 16 functions. Received:", res)
	}
}
