package types

// NodeType represents the type of AST node
type NodeType string

const (
	NodeRoot       NodeType = "ROOT"
	NodeTable      NodeType = "TABLE"
	NodeColumn     NodeType = "COLUMN"
	NodeConstraint NodeType = "CONSTRAINT"
	NodeIndex      NodeType = "INDEX"
)

// Position represents the location of a node in the source SQL
type Position struct {
	Line   int
	Column int
	File   string
}

// Node represents a single node in the schema AST
type Node struct {
	Type     NodeType
	Name     string
	Children []*Node
	Metadata map[string]interface{}
	Pos      Position
}

// SchemaTree represents the complete database schema
type SchemaTree struct {
	Root     *Node
	Version  string
	Metadata map[string]interface{}
}
