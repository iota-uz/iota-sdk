package analytics

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

// PermissionLogic defines how multiple permissions are evaluated.
type PermissionLogic int

const (
	// LogicAny means the user needs ANY ONE of the listed permissions (OR logic).
	LogicAny PermissionLogic = iota

	// LogicAll means the user needs ALL of the listed permissions (AND logic).
	LogicAll
)

// View defines a database view with its SQL definition and access control.
type View struct {
	// Name is the view name (without schema prefix).
	Name string

	// SQL is the SELECT body (the part after CREATE VIEW ... AS).
	SQL string

	// Schema is the target schema. Defaults to "analytics".
	Schema string

	// Required is the list of permissions needed to access this view.
	// If nil or empty, the view is considered public.
	Required []permission.Permission

	// Logic defines how multiple permissions are evaluated.
	Logic PermissionLogic

	// Description is an optional human-readable description.
	Description string

	// ColumnComments maps column names to their descriptions (COMMENT ON COLUMN).
	ColumnComments map[string]string
}
