package fp

import (
	"reflect"
	"strconv"
	"testing"
)

func TestMap_ExampleTestMapWithIndex_Example(t *testing.T) {
	res := Map(func(x int64) string {
		return strconv.FormatInt(x, 10)
	})([]int64{1, 2, 3})
	expected := []string{"1", "2", "3"}

	if reflect.DeepEqual(res, expected) == false {
		t.Error("Should map int64 into string. Received:", res)
	}
}

func TestMapWithIndex_Example(t *testing.T) {
	res := MapWithIndex(
		func(x int64, i int) string { return strconv.FormatInt(x+int64(i), 10) })([]int64{1, 2, 3})
	expected := []string{"1", "3", "5"}

	if reflect.DeepEqual(res, expected) == false {
		t.Error("Should map int64 into string. Received:", res)
	}
}

func TestMapWithSlice_Example(t *testing.T) {
	res := MapWithSlice(
		func(x int64, i int, xs []int64) string { return strconv.FormatInt(x+int64(i)+xs[0], 10) })([]int64{1, 2, 3})
	expected := []string{"2", "4", "6"}

	if reflect.DeepEqual(res, expected) == false {
		t.Error("Should map int64 into string. Received:", res)
	}
}
