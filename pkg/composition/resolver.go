package composition

import (
	"fmt"
	"reflect"
)

// ResolveType resolves a provider by reflect.Type. Used by pkg/di to wire
// controller handler parameters at request time via the serviceResolver
// interface.
func (c *Container) ResolveType(t reflect.Type) (any, error) {
	if c == nil {
		return nil, fmt.Errorf("composition: container is nil")
	}
	return c.resolveAny(keyFor(t, ""))
}
