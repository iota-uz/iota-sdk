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
//   - values are versioned like KV v2: a Put adds the next version and keeps the earlier ones until Prune;
//   - a Read returns one version whole (the one the Ref names, or the latest): never fields of two versions;
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
//
// Version names one version of the value for Read (KV v2's ?version=N); 0 is the latest. The records
// publish a value by its version: a rotation writes version N+1 and then commits the record that names it,
// so a rotation that fails between the two writes leaves every reader on version N.
type Ref struct {
	TenantID uuid.UUID
	Path     string
	Version  int
	// NotAfter fences a Put (zero: none): the adapter must not commit a version after this instant. It
	// refuses to start a write past it (ErrUnavailable), and bounds the write so it cannot commit later (a
	// server-side condition, a request deadline that ends before it). The records claim a write before
	// calling Put and give the store until NotAfter to land it, so a deleted secret's tombstone is retired
	// only when no claimed write can still create its value.
	NotAfter time.Time
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
	// Put writes data as the value's next version (the first when there is none) and returns the metadata
	// of exactly that version (Version is the one this call wrote, even when a concurrent Put wrote a later one).
	Put(ctx context.Context, ref Ref, data map[string]string) (Metadata, error)
	// Delete removes the value with every version; deleting what is not there is not an error.
	Delete(ctx context.Context, ref Ref) error
	// Prune removes the versions before keepFrom (KV v2: destroy them), once a later version is published;
	// pruning what is not there is not an error.
	Prune(ctx context.Context, ref Ref, keepFrom int) error
	// List is the paths that hold a value (any version) under root.Path, a prefix, in root's tenant (KV v2:
	// LIST on the metadata). The records sweep it: a value no record names is removed, whenever it was
	// written, so a write that commits after its secret was deleted never stays without a way to find it.
	List(ctx context.Context, root Ref) ([]string, error)
}

// Reader reads values: what resolving a secret for a node needs.
type Reader interface {
	// Read is the version ref.Version (0: the latest), whole; ErrNotFound when there is none.
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
	// Root is the prefix every value Locate gives the tenant lies under: the records sweep it, and remove
	// a value there that no record names. Nothing else may be stored under it.
	//
	// Locate must give each secret a path of its own for good: under Root and carrying secretID (a fresh
	// id per secret), never a path an earlier secret had. The manager refuses a path that does not.
	Root(tenantID uuid.UUID) Ref
}

// DefaultLocator is "<prefix>tenants/<tenant>/automation/secrets/<secret>"; prefix is the environment
// ("prod/"), may be empty.
func DefaultLocator(prefix string) Locator { return defaultLocator{prefix: prefix} }

type defaultLocator struct{ prefix string }

func (l defaultLocator) Root(tenantID uuid.UUID) Ref {
	return Ref{TenantID: tenantID, Path: l.prefix + "tenants/" + tenantID.String() + "/automation/secrets/"}
}

func (l defaultLocator) Locate(tenantID, secretID uuid.UUID) Ref {
	root := l.Root(tenantID)
	root.Path += secretID.String()
	return root
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
	// DeletedAt is set on a tombstone (a deleted secret whose value is being removed); a Manager never
	// returns one.
	DeletedAt *time.Time
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

// Resolver reads a tenant's secret by name for a node: the published version of its value. ErrNotFound
// when the tenant has no secret of the name (the caller tries the application's resolver next);
// ErrUnavailable when the records or the store fail (the caller fails, never falls back).
type Resolver interface {
	// Has reports whether the tenant has a secret of the name, from the records only: no value is read
	// and no use recorded, so a caller authorizes before Resolve.
	Has(ctx context.Context, tenantID uuid.UUID, name string) (bool, error)
	// Resolve reads the value and records the use.
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
