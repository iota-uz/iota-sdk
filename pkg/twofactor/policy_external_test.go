package twofactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthMethodExternalValue(t *testing.T) {
	tests := []struct {
		name string
		got  AuthMethod
		want AuthMethod
	}{
		{
			name: "external method value",
			got:  AuthMethodExternal,
			want: AuthMethod("external"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.got)
		})
	}
}
