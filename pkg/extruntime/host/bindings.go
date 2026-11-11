package host

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	opResolve  = serrors.Op("extruntime.host.Bindings.Resolve")
	opActivate = serrors.Op("extruntime.host.Bindings.Activate")
)

// ErrNoBinding is returned when a contract has no active implementation.
// The host never falls back to another implementation.
var ErrNoBinding = errors.New("extruntime: no active binding for contract")

// Binding is the active implementation selected for one invocation. It is
// recorded on the invocation and never changes for that invocation, even if
// the active binding switches while it runs.
type Binding struct {
	ID       string
	Contract string
	Package  string
	Version  string
	Export   string
	Artifact pool.Artifact
	// Secrets lists the secret names the package declared in its
	// capabilities; secrets.get is limited to them.
	Secrets []string
}

// Bindings resolves the single active implementation of a contract.
type Bindings interface {
	Resolve(ctx context.Context, contractID string) (Binding, error)
}

// ArtifactResolver maps a package version to its immutable artifact and
// declared secrets. It is implemented by the management slice.
type ArtifactResolver interface {
	ResolveArtifact(ctx context.Context, pkg, version string) (pool.Artifact, []string, error)
}

// PgBindings reads extruntime.bindings (slot_kind = 'command').
type PgBindings struct {
	db        *pgxpool.Pool
	artifacts ArtifactResolver
}

func NewPgBindings(db *pgxpool.Pool, artifacts ArtifactResolver) *PgBindings {
	return &PgBindings{db: db, artifacts: artifacts}
}

func (b *PgBindings) Resolve(ctx context.Context, contractID string) (Binding, error) {
	var out Binding
	err := b.db.QueryRow(ctx, `
		SELECT id::text, package, version, export
		FROM extruntime.bindings
		WHERE slot_kind = 'command' AND target_id = $1 AND active_to IS NULL`, contractID).
		Scan(&out.ID, &out.Package, &out.Version, &out.Export)
	if errors.Is(err, pgx.ErrNoRows) {
		return Binding{}, serrors.Wrap(opResolve, errors.Join(ErrNoBinding, errors.New(contractID)))
	}
	if err != nil {
		return Binding{}, serrors.FromDB(opResolve, err)
	}
	art, secrets, err := b.artifacts.ResolveArtifact(ctx, out.Package, out.Version)
	if err != nil {
		return Binding{}, serrors.Wrap(opResolve, err)
	}
	out.Contract = contractID
	out.Artifact = art
	out.Secrets = secrets
	return out, nil
}

// Activate makes package@version/export the active implementation of the
// contract. The previous open row is closed in the same transaction, so there
// is exactly one active binding per command. Running invocations keep the
// binding they already selected.
func (b *PgBindings) Activate(ctx context.Context, contractID, pkg, version, export string, activatedBy *int64) (string, error) {
	tx, err := b.db.Begin(ctx)
	if err != nil {
		return "", serrors.FromDB(opActivate, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		UPDATE extruntime.bindings SET active_to = CURRENT_TIMESTAMP
		WHERE slot_kind = 'command' AND target_id = $1 AND active_to IS NULL`, contractID); err != nil {
		return "", serrors.FromDB(opActivate, err)
	}
	id := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO extruntime.bindings (id, slot_kind, target_id, package, version, export, activated_by)
		VALUES ($1, 'command', $2, $3, $4, $5, $6)`, id, contractID, pkg, version, export, activatedBy); err != nil {
		return "", serrors.FromDB(opActivate, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", serrors.FromDB(opActivate, err)
	}
	return id, nil
}
