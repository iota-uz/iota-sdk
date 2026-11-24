package manager

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSameBirth(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		a, b int64
		want bool
	}{
		{name: "equal", a: 10, b: 10, want: true},
		{name: "recreated directory with a reused inode", a: 10, b: 11, want: false},
		{name: "record without a birth time", a: 0, b: 11, want: true},
		{name: "file system without birth times", a: 10, b: 0, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, sameBirth(tc.a, tc.b))
		})
	}
}
