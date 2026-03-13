package twofactor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthMethodExternalValue_Scenario(t *testing.T) {
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
			require.Equal(t, tc.want, tc.got)
		})
	}
}
