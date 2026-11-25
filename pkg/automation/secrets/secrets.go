// Package secrets holds the tenant secrets of automations (the built-in «Secret» connector of the
// integrations page) and the storage port their values live behind.
//
// A secret is two things kept apart:
//
//   - its record (Secret): name (API_KEY), description, the reference to its value (Ref) and safe metadata
//     — created, rotated, last used. The integrations page, the nodes and the module see only this;
//   - its value, behind the storage port: Writer (put a new version, delete), Reader (read the current
//     version whole) and MetadataReader (versions and times, never a value). The first adapter is the
//     encrypted per-tenant store of connections (connectors.NewSealedSecrets: AES-256-GCM with the
//     connections' keyring); an OpenBao KV v2 adapter (#1382) replaces it without changing the records,
//     the page or the nodes.
//
// The contract every adapter keeps:
//
//   - a Read returns one version whole: never fields of two versions;
//   - a Put is visible to the next Read of any process (no cache outlives a rotation);
//   - a Ref of another tenant is refused (ErrForbidden), whatever its path;
//   - an outage is ErrUnavailable: a caller never falls back to the environment or a stale value;
//   - errors never carry a value.
package secrets

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/google/uuid"
)

// Ref is where a secret's value lives: the tenant it belongs to and a path the application forms
// (Locator). An adapter maps it to its own address (an OpenBao KV v2 mount and path, a table row).
type Ref struct {
	TenantID uuid.UUID
	Path     string
}

// Value is one version of a secret's value, read whole.
type Value struct {
	Data    map[string]string
	Version int
}

// Metadata is what an adapter tells about a value without reading it.
type Metadata struct {
	Version   int // the current version, from 1
	CreatedAt time.Time
	UpdatedAt time.Time // when the current version was written
}

// Writer writes values: what managing a secret needs. A process may hold a Writer only (an OpenBao role
// that may write and not read).
type Writer interface {
	// Put writes data as the value's next version (the first when there is none) and returns its metadata.
	Put(ctx context.Context, ref Ref, data map[string]string) (Metadata, error)
	// Delete removes the value with every version; deleting what is not there is not an error.
	Delete(ctx context.Context, ref Ref) error
}

// Reader reads values: what resolving a secret for a node needs.
type Reader interface {
	// Read is the current version, whole; ErrNotFound when there is none.
	Read(ctx context.Context, ref Ref) (Value, error)
}

// MetadataReader reads what is safe to show: versions and times, never a value.
type MetadataReader interface {
	Metadata(ctx context.Context, ref Ref) (Metadata, error)
}

// Store is an adapter that serves all three; the composition hands each consumer only the part it needs.
type Store interface {
	Writer
	Reader
	MetadataReader
}

// Locator forms the reference of a secret's value: the application decides the path by environment and
// owner (an OpenBao policy is written against it).
type Locator interface {
	Locate(tenantID, secretID uuid.UUID) Ref
}

// LocatorFunc adapts a function to a Locator.
type LocatorFunc func(tenantID, secretID uuid.UUID) Ref

func (f LocatorFunc) Locate(tenantID, secretID uuid.UUID) Ref { return f(tenantID, secretID) }

// DefaultLocator is "<prefix>tenants/<tenant>/automation/secrets/<secret>"; prefix is the environment
// ("prod/"), may be empty.
func DefaultLocator(prefix string) Locator {
	return LocatorFunc(func(tenantID, secretID uuid.UUID) Ref {
		return Ref{TenantID: tenantID, Path: prefix + "tenants/" + tenantID.String() + "/automation/secrets/" + secretID.String()}
	})
}

// ValueKey is the key of a secret's value in Value.Data.
const ValueKey = "value"

// Secret is a tenant's secret without its value.
type Secret struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Name        string // ^[A-Z][A-Z0-9_]{0,63}$, unique in the tenant
	Description string
	Ref         Ref
	// Version is the record's version (optimistic concurrency of rename and rotate).
	Version int
	// ValueVersion is the version of the value the last write produced.
	ValueVersion int
	CreatedBy    int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
	RotatedAt    time.Time // the last write of the value (CreatedAt for a secret never rotated)
	LastUsedAt   *time.Time
}

var nameRe = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)

// IsName reports whether s is a secret's name: API_KEY, upper case letters, digits and _, ≤ 64.
func IsName(s string) bool { return nameRe.MatchString(s) }

// CreateInput creates a secret.
type CreateInput struct {
	Name, Description, Value string
	CreatedBy                int64
}

// UpdateInput renames a secret or changes its description (nil: kept); Version must match.
type UpdateInput struct {
	ID          uuid.UUID
	Version     int
	Name        *string
	Description *string
}

// RotateInput writes a new value; Version must match.
type RotateInput struct {
	ID      uuid.UUID
	Version int
	Value   string
}

// Manager administers a tenant's secrets: what the integrations page calls (after its own permission
// check). It never returns a value.
type Manager interface {
	List(ctx context.Context, tenantID uuid.UUID) ([]Secret, error)
	Create(ctx context.Context, tenantID uuid.UUID, in CreateInput) (Secret, error)
	Update(ctx context.Context, tenantID uuid.UUID, in UpdateInput) (Secret, error)
	Rotate(ctx context.Context, tenantID uuid.UUID, in RotateInput) (Secret, error)
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}

// Resolver reads a tenant's secret by name for a node: the current version of its value. ErrNotFound when
// the tenant has no secret of the name (the caller tries the application's resolver next); ErrUnavailable
// when the records or the store fail (the caller fails, never falls back).
type Resolver interface {
	Resolve(ctx context.Context, tenantID uuid.UUID, name string) (string, error)
}

var (
	// ErrNotFound: no secret (or value) of that name, id or ref in the tenant.
	ErrNotFound = errors.New("automation secrets: not found")
	// ErrUnavailable: the records or the value store cannot be reached now; retry later.
	ErrUnavailable = errors.New("automation secrets: the secret store is unavailable")
	// ErrForbidden: a ref of another tenant, or an operation the caller's role may not perform.
	ErrForbidden = errors.New("automation secrets: forbidden")
	// ErrNameTaken: the tenant has a secret of the name.
	ErrNameTaken = errors.New("automation secrets: name is taken")
	// ErrVersionConflict: the secret changed since it was read.
	ErrVersionConflict = errors.New("automation secrets: version conflict")
	// ErrInvalid: a name, description or value the store refuses (the error names the field, never the value).
	ErrInvalid = errors.New("automation secrets: invalid")
)
