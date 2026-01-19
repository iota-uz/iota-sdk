package crud

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// RelationType defines the cardinality of a relation
type RelationType int

const (
	// BelongsTo represents a many-to-one relationship (e.g., Vehicle -> VehicleType)
	BelongsTo RelationType = iota
	// HasMany represents a one-to-many relationship (e.g., Person -> Documents)
	HasMany
)

// String returns the string representation of the relation type
func (rt RelationType) String() string {
	switch rt {
	case BelongsTo:
		return "belongs_to"
	case HasMany:
		return "has_many"
	default:
		return "unknown"
	}
}

// RelationEntityMapper maps child entity from prefixed fields.
type RelationEntityMapper interface {
	MapEntity(ctx context.Context, fields []FieldValue) (any, error)
}

// ManualRelation defines explicit table/columns for relations
// without a CRUD schema (e.g., legacy entities).
type ManualRelation struct {
	// Table is the explicit table name (e.g., "insurance.persons")
	Table string

	// Columns to select. Simple names auto-prefix with alias
	// ("id" -> "p.id AS p__id"). Expressions with "AS" used verbatim.
	Columns []string
}

// RelationDescriptor is a non-generic interface for relation metadata.
// Allows storing heterogeneous Relation[T] in a slice.
type RelationDescriptor interface {
	TableName() string
	Validate() error
	GetType() RelationType
	GetAlias() string
	GetLocalKey() string
	GetRemoteKey() string
	GetJoinType() JoinType
	GetEntityField() string
	GetThrough() string
	GetManual() *ManualRelation
	GetSchema() any
	GetMapper() any
	GetSetOnParent() func(parent, child any) any
}

// Relation defines a relationship between two schemas.
type Relation[T any] struct {
	// Type is the cardinality of the relation (BelongsTo or HasMany)
	Type RelationType

	// Alias is the prefix used for columns in SELECT (e.g., "vt" -> vt__id, vt__name)
	Alias string

	// LocalKey is the foreign key column in this table (e.g., "vehicle_type_id")
	LocalKey string

	// RemoteKey is the primary key column in the related table (e.g., "id")
	RemoteKey string

	// JoinType specifies LEFT or INNER JOIN (default: LEFT)
	JoinType JoinType

	// Schema is the related schema
	Schema Schema[T]

	// EntityField is the name used for the EntityFieldValue in mapper (e.g., "vehicle_type_entity")
	EntityField string

	// Through specifies the parent alias for nested relations (e.g., "vt" for group through vehicle_type)
	Through string

	// Mapper maps child entity from prefixed fields.
	Mapper RelationEntityMapper

	// SetOnParent attaches a mapped child entity to its parent entity.
	// Returns the updated parent. Used by MapWithRelations.
	// Example: func(parent, child any) any { return parent.ApplyOptions(WithChild(child)) }
	SetOnParent func(parent, child any) any

	// Manual overrides when Schema is nil (for legacy entities without CRUD schema)
	Manual *ManualRelation
}

// Validate checks that the relation has all required fields.
func (r *Relation[T]) Validate() error {
	const op = serrors.Op("Relation.Validate")

	if r.Alias == "" {
		return serrors.E(op, serrors.Invalid, "alias is required")
	}
	if r.LocalKey == "" {
		return serrors.E(op, serrors.Invalid, "local key is required")
	}

	// Must have either Schema or Manual
	if r.Schema == nil && r.Manual == nil {
		return serrors.E(op, serrors.Invalid, "either schema or manual config is required")
	}

	// Validate Manual config if provided
	if r.Manual != nil {
		if r.Manual.Table == "" {
			return serrors.E(op, serrors.Invalid, "manual relation requires table name")
		}
		if len(r.Manual.Columns) == 0 {
			return serrors.E(op, serrors.Invalid, "manual relation requires at least one column")
		}
	}

	// EntityField is required for schema-based relations
	if r.Schema != nil && r.EntityField == "" {
		return serrors.E(op, serrors.Invalid, "entity field is required for schema relations")
	}

	if r.RemoteKey == "" {
		r.RemoteKey = "id"
	}
	if !r.JoinType.IsValid() {
		r.JoinType = JoinTypeLeft
	}

	return nil
}

// TableName extracts the table name from the related schema or manual config.
func (r *Relation[T]) TableName() string {
	if r.Manual != nil && r.Manual.Table != "" {
		return r.Manual.Table
	}
	if r.Schema != nil {
		return r.Schema.Name()
	}
	return ""
}

func (r *Relation[T]) GetType() RelationType                       { return r.Type }
func (r *Relation[T]) GetAlias() string                            { return r.Alias }
func (r *Relation[T]) GetLocalKey() string                         { return r.LocalKey }
func (r *Relation[T]) GetRemoteKey() string                        { return r.RemoteKey }
func (r *Relation[T]) GetJoinType() JoinType                       { return r.JoinType }
func (r *Relation[T]) GetEntityField() string                      { return r.EntityField }
func (r *Relation[T]) GetThrough() string                          { return r.Through }
func (r *Relation[T]) GetManual() *ManualRelation                  { return r.Manual }
func (r *Relation[T]) GetSchema() any                              { return r.Schema }
func (r *Relation[T]) GetMapper() any                              { return r.Mapper }
func (r *Relation[T]) GetSetOnParent() func(parent, child any) any { return r.SetOnParent }

var _ RelationDescriptor = (*Relation[any])(nil)
