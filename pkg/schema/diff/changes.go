package diff

import "github.com/iota-uz/iota-sdk/pkg/schema/types"

type ChangeType string

const (
	CreateTable      ChangeType = "CREATE_TABLE"
	DropTable        ChangeType = "DROP_TABLE"
	AlterTable       ChangeType = "ALTER_TABLE"
	AddColumn        ChangeType = "ADD_COLUMN"
	DropColumn       ChangeType = "DROP_COLUMN"
	ModifyColumn     ChangeType = "MODIFY_COLUMN"
	AddConstraint    ChangeType = "ADD_CONSTRAINT"
	DropConstraint   ChangeType = "DROP_CONSTRAINT"
	ModifyConstraint ChangeType = "MODIFY_CONSTRAINT"
	AddIndex         ChangeType = "ADD_INDEX"
	DropIndex        ChangeType = "DROP_INDEX"
	ModifyIndex      ChangeType = "MODIFY_INDEX"
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
