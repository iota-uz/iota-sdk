package reference

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// PackageVisible is the visibility rule of a package as a whole, which is
// what its end-user documentation follows: a reader holding Extensions.Read
// sees every package; any other reader sees a package if it sees at least one
// of its resources under Visible. lookup resolves any resource of the catalog
// (an overlay's base may be native or another package's).
func PackageVisible(sub spi.Subject, m manifest.Manifest, lookup Lookup) bool {
	if CanReadAll(sub) {
		return true
	}
	for _, r := range m.Resources {
		if Visible(sub, r, lookup) {
			return true
		}
	}
	return false
}

// VisiblePackages lists the activated package versions the reader may see
// (PackageVisible), ordered by namespace. It follows the catalog generation
// like Model: activating or deactivating a package changes the next result.
func (b *Builder) VisiblePackages(ctx context.Context, sub spi.Subject) ([]catalog.ActiveVersion, error) {
	const op = serrors.Op("extruntime.reference.Builder.VisiblePackages")
	f, err := b.snapshot(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	lookup := func(id string) (manifest.Resource, bool) {
		r, ok := f.resources[id]
		return r, ok
	}
	var out []catalog.ActiveVersion
	for _, av := range f.active {
		if PackageVisible(sub, av.Manifest, lookup) {
			out = append(out, av)
		}
	}
	return out, nil
}
