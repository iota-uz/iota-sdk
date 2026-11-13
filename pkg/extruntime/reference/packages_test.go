package reference_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/reference"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

func vault() catalog.ActiveVersion {
	return catalog.ActiveVersion{
		VersionID: "v2", Namespace: "acme.vault", Version: "1.0.0", ArtifactHash: "h",
		Manifest: manifest.Manifest{
			Schema: manifest.SchemaVersion, Namespace: "acme.vault",
			Resources: []manifest.Resource{{
				ID: "acme.vault.open", Kind: manifest.ResCommand, Permission: "Vault.Use",
				Input: &manifest.Type{Kind: manifest.KindObject}, Output: &manifest.Type{Kind: manifest.KindObject},
			}},
		},
	}
}

// generations is a source whose active set and generation the test controls.
type generations struct {
	fakeSource
	gen uint64
}

func (g *generations) Generation() uint64 { return g.gen }

func namespacesOf(avs []catalog.ActiveVersion) []string {
	out := []string{}
	for _, av := range avs {
		out = append(out, av.Namespace)
	}
	return out
}

func TestVisiblePackagesFollowTheReferenceVisibilityRule(t *testing.T) {
	t.Parallel()
	src := &generations{fakeSource: fakeSource{active: []catalog.ActiveVersion{loyalty(), vault()}, natives: nativeSet(t)}, gen: 1}
	b := reference.New(src)
	ctx := context.Background()
	names := func(sub spi.Subject) []string {
		t.Helper()
		avs, err := b.VisiblePackages(ctx, sub)
		require.NoError(t, err)
		return namespacesOf(avs)
	}

	// Extensions.Read sees every package.
	require.Equal(t, []string{"acme.loyalty", "acme.vault"}, names(admin()))
	// Anyone else sees a package when one of its resources is visible to them:
	// the loyalty package has open resources, the vault only a gated command.
	require.Equal(t, []string{"acme.loyalty"}, names(reader("en")))
	require.Equal(t, []string{"acme.loyalty", "acme.vault"}, names(reader("en", "Vault.Use")))
	// A reader with another package's permission sees nothing of the vault.
	require.Equal(t, []string{"acme.loyalty"}, names(reader("en", "Loyalty.Manage")))

	// The result follows the catalog generation: deactivating the loyalty
	// package takes effect with the next generation, not before.
	src.active = []catalog.ActiveVersion{vault()}
	require.Equal(t, []string{"acme.loyalty"}, names(reader("en")), "same generation: the cached snapshot")
	src.gen = 2
	require.Equal(t, []string{}, names(reader("en")))
	require.Equal(t, []string{"acme.vault"}, names(admin()))
}

func TestPackageVisibleChecksEveryResource(t *testing.T) {
	t.Parallel()
	m := vault().Manifest
	none := func(string) (manifest.Resource, bool) { return manifest.Resource{}, false }
	require.False(t, reference.PackageVisible(reader("en"), m, none))
	require.True(t, reference.PackageVisible(reader("en", "Vault.Use"), m, none))
	require.True(t, reference.PackageVisible(admin(), m, none))
	m.Resources = append(m.Resources, manifest.Resource{ID: "acme.vault.guide", Kind: manifest.ResPermission})
	require.True(t, reference.PackageVisible(reader("en"), m, none), "one open resource opens the package")
	require.False(t, reference.PackageVisible(reader("en"), manifest.Manifest{}, none), "a package without resources is visible to Extensions.Read only")
	require.True(t, reference.PackageVisible(admin(), manifest.Manifest{}, none))
}
