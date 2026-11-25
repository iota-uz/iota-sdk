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
	created_at, updated_at, rotated_at, last_used_at, deleted_at`

func scanSecret(row pgx.Row, extra ...any) (Secret, error) {
	var s Secret
	var path string
	dest := append([]any{&s.ID, &s.TenantID, &s.Name, &s.Description, &path, &s.Version, &s.ValueVersion, &s.CreatedBy,
		&s.CreatedAt, &s.UpdatedAt, &s.RotatedAt, &s.LastUsedAt, &s.DeletedAt}, extra...)
	if err := row.Scan(dest...); err != nil {
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
// The protocol between the records and the value store never holds a transaction (or a pooled connection)
// open while a value is written: the store may take its connections from the records' pool (the sealed
// adapter does), and one with a single connection would wait on itself. Every step is a statement of its
// own, and the records' state tells after any failure what is published:
//
//   - staged: value_version 0. The name is reserved and nothing reads it; its value may or may not exist.
//   - published: value_version N > 0. Readers read version N of the value.
//   - tombstone: deleted_at set. The name is free; the value is being removed.
//
// A write of a value is claimed on the record first (value_write_at, one statement before the store is
// called) and fenced: the store may commit it only until the claim plus writeFence (Ref.NotAfter). A tombstone stays, and its value is deleted again by every purge, until its last claim is older
// than writeWindow: a write that lands after the value was deleted (a rotation racing a delete, a create's
// value written late) is still found and removed, never left without a record.
//
// A step whose outcome is unknown (a lost COMMIT reply) is settled by reading the record again before
// anything is undone: a value is removed only while no published record names it, and a record that is
// given up becomes a tombstone (it is never removed before its value).

const (
	// live is the condition of a record readers and the page see: published, not a tombstone.
	live = `deleted_at IS NULL AND value_version > 0`
	// stagedFor is how long a staged record may wait for its value before purge gives it up.
	stagedFor = 10 * time.Minute

	// readAttempts bounds how often Resolve follows a rotation that moved past the version it looked up.
	readAttempts = 3
)

// writeWindow is how long after its claim a tombstone is kept. A claimed write is fenced to commit by
// writeFence after its claim (Ref.NotAfter, on the database's clock), well inside the window. Variables only
// so a test can shorten them.
var (
	// cleanupTimeout bounds the best-effort steps that run after the caller's context is done (one purge
	// batch as a whole).
	cleanupTimeout = 5 * time.Second
	writeWindow    = 10 * time.Minute
	writeFence     = writeWindow / 2
)

// publishHook runs after a record's publishing statement (tests: a COMMIT whose reply is lost).
var publishHook func() error

func afterPublish(err error) error {
	if err == nil && publishHook != nil {
		return publishHook()
	}
	return err
}

func (m *manager) List(ctx context.Context, tenantID uuid.UUID) ([]Secret, error) {
	if tenantID == uuid.Nil {
		return nil, ErrForbidden
	}
	rows, err := m.db.Query(ctx, `SELECT `+secretColumns+` FROM automation.secrets WHERE tenant_id = $1 AND `+live+` ORDER BY name`, tenantID)
	if err != nil {
		return nil, unavailable(err)
	}
	out := []Secret{}
	for rows.Next() {
		s, err := scanSecret(rows)
		if err != nil {
			rows.Close()
			return nil, unavailable(err)
		}
		out = append(out, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, unavailable(err)
	}
	m.purge(ctx, tenantID)
	return out, nil
}

// Create stages the record (the name is reserved), writes the value, then publishes the record with the
// value's version. A failure is settled by the record: published is a success; staged is undone (record,
// then value); a missing record means the value is an orphan and is removed.
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
	m.purge(ctx, tenantID)
	id := uuid.New()
	ref := m.locator.Locate(tenantID, id)
	if ref.TenantID != tenantID {
		return Secret{}, ErrForbidden
	}
	ref.Version = 0
	var claim time.Time
	if err := m.db.QueryRow(ctx, `INSERT INTO automation.secrets (id, tenant_id, name, description, ref_path, value_version, created_by, value_write_at)
		VALUES ($1, $2, $3, $4, $5, 0, $6, now()) RETURNING value_write_at`, id, tenantID, in.Name, in.Description, ref.Path, in.CreatedBy).Scan(&claim); err != nil {
		if isUnique(err) {
			return Secret{}, ErrNameTaken
		}
		// A lost reply may hide a staged row: purge removes it once it is stale.
		return Secret{}, unavailable(err)
	}
	ref.NotAfter = claim.Add(writeFence)
	md, err := m.writer.Put(ctx, ref, map[string]string{ValueKey: in.Value})
	ref.NotAfter = time.Time{}
	if err != nil {
		m.settleCreate(ctx, tenantID, id, ref)
		return Secret{}, unavailable(err)
	}
	s, err := scanSecret(m.db.QueryRow(ctx, `UPDATE automation.secrets SET value_version = $3
		WHERE tenant_id = $1 AND id = $2 AND value_version = 0 AND deleted_at IS NULL RETURNING `+secretColumns, tenantID, id, md.Version))
	if err = afterPublish(err); err != nil {
		if published, ok := m.settleCreate(ctx, tenantID, id, ref); ok {
			return published, nil
		}
		return Secret{}, unavailable(err)
	}
	return s, nil
}

// settleCreate reads the record of a create that failed after it was staged: published (returned, ok) is a
// success whatever the error said; staged is given up (a tombstone, then its value is removed); gone leaves
// an orphan value to remove. When the record cannot be read nothing is undone: purge settles it later from
// the row.
func (m *manager) settleCreate(ctx context.Context, tenantID, id uuid.UUID, ref Ref) (Secret, bool) {
	ctx, cancel := cleanupContext(ctx)
	defer cancel()
	s, err := scanSecret(m.db.QueryRow(ctx, `SELECT `+secretColumns+` FROM automation.secrets WHERE tenant_id = $1 AND id = $2`, tenantID, id))
	switch {
	case err == nil && s.ValueVersion > 0 && s.DeletedAt == nil:
		return s, true
	case err == nil && s.ValueVersion == 0:
		m.giveUpStaged(ctx, tenantID, id, ref)
	case errors.Is(err, pgx.ErrNoRows):
		_ = m.writer.Delete(ctx, ref) // no record names it: the path is this secret's own
	}
	return Secret{}, false
}

// giveUpStaged turns a staged record into a tombstone (one statement: it can no longer be published, and
// its name is free), then removes its value. The tombstone keeps the reference until the value is gone, so a
// store that fails here, or a process that stops, leaves it for purge.
func (m *manager) giveUpStaged(ctx context.Context, tenantID, id uuid.UUID, ref Ref) {
	tag, err := m.db.Exec(ctx, `UPDATE automation.secrets SET deleted_at = now(), updated_at = now()
		WHERE tenant_id = $1 AND id = $2 AND value_version = 0 AND deleted_at IS NULL`, tenantID, id)
	if err != nil || tag.RowsAffected() == 0 {
		return
	}
	m.removeValue(ctx, tenantID, id, ref.Path)
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

// Rotate writes the new value as the next version, then publishes it with one statement that also checks
// the record's version (optimistic: two rotations of one secret never both publish). Readers resolve the
// version the record names: a rotation that fails after the value was written leaves them, and the record,
// on the version before; the unpublished version is pruned by a later rotation. A failure of the publishing
// statement is settled by reading the record: if it names the new version, the rotation happened.
//
// Once published, the versions before the previous one are pruned: the previous one stays for the readers
// that looked the record up just before the rotation (Resolve also follows a rotation it raced with).
func (m *manager) Rotate(ctx context.Context, tenantID uuid.UUID, in RotateInput) (Secret, error) {
	if tenantID == uuid.Nil {
		return Secret{}, ErrForbidden
	}
	if err := validateValue(in.Value); err != nil {
		return Secret{}, err
	}
	m.purge(ctx, tenantID)
	// The claim of the write, before the store is called: a delete that overtakes this rotation keeps its
	// tombstone until the value written here can no longer land, and removes it then.
	var claim time.Time
	cur, err := scanSecret(m.db.QueryRow(ctx, `UPDATE automation.secrets SET value_write_at = now()
		WHERE tenant_id = $1 AND id = $2 AND `+live+` RETURNING `+secretColumns+`, value_write_at`, tenantID, in.ID), &claim)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Secret{}, ErrNotFound
	case err != nil:
		return Secret{}, unavailable(err)
	case cur.Version != in.Version:
		return Secret{}, ErrVersionConflict
	}
	ref := Ref{TenantID: tenantID, Path: cur.Ref.Path}
	md, err := m.writer.Put(ctx, Ref{TenantID: tenantID, Path: cur.Ref.Path, NotAfter: claim.Add(writeFence)}, map[string]string{ValueKey: in.Value})
	if err != nil {
		return Secret{}, unavailable(err)
	}
	out, err := scanSecret(m.db.QueryRow(ctx, `UPDATE automation.secrets
		SET value_version = $4, rotated_at = now(), version = version + 1, updated_at = now()
		WHERE tenant_id = $1 AND id = $2 AND version = $3 AND `+live+` RETURNING `+secretColumns, tenantID, in.ID, in.Version, md.Version))
	if err = afterPublish(err); err != nil {
		settle, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
		defer cancel()
		now, rerr := scanSecret(m.db.QueryRow(settle, `SELECT `+secretColumns+` FROM automation.secrets WHERE tenant_id = $1 AND id = $2`, tenantID, in.ID))
		switch {
		case rerr == nil && now.ValueVersion == md.Version && now.DeletedAt == nil:
			out = now // published: the error was the reply's
		case rerr == nil && now.DeletedAt != nil:
			// Deleted while the value was written: the value written here goes with the rest.
			m.removeValue(settle, tenantID, in.ID, now.Ref.Path)
			return Secret{}, ErrNotFound
		case errors.Is(err, pgx.ErrNoRows):
			return Secret{}, m.missOrConflict(settle, tenantID, in.ID)
		default:
			return Secret{}, unavailable(err)
		}
	}
	if out.ValueVersion > 1 {
		prune, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
		defer cancel()
		_ = m.writer.Prune(prune, ref, out.ValueVersion-1)
	}
	return out, nil
}

// Delete makes the record a tombstone first (one statement: the name resolves no more and is free again),
// then removes the value and the tombstone. When the store fails the tombstone stays and the value is
// removed by a later call of the manager for the tenant (purge): the secret is never live without its value,
// nor its value kept without a record to find it by.
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
	cleanup, cancel := cleanupContext(ctx)
	m.removeValue(cleanup, tenantID, id, path)
	cancel()
	m.purge(ctx, tenantID)
	return nil
}

// cleanupContext is ctx detached from its cancellation (a cleanup finishes after the caller gave up) and
// bounded by cleanupTimeout.
func cleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
}

// removeValue deletes a tombstone's value, then the tombstone once no claimed write can still land; a
// failure, or a recent claim, leaves the tombstone for purge, which deletes the value again.
//
// Whether the tombstone may go is decided before the value is deleted: only when its last claim had
// already expired then (older than writeWindow, so past its fence of writeFence), and the claim is still
// the same when the row is deleted. Every write claimed before that instant was fenced to commit before
// it, so the delete that follows removed whatever it wrote; a claim that expires during the delete keeps
// the tombstone for the next purge.
//
// ctx bounds the whole step; callers pass a cleanup context (cleanupContext, or purge's batch deadline).
func (m *manager) removeValue(ctx context.Context, tenantID, id uuid.UUID, path string) {
	var (
		claim   *time.Time
		expired bool
	)
	if err := m.db.QueryRow(ctx, `SELECT value_write_at, value_write_at IS NULL OR value_write_at < now() - make_interval(secs => $3)
		FROM automation.secrets WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NOT NULL`,
		tenantID, id, writeWindow.Seconds()).Scan(&claim, &expired); err != nil {
		return
	}
	if err := m.writer.Delete(ctx, Ref{TenantID: tenantID, Path: path}); err != nil || !expired {
		return
	}
	_, _ = m.db.Exec(ctx, `DELETE FROM automation.secrets WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NOT NULL
		AND value_write_at IS NOT DISTINCT FROM $3`, tenantID, id, claim)
}

// purgeBatch is how many records one call of the manager settles.
const purgeBatch = 20

// purge settles the tenant's records a failure left behind: tombstones whose value is still there, and
// staged records older than stagedFor (a create that never published). Rows are read first and the
// connection released before the value store is called.
func (m *manager) purge(ctx context.Context, tenantID uuid.UUID) {
	type left struct {
		id     uuid.UUID
		path   string
		staged bool
	}
	rows, err := m.db.Query(ctx, `SELECT id, ref_path, deleted_at IS NULL FROM automation.secrets
		WHERE tenant_id = $1 AND (deleted_at IS NOT NULL OR (value_version = 0 AND created_at < now() - make_interval(secs => $2)))
		ORDER BY updated_at LIMIT $3`, tenantID, stagedFor.Seconds(), purgeBatch)
	if err != nil {
		return
	}
	var found []left
	for rows.Next() {
		var l left
		if rows.Scan(&l.id, &l.path, &l.staged) == nil {
			found = append(found, l)
		}
	}
	rows.Close()
	// One deadline for the whole batch: a slow or failing store costs the caller at most cleanupTimeout,
	// and what is left unfinished waits for the next call.
	batch, cancel := cleanupContext(ctx)
	defer cancel()
	for _, l := range found {
		if batch.Err() != nil {
			return
		}
		if l.staged {
			m.giveUpStaged(batch, tenantID, l.id, Ref{TenantID: tenantID, Path: l.path})
			continue
		}
		m.removeValue(batch, tenantID, l.id, l.path)
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

// published is what a record says a reader reads.
type published struct {
	id       uuid.UUID
	path     string
	version  int
	lastUsed *time.Time
}

func (r *resolver) lookup(ctx context.Context, tenantID uuid.UUID, name string) (published, error) {
	var p published
	err := r.db.QueryRow(ctx, `SELECT id, ref_path, value_version, last_used_at FROM automation.secrets WHERE tenant_id = $1 AND name = $2 AND `+live,
		tenantID, name).Scan(&p.id, &p.path, &p.version, &p.lastUsed)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return p, ErrNotFound
	case err != nil:
		return p, unavailable(err)
	}
	return p, nil
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
	p, err := r.lookup(ctx, tenantID, name)
	if err != nil {
		return "", err
	}
	for attempt := 1; ; attempt++ {
		// The version the record publishes, never simply the latest: a rotation not yet published is not read.
		v, err := r.reader.Read(ctx, Ref{TenantID: tenantID, Path: p.path, Version: p.version})
		if errors.Is(err, ErrNotFound) && attempt < readAttempts {
			// A rotation may have published a later version and pruned this one between the two reads:
			// follow the record. An unchanged record means the value is gone: a broken store.
			next, lerr := r.lookup(ctx, tenantID, name)
			switch {
			case lerr != nil:
				return "", lerr
			case next.id == p.id && next.version != p.version, next.id != p.id:
				p = next
				continue
			}
		}
		switch {
		case errors.Is(err, ErrNotFound):
			return "", fmt.Errorf("%w: the value of the secret is missing", ErrUnavailable)
		case err != nil:
			return "", unavailable(err)
		}
		value := v.Data[ValueKey]
		if value == "" {
			return "", fmt.Errorf("%w: the value of the secret is empty", ErrUnavailable)
		}
		if p.lastUsed == nil || time.Since(*p.lastUsed) > lastUsedEvery {
			r.touch(ctx, tenantID, p.id)
		}
		return value, nil
	}
}

// touch records the use, throttled in SQL too (replicas); a failure is ignored.
func (r *resolver) touch(ctx context.Context, tenantID, id uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), lastUsedTimeout)
	defer cancel()
	_, _ = r.db.Exec(ctx, `UPDATE automation.secrets SET last_used_at = now()
		WHERE tenant_id = $1 AND id = $2 AND (last_used_at IS NULL OR last_used_at < now() - make_interval(secs => $3))`,
		tenantID, id, lastUsedEvery.Seconds())
}
