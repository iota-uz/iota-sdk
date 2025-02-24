// Package ast provides types and functionality for SQL Abstract Syntax Trees
package ast

import "github.com/iota-uz/iota-sdk/pkg/schema/types"

// NewSchemaTree creates a new schema tree instance
func NewSchemaTree() *types.SchemaTree {
	return &types.SchemaTree{
		Root: &types.Node{
			Type:     types.NodeRoot,
			Children: make([]*types.Node, 0),
			Metadata: make(map[string]interface{}),
		},
		Metadata: make(map[string]interface{}),
	}
}
