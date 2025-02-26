package diff

import (
	"github.com/iota-uz/iota-sdk/pkg/schema/common"
)

// Import common types to maintain API compatibility
type ChangeType = common.ChangeType
type Schema = common.Schema
type SchemaObject = common.SchemaObject
type Change = common.Change
type ChangeSet = common.ChangeSet

// Re-export constants for backward compatibility
const (
	CreateTable    = common.CreateTable
	DropTable      = common.DropTable
	DropColumn     = common.DropColumn
	AddColumn      = common.AddColumn
	ModifyColumn   = common.ModifyColumn
	AddConstraint  = common.AddConstraint
	DropConstraint = common.DropConstraint
	AddIndex       = common.AddIndex
	DropIndex      = common.DropIndex
	ModifyIndex    = common.ModifyIndex
)

// NewSchema creates a new empty schema
func NewSchema() *Schema {
	return common.NewSchema()
}

// NewChangeSet creates a new empty change set
func NewChangeSet() *ChangeSet {
	return common.NewChangeSet()
}
