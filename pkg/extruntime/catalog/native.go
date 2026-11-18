// Package catalog implements spi.Catalog over the native (Go-registered)
// contracts and the manifests of activated package versions.
package catalog

import (
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
)

// NativeSet is the catalog's read view of the application's native.Set: the
// native resources it publishes next to package resources, grouped for the
// developer kit and the catalog snapshot. It adds nothing to the set.
type NativeSet struct {
	set *native.Set
}

// Natives is the catalog view of set (nil: no natives).
func Natives(set *native.Set) *NativeSet {
	if set == nil {
		set = native.NewSet()
	}
	return &NativeSet{set: set}
}

// NewNativeSet is the view of a set that holds only the given native entities
// (nil: none), for tools and tests that need no other native resource.
func NewNativeSet(entities *data.NativeRegistry) *NativeSet {
	set := native.NewSet()
	if entities != nil {
		for _, e := range entities.List() {
			if err := set.Add(native.Entity(e)); err != nil {
				// The registry validated every descriptor and its IDs are unique.
				panic(err)
			}
		}
	}
	return &NativeSet{set: set}
}

// Set is the underlying native set.
func (n *NativeSet) Set() *native.Set {
	if n == nil {
		return nil
	}
	return n.set
}

// Resources lists every native resource sorted by ID, with contract hashes.
func (n *NativeSet) Resources() []manifest.Resource {
	if n == nil {
		return nil
	}
	return n.set.Resources()
}

// Get returns one native resource.
func (n *NativeSet) Get(id string) (manifest.Resource, bool) {
	if n == nil {
		return manifest.Resource{}, false
	}
	return n.set.Resource(id)
}

// Namespace is the kit namespace of a native resource: the first ID segment
// ("core.user" -> "core").
func Namespace(id string) string {
	ns, _, _ := strings.Cut(id, ".")
	return ns
}

// Sources groups the native resources and lists by namespace for kit
// generation.
func (n *NativeSet) Sources() []build.KitSource {
	byNS := map[string][]manifest.Resource{}
	for _, r := range n.Resources() {
		ns := Namespace(r.ID)
		byNS[ns] = append(byNS[ns], r)
	}
	listsByNS := map[string][]string{}
	if n != nil {
		for _, l := range n.set.Lists() {
			ns := Namespace(l)
			listsByNS[ns] = append(listsByNS[ns], l)
			if _, ok := byNS[ns]; !ok {
				byNS[ns] = nil
			}
		}
	}
	out := make([]build.KitSource, 0, len(byNS))
	for ns, res := range byNS {
		out = append(out, build.KitSource{Namespace: ns, Resources: res, Lists: listsByNS[ns]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Namespace < out[j].Namespace })
	return out
}

// Reserved reports whether a package namespace lies under a native root
// ("core", "crm"): packages may not declare resources there.
func (n *NativeSet) Reserved(namespace string) bool {
	root := Namespace(namespace)
	for _, s := range n.Sources() {
		if s.Namespace == root {
			return true
		}
	}
	return false
}
