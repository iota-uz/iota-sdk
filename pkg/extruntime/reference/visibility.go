package reference

import (
	corePerms "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// PermRead is the core permission (Extensions.Read) that shows a reader the
// whole reference; it is the permission that opens the configurator.
var PermRead = corePerms.ExtensionsRead.Name()

// CanReadAll reports whether the reader sees everything: it holds
// Extensions.Read.
func CanReadAll(sub spi.Subject) bool { return sub.Has(PermRead) }

// Lookup resolves a resource by ID (the base of an overlay).
type Lookup func(id string) (manifest.Resource, bool)

// Visible is the visibility rule of the reference, for one resource:
//
//   - a reader holding Extensions.Read sees every resource;
//   - any other reader sees a resource whose Permission is empty or held (the
//     rule the host applies to every call of the resource);
//   - an entity or overlay is shown only if the reader may read its rows: the
//     rule the data layer applies to queries (data.EntityReadable), including
//     the base of an overlay.
//
// Everything else the reference shows is derived from visible resources
// (see EndpointVisible for endpoints).
func Visible(sub spi.Subject, r manifest.Resource, lookup Lookup) bool {
	if CanReadAll(sub) {
		return true
	}
	if !sub.Has(r.Permission) {
		return false
	}
	if r.Kind == manifest.ResEntity || r.Kind == manifest.ResOverlay {
		return data.EntityReadable(sub, r, lookup)
	}
	return true
}

// FieldVisible reports whether a field of a visible entity is shown: a reader
// without Extensions.Read does not see fields whose read permissions it lacks
// (it could neither read nor write them).
func FieldVisible(sub spi.Subject, r manifest.Resource, fieldID string) bool {
	return CanReadAll(sub) || data.FieldReadable(sub, r.Access, fieldID)
}

// EndpointVisible reports whether an endpoint is shown: a public endpoint is
// open to anyone, any other runs as the signed-in user, so every signed-in
// reader could call it; Extensions.Read sees all.
func EndpointVisible(sub spi.Subject, e EndpointDoc) bool {
	return CanReadAll(sub) || e.Public || sub.UserID != 0
}
