package collector

import (
	"github.com/iota-uz/iota-sdk/pkg/schema/common"
)

// NodeType represents the type of a schema node
type NodeType string

// Re-export node type constants from common package
const (
	NodeRoot       NodeType = NodeType(common.NodeRoot)
	NodeTable      NodeType = NodeType(common.NodeTable)
	NodeColumn     NodeType = NodeType(common.NodeColumn)
	NodeConstraint NodeType = NodeType(common.NodeConstraint)
	NodeIndex      NodeType = NodeType(common.NodeIndex)
)

// Node represents a node in the schema tree
type Node struct {
	Type     NodeType
	Name     string
	Children []*Node
	Metadata map[string]interface{}
}

// SchemaTree represents a complete database schema
type SchemaTree struct {
	Root     *Node
	Metadata map[string]interface{}
}

// NewSchemaTree creates a new schema tree instance
func NewSchemaTree() *SchemaTree {
	return &SchemaTree{
		Root: &Node{
			Type:     NodeRoot,
			Children: make([]*Node, 0),
			Metadata: make(map[string]interface{}),
		},
		Metadata: make(map[string]interface{}),
	}
}