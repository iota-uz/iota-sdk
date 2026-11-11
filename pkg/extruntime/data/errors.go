package data

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Sentinel errors. Use errors.Is to classify; the returned errors carry a
// contextual message and the serrors code of the sentinel.
var (
	// ErrNativeWrite is returned by generic Insert/Update/Delete/PutOverlay on
	// a native (core/Granite) entity. Native writes go through published
	// operations.
	ErrNativeWrite = serrors.New(serrors.FailedPrecondition, "native entities are read-only through generic data calls")
	// ErrForeignWrite is returned when package code writes another package's
	// entity or overlay through generic data calls; cross-package writes go
	// through the owner's published operations, like native ones.
	ErrForeignWrite = serrors.New(serrors.FailedPrecondition, "entities of another package are read-only through generic data calls")
	// ErrUnknownEntity: the resource is not a known entity or overlay.
	ErrUnknownEntity = serrors.New(serrors.NotFound, "unknown entity")
	// ErrNotFound: the addressed record does not exist (or is not visible).
	ErrNotFound = serrors.New(serrors.NotFound, "record not found")
	// ErrVersionConflict: optimistic row_version check failed.
	ErrVersionConflict = serrors.New(serrors.Conflict, "record was changed concurrently")
	// ErrFieldNotAccessible: a hidden field was used in a filter or order.
	ErrFieldNotAccessible = serrors.New(serrors.PermissionDenied, "field is not accessible")
	// ErrUnknownField: a field is not declared by the entity.
	ErrUnknownField = serrors.New(serrors.Invalid, "unknown field")
	// ErrInvalidValue: a value does not match the declared wire type.
	ErrInvalidValue = serrors.New(serrors.Invalid, "invalid value")
	// ErrInvalidQuery: a filter, order or paging argument is malformed.
	ErrInvalidQuery = serrors.New(serrors.Invalid, "invalid query")
	// ErrUnsupported: the operation does not apply to this kind of resource.
	ErrUnsupported = serrors.New(serrors.FailedPrecondition, "operation not supported for this resource")
	// ErrDenied: the subject lacks the permission to read the resource.
	ErrDenied = serrors.New(serrors.PermissionDenied, "permission denied")
	// ErrUniqueViolation: a unique field already holds the value.
	ErrUniqueViolation = serrors.New(serrors.AlreadyExists, "unique constraint violated")
	// ErrReferenceViolation: a ref points to a missing record, or a record
	// being deleted is still referenced.
	ErrReferenceViolation = serrors.New(serrors.FailedPrecondition, "reference constraint violated")
	// ErrIncompatibleSchema: the manifest cannot be mapped to tables, or the
	// change from the applied schema needs an explicit reviewed migration.
	ErrIncompatibleSchema = serrors.New(serrors.FailedPrecondition, "incompatible schema")
	// ErrStalePlan: the applied schema changed between Plan and Apply.
	ErrStalePlan = serrors.New(serrors.Conflict, "schema plan is stale")
)

func fail(op serrors.Op, sentinel *serrors.Error, format string, args ...any) error {
	return serrors.New(serrors.CodeOf(sentinel), fmt.Sprintf(format, args...)).WithOp(op).WithCause(sentinel)
}

// Diagnostic is one precise reason a schema was refused.
type Diagnostic struct {
	Resource string `json:"resource"`
	Field    string `json:"field,omitempty"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

func (d Diagnostic) String() string {
	loc := d.Resource
	if d.Field != "" {
		loc += "." + d.Field
	}
	return fmt.Sprintf("%s [%s]: %s", loc, d.Code, d.Message)
}

// Diagnostic codes of SchemaError.
const (
	DiagInvalidResource     = "invalid_resource"
	DiagInvalidField        = "invalid_field"
	DiagUnresolvedRef       = "unresolved_ref"
	DiagTypeChange          = "type_change"
	DiagFieldDropped        = "field_dropped"
	DiagEntityDropped       = "entity_dropped"
	DiagRequiredNoDefault   = "required_without_default"
	DiagTightenNullability  = "optional_to_required"
	DiagNarrowing           = "narrowing"
	DiagStructureChange     = "structure_change"
	DiagAmbiguousRename     = "ambiguous_rename"
	DiagUniqueViolatedByRow = "unique_violated_by_data"
	DiagIncompatiblePinned  = "pinned_incompatible"
)

// SchemaError lists every reason a manifest cannot be compiled or a change
// cannot be applied automatically. It matches ErrIncompatibleSchema.
type SchemaError struct {
	Diagnostics []Diagnostic
}

func (e *SchemaError) Error() string {
	parts := make([]string, len(e.Diagnostics))
	for i, d := range e.Diagnostics {
		parts[i] = d.String()
	}
	return "incompatible schema: " + strings.Join(parts, "; ")
}

func (e *SchemaError) Unwrap() error { return ErrIncompatibleSchema }

func pgCode(err error) (string, *pgconn.PgError) {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return pg.Code, pg
	}
	return "", nil
}
