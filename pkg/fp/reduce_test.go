package fp

import (
	"testing"
)

func TestReduce_Example(t *testing.T) {
	res := Reduce(func(acc int, curr int) int { return acc + curr }, 0)([]int{1, 2, 3})

	if res != 6 {
		t.Error("Reduce should call the specified callback function for all the elements in an array. The return value of the callback function should be the accumulated result, and is provided as an argument in the next call to the callback function. Received:", res)
	}
}

func TestReduceWithIndex_Example(t *testing.T) {
	res := ReduceWithIndex(func(acc int, curr int, i int) int { return acc + curr + i }, 0)([]int{1, 2, 3})

	if res != 9 {
		t.Error("Reduce should call the specified callback function for all the elements in an array. The return value of the callback function should be the accumulated result, and is provided as an argument in the next call to the callback function. Received:", res)
	}
}

func TestReduceWithSlice_Example(t *testing.T) {
	res := ReduceWithSlice(func(acc int, curr int, i int, xs []int) int { return acc + curr + i + xs[0] }, 0)([]int{1, 2, 3})

	if res != 12 {
		t.Error("Reduce should call the specified callback function for all the elements in an array. The return value of the callback function should be the accumulated result, and is provided as an argument in the next call to the callback function. Received:", res)
	}
}
