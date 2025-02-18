package types

// NodeType represents the type of a schema node
type NodeType string

const (
	NodeRoot       NodeType = "ROOT"
	NodeTable      NodeType = "TABLE"
	NodeColumn     NodeType = "COLUMN"
	NodeConstraint NodeType = "CONSTRAINT"
	NodeIndex      NodeType = "INDEX"
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
