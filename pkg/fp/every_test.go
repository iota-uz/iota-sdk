package fp

import (
	"testing"
)

func TestEvery_TrueExample(t *testing.T) {
	res := Every(func(x int) bool { return x > 0 })([]int{1, 2, 3})

	if res != true {
		t.Error("Every should return true as all elements match the condition. Received:", res)
	}
}

func TestEvery_FalseExample(t *testing.T) {
	res := Every(func(x int) bool { return x < 0 })([]int{-1, -2, 3})

	if res != false {
		t.Error("Every should return false if at least one element does not match the condition. Received:", res)
	}
}

func TestEveryWithIndex_TrueExample(t *testing.T) {
	res := EveryWithIndex(func(x int, i int) bool { return x+i > 0 })([]int{1, 2, -1})

	if res != true {
		t.Error("Every should return true as all elements match the condition. Received:", res)
	}
}

func TestEveryWithIndex_FalseExample(t *testing.T) {
	res := EveryWithIndex(func(x int, i int) bool { return x+i < 0 })([]int{-1, -2, 3})

	if res != false {
		t.Error("Every should return false if at least one element does not match the condition. Received:", res)
	}
}

func TestEveryWithSlice_TrueExample(t *testing.T) {
	res := EveryWithSlice(func(x int, i int, xs []int) bool { return x+i+xs[0] > 0 })([]int{1, 2, -2})

	if res != true {
		t.Error("Every should return true as all elements match the condition. Received:", res)
	}
}

func TestEveryWithSlice_FalseExample(t *testing.T) {
	res := EveryWithSlice(func(x int, i int, xs []int) bool { return x+i+xs[0] < 0 })([]int{-1, -2, 3})

	if res != false {
		t.Error("Every should return false if at least one element does not match the condition. Received:", res)
	}
}
