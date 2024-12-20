package fp

import (
	"reflect"
	"testing"
)

func TestFilter(t *testing.T) {
	res := Filter(func(x int) bool { return x > 0 })([]int{-1, 2, -3, 4})
	expected := []int{2, 4}

	if reflect.DeepEqual(res, expected) == false {
		t.Error("Filter should return only elements for which the callback returns true. Received:", res)
	}
}

func TestFilterWithIndex(t *testing.T) {
	res := FilterWithIndex(func(x int, i int) bool { return x+i > 0 })([]int{-3, 2, -1, 4})
	expected := []int{2, -1, 4}

	if reflect.DeepEqual(res, expected) == false {
		t.Error("Filter should return only elements for which the callback returns true. Received:", res)
	}
}

func TestFilterWithSlice(t *testing.T) {
	res := FilterWithSlice(func(x int, i int, xs []int) bool { return x+i+xs[0] > 0 })([]int{2, -1, 4, -3})
	expected := []int{2, -1, 4, -3}

	if reflect.DeepEqual(res, expected) == false {
		t.Error("Filter should return only elements for which the callback returns true. Received:", res)
	}
}
