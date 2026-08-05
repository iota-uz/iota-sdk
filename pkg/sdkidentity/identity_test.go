package sdkidentity_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/sdkidentity"
)

func TestIdentityRequiresExactReleaseAndSourceCommit(t *testing.T) {
	t.Parallel()
	valid := sdkidentity.Identity{
		ReleaseVersion:  sdkidentity.ReleaseVersion,
		SourceCommit:    "0123456789abcdef0123456789abcdef01234567",
		ProtocolVersion: sdkidentity.ProtocolVersion,
	}
	require.NoError(t, valid.Validate())

	wrongRelease := valid
	wrongRelease.ReleaseVersion = "0.4.44"
	require.Error(t, wrongRelease.Validate())

	wrongCommit := valid
	wrongCommit.SourceCommit = "short"
	require.Error(t, wrongCommit.Validate())

	wrongProtocol := valid
	wrongProtocol.ProtocolVersion = "2.0.0"
	require.Error(t, wrongProtocol.Validate())
}
