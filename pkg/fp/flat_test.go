package fp

import (
	"reflect"
	"testing"
)

func TestFlat_Example(t *testing.T) {
	res := Flat([][]int{{1, 2}, {3, 4}})
	expected := []int{1, 2, 3, 4}

	if reflect.DeepEqual(res, expected) == false {
		t.Error("Should return a new array with all sub-array elements concatenated into it recursively up to the specified depth. Received:", res)
	}
}
