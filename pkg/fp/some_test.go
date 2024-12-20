package fp

import (
	"testing"
)

func TestSome_TrueExmple(t *testing.T) {
	res := Some(func(x int) bool { return x > 0 })([]int{-1, -2, 3})

	if res != true {
		t.Error("Some should return true as at least one element match the condition. Received:", res)
	}
}

func TestSome_FalseExmple(t *testing.T) {
	res := Some(func(x int) bool { return x < 0 })([]int{1, 2, 3})

	if res != false {
		t.Error("Some should return false as none of the element match the condition. Received:", res)
	}
}

func TestSomeWithIndex_TrueExmple(t *testing.T) {
	res := SomeWithIndex(func(x int, i int) bool { return x+i > 0 })([]int{-10, -20, -1})

	if res != true {
		t.Error("Some should return true as at least one element match the condition. Received:", res)
	}
}

func TestSomeWithIndex_FalseExmple(t *testing.T) {
	res := SomeWithIndex(func(x int, i int) bool { return x+i < 0 })([]int{1, 2, 3})

	if res != false {
		t.Error("Some should return false as none of the element match the condition. Received:", res)
	}
}

func TestSomeWithSlice_TrueExmple(t *testing.T) {
	res := SomeWithSlice(func(x int, i int, xs []int) bool { return x+i-xs[0] > 0 })([]int{-1, -20, -2})

	if res != true {
		t.Error("Some should return true as at least one element match the condition. Received:", res)
	}
}

func TestSomeWithSlice_FalseExmple(t *testing.T) {
	res := SomeWithSlice(func(x int, i int, xs []int) bool { return x+i+xs[0] < 0 })([]int{1, 2, 3, -4})

	if res != false {
		t.Error("Some should return false as none of the element match the condition. Received:", res)
	}
}
