package data

import (
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// RuleGrantsRows reports whether one read rule gives the subject rows: the
// subject holds the rule's permission (empty: everyone) and the rule is "all
// rows" or has a row filter. The query builder compiles exactly the rules
// this accepts; readers of the catalog (the reference, the OpenAPI document)
// use it to decide what a subject may see without running a query.
func RuleGrantsRows(sub spi.Subject, r manifest.ReadRule) bool {
	return sub.Has(r.Permission) && (r.All || r.Rows != nil)
}

// AccessAllowsRead reports whether read access a lets the subject see any
// row: no rules means every subject of the tenant, otherwise at least one
// rule must grant rows (native entities carry a single "all rows" rule on
// their read permission).
func AccessAllowsRead(sub spi.Subject, a *manifest.Access) bool {
	if a == nil || len(a.Read) == 0 {
		return true
	}
	for _, r := range a.Read {
		if RuleGrantsRows(sub, r) {
			return true
		}
	}
	return false
}

// FieldReadable reports whether the subject holds every permission the entity
// requires to read (and so to write) the field.
func FieldReadable(sub spi.Subject, a *manifest.Access, fieldID string) bool {
	if a == nil {
		return true
	}
	for _, p := range a.Fields[fieldID] {
		if !sub.Has(p) {
			return false
		}
	}
	return true
}

// EntityReadable reports whether the subject can read rows of the entity or
// overlay resource res, by the rules the data layer applies to queries: the
// resource's own read access, and for an overlay also that of its base (an
// overlay row is visible only with its base row). lookup resolves a base by
// resource ID; a base it cannot resolve denies.
func EntityReadable(sub spi.Subject, res manifest.Resource, lookup func(id string) (manifest.Resource, bool)) bool {
	return entityReadable(sub, res, lookup, 0)
}

// maxBaseDepth bounds the walk along base references; overlays of overlays
// are not a thing, the bound only keeps a malformed catalog from looping.
const maxBaseDepth = 8

func entityReadable(sub spi.Subject, res manifest.Resource, lookup func(string) (manifest.Resource, bool), depth int) bool {
	if !AccessAllowsRead(sub, res.Access) {
		return false
	}
	if res.Kind != manifest.ResOverlay || res.Base == "" {
		return true
	}
	if depth >= maxBaseDepth || lookup == nil {
		return false
	}
	base, ok := lookup(res.Base)
	if !ok {
		return false
	}
	return entityReadable(sub, base, lookup, depth+1)
}
