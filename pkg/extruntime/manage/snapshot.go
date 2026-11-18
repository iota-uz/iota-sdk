package manage

import (
	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// NativeSnapshot exports the native catalog of this application (SDK natives
// plus whatever the host registered) as the snapshot document that
// `iota-ext kit generate --catalog` reads offline.
func (s *Service) NativeSnapshot(actor spi.Subject) (catalog.Snapshot, error) {
	if err := require(actor, PermRead); err != nil {
		return catalog.Snapshot{}, err
	}
	return s.catalog.Natives().Snapshot(), nil
}
