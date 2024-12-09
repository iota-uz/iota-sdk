package mapping_test

import (
	"github.com/iota-agency/iota-sdk/pkg/mapping"
	"testing"
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
