package mapping_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/stretchr/testify/assert"
)

func TestValue(t *testing.T) {
	type args struct {
		v *int
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "Test Value",
			args: args{
				v: nil,
			},
			want: 0,
		},
		{
			name: "Test Value",
			args: args{
				v: mapping.Pointer(1),
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mapping.Value(tt.args.v); got != tt.want {
				t.Errorf("Value() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToInterfaceSlice(t *testing.T) {
	t.Run("Float64 slice", func(t *testing.T) {
		input := []float64{1.5, 2.3, 3.7}
		expected := []interface{}{1.5, 2.3, 3.7}
		result := mapping.ToInterfaceSlice(input)
		assert.Equal(t, expected, result)
	})

	t.Run("Int slice", func(t *testing.T) {
		input := []int{1, 2, 3}
		expected := []interface{}{1, 2, 3}
		result := mapping.ToInterfaceSlice(input)
		assert.Equal(t, expected, result)
	})

	t.Run("String slice", func(t *testing.T) {
		input := []string{"a", "b", "c"}
		expected := []interface{}{"a", "b", "c"}
		result := mapping.ToInterfaceSlice(input)
		assert.Equal(t, expected, result)
	})

	t.Run("Empty int slice", func(t *testing.T) {
		input := []int{}
		expected := []interface{}{}
		result := mapping.ToInterfaceSlice(input)
		assert.Equal(t, expected, result)
	})

	t.Run("Single element", func(t *testing.T) {
		input := []float64{42.0}
		expected := []interface{}{42.0}
		result := mapping.ToInterfaceSlice(input)
		assert.Equal(t, expected, result)
	})
}
