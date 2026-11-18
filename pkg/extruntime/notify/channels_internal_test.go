package notify

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// A package may ship templates for the channels the manifest admits at
// publish, and the adapter must know every one of them under the same name.
func TestEveryManifestTemplateChannelIsAKnownChannel(t *testing.T) {
	t.Parallel()
	for _, name := range manifest.TemplateChannels {
		c, err := coreChannel(name)
		require.NoError(t, err, name)
		require.Equal(t, name, templateChannel(c), "templates are matched by the name the manifest carries")
	}
}
