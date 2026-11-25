package secrets

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// MaxValueBytes is the most a secret's value holds.
	MaxValueBytes = 64 << 10
	// MaxDescription is the most a description holds, in characters.
	MaxDescription = 500

	lastUsedEvery   = time.Minute
	lastUsedTimeout = 2 * time.Second
)

const secretColumns = `id, tenant_id, name, description, ref_path, version, value_version, created_by,
	created_at, updated_at, rotated_at, last_used_at`

func scanSecret(row pgx.Row) (Secret, error) {
	var s Secret
	var path string
	if err := row.Scan(&s.ID, &s.TenantID, &s.Name, &s.Description, &path, &s.Version, &s.ValueVersion, &s.CreatedBy,
		&s.CreatedAt, &s.UpdatedAt, &s.RotatedAt, &s.LastUsedAt); err != nil {
		return Secret{}, err
	}
	s.Ref = Ref{TenantID: s.TenantID, Path: path}
	return s, nil
}

// unavailable wraps a failure of the records or the store as ErrUnavailable; an error of the contract
// passes as it is. Nothing here ever holds a value.
func unavailable(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrUnavailable), errors.Is(err, ErrForbidden), errors.Is(err, ErrInvalid), errors.Is(err, ErrNotFound):
		return err
	}
	return fmt.Errorf("%w: %w", ErrUnavailable, err)
}

func isUnique(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

type manager struct {
	db      *pgxpool.Pool
	writer  Writer
	meta    MetadataReader
	locator Locator
}

// NewManager administers secrets: the records in automation.secrets and the values through writer. meta
// (may be nil) is consulted to reconcile a record's value version; locator forms the reference of a new
// secret's value (nil: DefaultLocator("")).
func NewManager(db *pgxpool.Pool, writer Writer, meta MetadataReader, locator Locator) Manager {
	if locator == nil {
		locator = DefaultLocator("")
	}
	return &manager{db: db, writer: writer, meta: meta, locator: locator}
}

func validateValue(v string) error {
	switch {
	case v == "":
		return fmt.Errorf("%w: value is empty", ErrInvalid)
	case len(v) > MaxValueBytes:
		return fmt.Errorf("%w: value is longer than %d bytes", ErrInvalid, MaxValueBytes)
	}
	return nil
}

func validateDescription(d string) error {
	if utf8.RuneCountInString(d) > MaxDescription {
		return fmt.Errorf("%w: description is longer than %d characters", ErrInvalid, MaxDescription)
	}
	return nil
}

func validateName(n string) error {
	if !IsName(n) {
		return fmt.Errorf("%w: name is upper case letters, digits and _, starting with a letter (up to 64)", ErrInvalid)
	}
	return nil
}

// live is the condition of a record that is not a tombstone.
const live = `deleted_at IS NULL`

func (m *manager) List(ctx context.Context, tenantID uuid.UUID) ([]Secret, error) {
	if tenantID == uuid.Nil {
		return nil, ErrForbidden
	}
	rows, err := m.db.Query(ctx, `SELECT `+secretColumns+` FROM automation.secrets WHERE tenant_id = $1 AND `+live+` ORDER BY name`, tenantID)
	if err != nil {
		return nil, unavailable(err)
	}
	defer rows.Close()
	out := []Secret{}
	for rows.Next() {
		s, err := scanSecret(rows)
		if err != nil {
			return nil, unavailable(err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, unavailable(err)
	}
	m.purge(ctx, tenantID)
	return out, nil
}

func (m *manager) Create(ctx context.Context, tenantID uuid.UUID, in CreateInput) (Secret, error) {
	if tenantID == uuid.Nil {
		return Secret{}, ErrForbidden
	}
	in.Name = strings.TrimSpace(in.Name)
	if err := validateName(in.Name); err != nil {
		return Secret{}, err
	}
	if err := validateDescription(in.Description); err != nil {
		return Secret{}, err
	}
	if err := validateValue(in.Value); err != nil {
		return Secret{}, err
	}
	id := uuid.New()
	ref := m.locator.Locate(tenantID, id)
	if ref.TenantID != tenantID {
		return Secret{}, ErrForbidden
	}
	ref.Version = 0
	// The record is inserted in a transaction that also writes the value and commits only after it: the
	// name is published with its value or not at all. A value written for a record that then fails to
	// commit is removed (and is unreachable anyway: no record names it).
	var out Secret
	err := pgx.BeginFunc(ctx, m.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO automation.secrets (id, tenant_id, name, description, ref_path, value_version, created_by)
			VALUES ($1, $2, $3, $4, $5, 1, $6)`, id, tenantID, in.Name, in.Description, ref.Path, in.CreatedBy); err != nil {
			if isUnique(err) {
				return ErrNameTaken
			}
			return unavailable(err)
		}
		md, err := m.writer.Put(ctx, ref, map[string]string{ValueKey: in.Value})
		if err != nil {
			return unavailable(err)
		}
		out, err = scanSecret(tx.QueryRow(ctx, `UPDATE automation.secrets SET value_version = $3 WHERE tenant_id = $1 AND id = $2 RETURNING `+secretColumns,
			tenantID, id, md.Version))
		return unavailable(err)
	})
	if err != nil {
		if !errors.Is(err, ErrNameTaken) {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			_ = m.writer.Delete(cleanup, ref)
		}
		return Secret{}, err
	}
	m.purge(ctx, tenantID)
	return out, nil
}

func (m *manager) Update(ctx context.Context, tenantID uuid.UUID, in UpdateInput) (Secret, error) {
	if tenantID == uuid.Nil {
		return Secret{}, ErrForbidden
	}
	var name, desc *string
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if err := validateName(n); err != nil {
			return Secret{}, err
		}
		name = &n
	}
	if in.Description != nil {
		if err := validateDescription(*in.Description); err != nil {
			return Secret{}, err
		}
		desc = in.Description
	}
	s, err := scanSecret(m.db.QueryRow(ctx, `UPDATE automation.secrets
		SET name = COALESCE($4, name), description = COALESCE($5, description), version = version + 1, updated_at = now()
		WHERE tenant_id = $1 AND id = $2 AND version = $3 AND `+live+` RETURNING `+secretColumns,
		tenantID, in.ID, in.Version, name, desc))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Secret{}, m.missOrConflict(ctx, tenantID, in.ID)
	case isUnique(err):
		return Secret{}, ErrNameTaken
	case err != nil:
		return Secret{}, unavailable(err)
	}
	return s, nil
}

// missOrConflict tells a secret that is not there from one that changed.
func (m *manager) missOrConflict(ctx context.Context, tenantID, id uuid.UUID) error {
	var one int
	err := m.db.QueryRow(ctx, `SELECT 1 FROM automation.secrets WHERE tenant_id = $1 AND id = $2 AND `+live, tenantID, id).Scan(&one)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return ErrNotFound
	case err != nil:
		return unavailable(err)
	}
	return ErrVersionConflict
}

// Rotate writes the new value as the next version, then publishes it by committing the record that names
// it (value_version), under the record's lock. Readers resolve the version the record names: a rotation
// that fails after the value was written leaves them, and the record, on the version before. Once
// published, the earlier versions are pruned (best effort: a failed prune only keeps old versions longer).
func (m *manager) Rotate(ctx context.Context, tenantID uuid.UUID, in RotateInput) (Secret, error) {
	if tenantID == uuid.Nil {
		return Secret{}, ErrForbidden
	}
	if err := validateValue(in.Value); err != nil {
		return Secret{}, err
	}
	var out Secret
	err := pgx.BeginFunc(ctx, m.db, func(tx pgx.Tx) error {
		cur, err := scanSecret(tx.QueryRow(ctx, `SELECT `+secretColumns+` FROM automation.secrets WHERE tenant_id = $1 AND id = $2 AND `+live+` FOR UPDATE`, tenantID, in.ID))
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return ErrNotFound
		case err != nil:
			return unavailable(err)
		case cur.Version != in.Version:
			return ErrVersionConflict
		}
		md, err := m.writer.Put(ctx, Ref{TenantID: tenantID, Path: cur.Ref.Path}, map[string]string{ValueKey: in.Value})
		if err != nil {
			return unavailable(err)
		}
		out, err = scanSecret(tx.QueryRow(ctx, `UPDATE automation.secrets
			SET value_version = $3, rotated_at = now(), version = version + 1, updated_at = now()
			WHERE tenant_id = $1 AND id = $2 RETURNING `+secretColumns, tenantID, in.ID, md.Version))
		return unavailable(err)
	})
	if err != nil {
		return Secret{}, err
	}
	prune, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = m.writer.Prune(prune, Ref{TenantID: tenantID, Path: out.Ref.Path}, out.ValueVersion)
	m.purge(ctx, tenantID)
	return out, nil
}

// Delete makes the record a tombstone first (committed: the name resolves no more and is free again), then
// removes the value and the tombstone. When the store fails the tombstone stays and the value is removed by
// a later call of the manager for the tenant (purge): the secret is never live without its value, nor its
// value kept without a record to find it by.
func (m *manager) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	if tenantID == uuid.Nil {
		return ErrForbidden
	}
	var path string
	err := m.db.QueryRow(ctx, `UPDATE automation.secrets SET deleted_at = now(), updated_at = now()
		WHERE tenant_id = $1 AND id = $2 AND `+live+` RETURNING ref_path`, tenantID, id).Scan(&path)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return ErrNotFound
	case err != nil:
		return unavailable(err)
	}
	m.removeValue(ctx, tenantID, id, path)
	m.purge(ctx, tenantID)
	return nil
}

// removeValue deletes a tombstone's value and then the tombstone; a failure leaves both for purge.
func (m *manager) removeValue(ctx context.Context, tenantID, id uuid.UUID, path string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := m.writer.Delete(ctx, Ref{TenantID: tenantID, Path: path}); err != nil {
		return
	}
	_, _ = m.db.Exec(ctx, `DELETE FROM automation.secrets WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NOT NULL`, tenantID, id)
}

// purgeBatch is how many tombstones one call of the manager retries.
const purgeBatch = 20

// purge retries removing the values of the tenant's tombstones (a delete whose store call failed).
func (m *manager) purge(ctx context.Context, tenantID uuid.UUID) {
	rows, err := m.db.Query(ctx, `SELECT id, ref_path FROM automation.secrets WHERE tenant_id = $1 AND deleted_at IS NOT NULL ORDER BY deleted_at LIMIT $2`,
		tenantID, purgeBatch)
	if err != nil {
		return
	}
	type tomb struct {
		id   uuid.UUID
		path string
	}
	var tombs []tomb
	for rows.Next() {
		var t tomb
		if rows.Scan(&t.id, &t.path) == nil {
			tombs = append(tombs, t)
		}
	}
	rows.Close()
	for _, t := range tombs {
		m.removeValue(ctx, tenantID, t.id, t.path)
	}
}

type resolver struct {
	db     *pgxpool.Pool
	reader Reader
}

// NewResolver reads a tenant's secret by name: the record from automation.secrets, then the version of the
// value it publishes from reader, on every call (nothing is cached, so a rotation is effective at once in
// every process).
func NewResolver(db *pgxpool.Pool, reader Reader) Resolver {
	return &resolver{db: db, reader: reader}
}

func (r *resolver) Has(ctx context.Context, tenantID uuid.UUID, name string) (bool, error) {
	if tenantID == uuid.Nil {
		return false, ErrForbidden
	}
	if !IsName(name) {
		return false, nil
	}
	if r.db == nil {
		return false, fmt.Errorf("%w: not configured", ErrUnavailable)
	}
	var one int
	err := r.db.QueryRow(ctx, `SELECT 1 FROM automation.secrets WHERE tenant_id = $1 AND name = $2 AND `+live, tenantID, name).Scan(&one)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return false, nil
	case err != nil:
		return false, unavailable(err)
	}
	return true, nil
}

func (r *resolver) Resolve(ctx context.Context, tenantID uuid.UUID, name string) (string, error) {
	if tenantID == uuid.Nil {
		return "", ErrForbidden
	}
	if !IsName(name) {
		return "", ErrNotFound
	}
	if r.db == nil || r.reader == nil {
		return "", fmt.Errorf("%w: not configured", ErrUnavailable)
	}
	var (
		id       uuid.UUID
		path     string
		version  int
		lastUsed *time.Time
	)
	err := r.db.QueryRow(ctx, `SELECT id, ref_path, value_version, last_used_at FROM automation.secrets WHERE tenant_id = $1 AND name = $2 AND `+live,
		tenantID, name).Scan(&id, &path, &version, &lastUsed)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", ErrNotFound
	case err != nil:
		return "", unavailable(err)
	}
	// The version the record publishes, never simply the latest: a rotation not yet published is not read.
	v, err := r.reader.Read(ctx, Ref{TenantID: tenantID, Path: path, Version: version})
	switch {
	case errors.Is(err, ErrNotFound):
		// The record is there and its value is not: a broken store, not an unknown name.
		return "", fmt.Errorf("%w: the value of the secret is missing", ErrUnavailable)
	case err != nil:
		return "", unavailable(err)
	}
	value := v.Data[ValueKey]
	if value == "" {
		return "", fmt.Errorf("%w: the value of the secret is empty", ErrUnavailable)
	}
	if lastUsed == nil || time.Since(*lastUsed) > lastUsedEvery {
		r.touch(ctx, tenantID, id)
	}
	return value, nil
}

// touch records the use, throttled in SQL too (replicas); a failure is ignored.
func (r *resolver) touch(ctx context.Context, tenantID, id uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), lastUsedTimeout)
	defer cancel()
	_, _ = r.db.Exec(ctx, `UPDATE automation.secrets SET last_used_at = now()
		WHERE tenant_id = $1 AND id = $2 AND (last_used_at IS NULL OR last_used_at < now() - make_interval(secs => $3))`,
		tenantID, id, lastUsedEvery.Seconds())
}
