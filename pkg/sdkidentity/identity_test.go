package sdkidentity_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/sdkidentity"
)

func TestIdentityRequiresExactReleaseAndSourceCommit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*sdkidentity.Identity)
		wantErr bool
	}{
		{name: "valid identity"},
		{name: "incorrect release", mutate: func(identity *sdkidentity.Identity) { identity.ReleaseVersion = "0.4.44" }, wantErr: true},
		{name: "short commit", mutate: func(identity *sdkidentity.Identity) { identity.SourceCommit = "short" }, wantErr: true},
		{name: "non hexadecimal commit", mutate: func(identity *sdkidentity.Identity) {
			identity.SourceCommit = "z123456789abcdef0123456789abcdef01234567"
		}, wantErr: true},
		{name: "uppercase commit", mutate: func(identity *sdkidentity.Identity) {
			identity.SourceCommit = "A123456789abcdef0123456789abcdef01234567"
		}, wantErr: true},
		{name: "incorrect protocol", mutate: func(identity *sdkidentity.Identity) { identity.ProtocolVersion = "2.0.0" }, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			identity := sdkidentity.Identity{
				ReleaseVersion:  sdkidentity.ReleaseVersion,
				SourceCommit:    "0123456789abcdef0123456789abcdef01234567",
				ProtocolVersion: sdkidentity.ProtocolVersion,
			}
			if test.mutate != nil {
				require.NotNil(t, test.mutate)
				test.mutate(&identity)
			}
			if test.wantErr {
				assert.Error(t, identity.Validate())
				return
			}
			assert.NoError(t, identity.Validate())
		})
	}
}
