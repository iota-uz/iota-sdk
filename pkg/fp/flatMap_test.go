package fp

import (
	"reflect"
	"testing"
)

func TestFlatMap_Example(t *testing.T) {
	res := FlatMap(func(x int) []int { return []int{x, x} })([]int{1, 2})
	expected := []int{1, 1, 2, 2}

	if reflect.DeepEqual(res, expected) == false {
		t.Error("Should call a defined callback function on each element of an array. Then, flattens the result into a new array. Received:", res)
	}
}

func TestFlatMapWithIndex_Example(t *testing.T) {
	res := FlatMapWithIndex(func(x int, i int) []int { return []int{x + i, x + i} })([]int{1, 2})
	expected := []int{1, 1, 3, 3}

	if reflect.DeepEqual(res, expected) == false {
		t.Error("Should call a defined callback function on each element of an array. Then, flattens the result into a new array. Received:", res)
	}
}
func TestFlatMapWithSlice_Example(t *testing.T) {
	res := FlatMapWithSlice(func(x int, i int, xs []int) []int { return []int{x + i + xs[0], x + i + xs[0]} })([]int{1, 2})
	expected := []int{2, 2, 4, 4}

	if reflect.DeepEqual(res, expected) == false {
		t.Error("Should call a defined callback function on each element of an array. Then, flattens the result into a new array. Received:", res)
	}
}
