package fp

import "testing"

func TestPipe2_Example(t *testing.T) {
	res := Pipe2(add1, double)(0)
	if res != 2 {
		t.Error("Should perform left-to-right function composition of 2 functions. Received:", res)
	}
}

func TestPipe3_Example(t *testing.T) {
	res := Pipe3(add1, double, add1)(0)
	if res != 3 {
		t.Error("Should perform left-to-right function composition of 3 functions. Received:", res)
	}
}

func TestPipe4_Example(t *testing.T) {
	res := Pipe4(add1, double, add1, double)(0)
	if res != 6 {
		t.Error("Should perform left-to-right function composition of 4 functions. Received:", res)
	}
}

func TestPipe5_Example(t *testing.T) {
	res := Pipe5(add1, double, add1, double, add1)(0)
	if res != 7 {
		t.Error("Should perform left-to-right function composition of 5 functions. Received:", res)
	}
}

func TestPipe6_Example(t *testing.T) {
	res := Pipe6(add1, double, add1, double, add1, double)(0)
	if res != 14 {
		t.Error("Should perform left-to-right function composition of 6 functions. Received:", res)
	}
}

func TestPipe7_Example(t *testing.T) {
	res := Pipe7(add1, double, add1, double, add1, double, add1)(0)
	if res != 15 {
		t.Error("Should perform left-to-right function composition of 7 functions. Received:", res)
	}
}

func TestPipe8_Example(t *testing.T) {
	res := Pipe8(add1, double, add1, double, add1, double, add1, double)(0)
	if res != 30 {
		t.Error("Should perform left-to-right function composition of 8 functions. Received:", res)
	}
}

func TestPipe9_Example(t *testing.T) {
	res := Pipe9(add1, double, add1, double, add1, double, add1, double, add1)(0)
	if res != 31 {
		t.Error("Should perform left-to-right function composition of 9 functions. Received:", res)
	}
}

func TestPipe10_Example(t *testing.T) {
	res := Pipe10(add1, double, add1, double, add1, double, add1, double, add1, double)(0)
	if res != 62 {
		t.Error("Should perform left-to-right function composition of 10 functions. Received:", res)
	}
}

func TestPipe11_Example(t *testing.T) {
	res := Pipe11(add1, double, add1, double, add1, double, add1, double, add1, double, add1)(0)
	if res != 63 {
		t.Error("Should perform left-to-right function composition of 11 functions. Received:", res)
	}
}

func TestPipe12_Example(t *testing.T) {
	res := Pipe12(add1, double, add1, double, add1, double, add1, double, add1, double, add1, double)(0)
	if res != 126 {
		t.Error("Should perform left-to-right function composition of 12 functions. Received:", res)
	}
}

func TestPipe13_Example(t *testing.T) {
	res := Pipe13(add1, double, add1, double, add1, double, add1, double, add1, double, add1, double, add1)(0)
	if res != 127 {
		t.Error("Should perform left-to-right function composition of 13 functions. Received:", res)
	}
}

func TestPipe14_Example(t *testing.T) {
	res := Pipe14(add1, double, add1, double, add1, double, add1, double, add1, double, add1, double, add1, double)(0)
	if res != 254 {
		t.Error("Should perform left-to-right function composition of 14 functions. Received:", res)
	}
}

func TestPipe15_Example(t *testing.T) {
	res := Pipe15(add1, double, add1, double, add1, double, add1, double, add1, double, add1, double, add1, double, add1)(0)
	if res != 255 {
		t.Error("Should perform left-to-right function composition of 15 functions. Received:", res)
	}
}

func TestPipe16_Example(t *testing.T) {
	res := Pipe16(add1, double, add1, double, add1, double, add1, double, add1, double, add1, double, add1, double, add1, double)(0)
	if res != 510 {
		t.Error("Should perform left-to-right function composition of 16 functions. Received:", res)
	}
}
