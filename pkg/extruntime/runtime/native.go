package runtime

import "github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"

// nativeResource wraps a manifest.Resource so Config can carry native
// contracts registered by Go modules.
type nativeResource struct{ res manifest.Resource }

// Native declares a native contract (command, operation, query, event, page
// or slot) published in the catalog and the developer kit.
func Native(r manifest.Resource) NativeResource { return NativeResource{res: r} }

// Resource returns the contract.
func (n nativeResource) Resource() manifest.Resource { return n.res }
