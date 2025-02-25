package diff

import "github.com/iota-uz/iota-sdk/pkg/schema/types"

// ChangeType represents the type of schema change
type ChangeType string

const (
	// CreateTable represents a new table creation
	CreateTable ChangeType = "CREATE_TABLE"
	// DropTable represents a table deletion
	DropTable ChangeType = "DROP_TABLE"
	// DropColumn represents dropping a column
	DropColumn ChangeType = "DROP_COLUMN"
	// AddColumn represents adding a column to a table
	AddColumn ChangeType = "ADD_COLUMN"
	// ModifyColumn represents modifying an existing column
	ModifyColumn ChangeType = "MODIFY_COLUMN"
	// AddConstraint represents adding a constraint
	AddConstraint ChangeType = "ADD_CONSTRAINT"
	// DropConstraint represents dropping a constraint
	DropConstraint ChangeType = "DROP_CONSTRAINT"
	// AddIndex represents adding an index
	AddIndex ChangeType = "ADD_INDEX"
	// DropIndex represents dropping an index
	DropIndex ChangeType = "DROP_INDEX"
	// ModifyIndex represents modifying an index
	ModifyIndex ChangeType = "MODIFY_INDEX"
)

// Change represents a single schema change
type Change struct {
	Type         ChangeType
	Object       *types.Node
	ObjectName   string
	ParentName   string
	Statements   []string
	Reversible   bool
	Dependencies []string
	Metadata     map[string]interface{}
}

// ChangeSet represents a collection of related schema changes
type ChangeSet struct {
	Changes   []*Change
	Timestamp int64
	Version   string
	Hash      string
	Metadata  map[string]interface{}
}

func NewChangeSet() *ChangeSet {
	return &ChangeSet{
		Changes:  make([]*Change, 0),
		Metadata: make(map[string]interface{}),
	}
}
