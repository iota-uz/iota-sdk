package crud

import "github.com/iota-uz/iota-sdk/pkg/serrors"

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

// Relation defines a relationship between two schemas.
type Relation struct {
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

	// Schema is the related schema (type-erased for storage in slice)
	Schema any

	// EntityField is the name used for the EntityFieldValue in mapper (e.g., "vehicle_type_entity")
	EntityField string

	// Through specifies the parent alias for nested relations (e.g., "vt" for group through vehicle_type)
	Through string
}

// Validate checks that the relation has all required fields.
func (r *Relation) Validate() error {
	const op = serrors.Op("Relation.Validate")

	if r.Alias == "" {
		return serrors.E(op, serrors.Invalid, "alias is required")
	}
	if r.LocalKey == "" {
		return serrors.E(op, serrors.Invalid, "local key is required")
	}
	if r.EntityField == "" {
		return serrors.E(op, serrors.Invalid, "entity field is required")
	}
	// RemoteKey defaults to "id" if not specified
	if r.RemoteKey == "" {
		r.RemoteKey = "id"
	}
	// JoinType defaults to LEFT if not specified
	if r.JoinType == 0 {
		r.JoinType = JoinTypeLeft
	}

	return nil
}

// TableName extracts the table name from the related schema.
func (r *Relation) TableName() string {
	if r.Schema == nil {
		return ""
	}
	if namer, ok := r.Schema.(interface{ Name() string }); ok {
		return namer.Name()
	}
	return ""
}
