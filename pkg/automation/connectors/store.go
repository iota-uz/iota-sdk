package connectors

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

var nameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)

const (
	maxDisplayName  = 200
	maxStatusReason = 2000
	maxInboundBytes = MaxInboundBytes
)

// MaxInboundBytes is the most a connection's inbound state (a polling cursor, a subscription id) holds,
// as JSON: UpdateInbound refuses more, and so does whoever writes the state in its own transaction.
const MaxInboundBytes = 1 << 20

// IsName reports whether s is a valid connection name.
func IsName(s string) bool { return nameRe.MatchString(s) }

type store struct {
	db       *pgxpool.Pool
	keys     Keyring
	usage    UsageIndex
	platform PlatformCredentials
	grants   PlatformGrants
}

// Store is the ConnectionStore over automation.connections; it is also the CredentialSource of the
// engine.
type Store interface {
	ConnectionStore
	CredentialSource
	// SaveWith is Save with within run in the same transaction after the write: a record that must
	// exist exactly when the connection change does (an idempotency key). An error of within rolls the
	// write back.
	SaveWith(ctx context.Context, c Connection, creds Credentials, within func(ctx context.Context, tx pgx.Tx, saved Connection) error) (Connection, error)
	// Fingerprint is an HMAC of data under the active key: it binds a record to a payload with
	// credentials without storing anything a value could be guessed back from. ErrNotConfigured without
	// a keyring.
	Fingerprint(data []byte) (string, error)
	// MatchesFingerprint reports whether fp (from Fingerprint) is data's fingerprint under the key that
	// made it, so a record stays verifiable after the active key rotates while that key is readable.
	MatchesFingerprint(fp string, data []byte) bool
}

// NewStore is the connection store. keys nil: tenant connections are unavailable (ErrNotConfigured)
// and only platform credentials work. usage nil: nothing uses a connection yet, so deleting is never
// blocked. platform and grants may be nil (no platform credentials; no opt_in grants).
func NewStore(db *pgxpool.Pool, keys Keyring, usage UsageIndex, platform PlatformCredentials, grants PlatformGrants) Store {
	return &store{db: db, keys: keys, usage: usage, platform: platform, grants: grants}
}

const connectionColumns = `tenant_id, id, kind, name, display_name, identity, status, status_reason, version, key_id,
	COALESCE(created_by, 0), created_at, updated_at`

func scanConnection(row pgx.Row, extra ...any) (Connection, error) {
	var (
		c        Connection
		kind     string
		status   string
		identity []byte
	)
	dest := append([]any{&c.TenantID, &c.ID, &kind, &c.Name, &c.DisplayName, &identity, &status, &c.StatusReason,
		&c.Version, &c.KeyID, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt}, extra...)
	if err := row.Scan(dest...); err != nil {
		return Connection{}, err
	}
	c.Kind = catalog.ConnectorKind(kind)
	c.Status = ConnectionStatus(status)
	if len(identity) > 0 {
		if err := json.Unmarshal(identity, &c.Identity); err != nil {
			return Connection{}, fmt.Errorf("identity: %w", err)
		}
	}
	return c, nil
}

func (s *store) configured() error {
	if s.keys == nil {
		return ErrNotConfigured
	}
	return nil
}

// Connected lists the working connections of a tenant: its connected connections (when the keyring can
// decrypt them), then the platform credential by the connector's policy.
func (s *store) Connected(ctx context.Context, tenantID uuid.UUID) (map[catalog.ConnectorKind][]catalog.ConnectionRef, error) {
	const op = serrors.Op("automation.connectors.Store.Connected")
	out := map[catalog.ConnectorKind][]catalog.ConnectionRef{}
	if s.keys != nil {
		rows, err := s.db.Query(ctx, `SELECT `+connectionColumns+`, nonce, secret FROM automation.connections
			WHERE tenant_id = $1 AND status = $2 ORDER BY kind, created_at, name`, tenantID, string(StatusConnected))
		if err != nil {
			return nil, serrors.Wrap(op, err)
		}
		for rows.Next() {
			var nonce, secret []byte
			c, err := scanConnection(rows, &nonce, &secret)
			if err != nil {
				rows.Close()
				return nil, serrors.Wrap(op, err)
			}
			// Only a row whose credentials authenticate is working: its key is gone, or the row was moved
			// or changed outside the store, and Open would refuse it.
			if _, err := s.decrypt(c, nonce, secret); err != nil {
				continue
			}
			out[c.Kind] = append(out[c.Kind], catalog.ConnectionRef{ID: c.ID.String(), Name: c.Name})
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, serrors.Wrap(op, err)
		}
	}
	pc, ok := s.platform.(PlatformCatalog)
	if !ok {
		return out, nil
	}
	for _, kind := range pc.Kinds() {
		available, err := s.platformAvailable(ctx, tenantID, pc, kind)
		if err != nil {
			return nil, serrors.Wrap(op, err)
		}
		if available {
			out[kind] = append(out[kind], catalog.ConnectionRef{ID: catalog.PlatformConnectionID(kind), Name: "platform"})
		}
	}
	return out, nil
}

func (s *store) platformAvailable(ctx context.Context, tenantID uuid.UUID, pc PlatformCatalog, kind catalog.ConnectorKind) (bool, error) {
	if _, _, ok := s.platform.Platform(kind); !ok {
		return false, nil
	}
	switch pc.Policy(kind) {
	case PlatformAuto:
		return true, nil
	case PlatformOptIn:
		if s.grants == nil {
			return false, nil
		}
		return s.grants.Enabled(ctx, tenantID, kind)
	case PlatformNone:
		return false, nil
	}
	return false, nil
}

func (s *store) List(ctx context.Context, tenantID uuid.UUID, kind catalog.ConnectorKind) ([]Connection, error) {
	const op = serrors.Op("automation.connectors.Store.List")
	if err := s.configured(); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	rows, err := s.db.Query(ctx, `SELECT `+connectionColumns+` FROM automation.connections
		WHERE tenant_id = $1 AND ($2 = '' OR kind = $2) ORDER BY kind, name`, tenantID, string(kind))
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer rows.Close()
	out := []Connection{}
	for rows.Next() {
		c, err := scanConnection(rows)
		if err != nil {
			return nil, serrors.Wrap(op, err)
		}
		out = append(out, c)
	}
	return out, serrors.Wrap(op, rows.Err())
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// load reads a connection and decrypts its credentials; forUpdate locks the row.
func (s *store) load(ctx context.Context, q querier, tenantID, id uuid.UUID, forUpdate bool) (Connection, Credentials, error) {
	if err := s.configured(); err != nil {
		return Connection{}, nil, err
	}
	sql := `SELECT ` + connectionColumns + `, nonce, secret FROM automation.connections WHERE tenant_id = $1 AND id = $2`
	if forUpdate {
		sql += ` FOR UPDATE`
	}
	var nonce, secret []byte
	c, err := scanConnection(q.QueryRow(ctx, sql, tenantID, id), &nonce, &secret)
	if errors.Is(err, pgx.ErrNoRows) {
		return Connection{}, nil, ErrNotFound
	}
	if err != nil {
		return Connection{}, nil, err
	}
	creds, err := s.decrypt(c, nonce, secret)
	if err != nil {
		return Connection{}, nil, err
	}
	return c, creds, nil
}

func (s *store) decrypt(c Connection, nonce, secret []byte) (Credentials, error) {
	key, ok := s.keys.Key(c.KeyID)
	if !ok {
		return nil, fmt.Errorf("%w: key %s of the connection is not in the keyring", ErrNotConfigured, c.KeyID)
	}
	plain, err := open(key, nonce, secret, aad(c.TenantID, c.ID, c.Kind))
	if err != nil {
		return nil, err
	}
	var creds Credentials
	if err := json.Unmarshal(plain, &creds); err != nil {
		return nil, ErrUndecryptable
	}
	if creds == nil {
		creds = Credentials{}
	}
	return creds, nil
}

// encrypt seals credentials with the active key.
// It returns the key id, the nonce and the sealed secret.
func (s *store) encrypt(c Connection, creds Credentials) (string, []byte, []byte, error) {
	keyID, key := s.keys.Active()
	plain, err := json.Marshal(creds)
	if err != nil {
		return "", nil, nil, fmt.Errorf("%w: credentials are not JSON", ErrInvalid)
	}
	nonce, secret, err := seal(key, plain, aad(c.TenantID, c.ID, c.Kind))
	return keyID, nonce, secret, err
}

func (s *store) Get(ctx context.Context, tenantID, id uuid.UUID) (Connection, Credentials, error) {
	c, creds, err := s.load(ctx, s.db, tenantID, id, false)
	if err != nil {
		return Connection{}, nil, serrors.Wrap("automation.connectors.Store.Get", err)
	}
	return c, creds, nil
}

func checkConnection(c Connection) (Connection, error) {
	c.DisplayName = strings.TrimSpace(c.DisplayName)
	if c.DisplayName == "" {
		c.DisplayName = c.Name
	}
	if c.Status == "" {
		c.Status = StatusConnected
	}
	switch {
	case c.TenantID == uuid.Nil:
		return c, fmt.Errorf("%w: a connection without a tenant", ErrInvalid)
	case c.Kind == "":
		return c, fmt.Errorf("%w: a connection without a kind", ErrInvalid)
	case !IsName(c.Name):
		return c, fmt.Errorf("%w: name %q: a lowercase letter, then lowercase letters, digits and _ (≤ 40)", ErrInvalid, c.Name)
	case utf8.RuneCountInString(c.DisplayName) > maxDisplayName:
		return c, fmt.Errorf("%w: display name is longer than %d", ErrInvalid, maxDisplayName)
	case !c.Status.valid():
		return c, fmt.Errorf("%w: status %q", ErrInvalid, c.Status)
	case utf8.RuneCountInString(c.StatusReason) > maxStatusReason:
		return c, fmt.Errorf("%w: status reason is longer than %d", ErrInvalid, maxStatusReason)
	case c.Version < 0:
		return c, fmt.Errorf("%w: a negative version", ErrInvalid)
	}
	return c, nil
}

func (s *store) Save(ctx context.Context, c Connection, creds Credentials) (Connection, error) {
	const op = serrors.Op("automation.connectors.Store.Save")
	if err := s.configured(); err != nil {
		return Connection{}, serrors.Wrap(op, err)
	}
	c, err := checkConnection(c)
	if err != nil {
		return Connection{}, serrors.Wrap(op, err)
	}
	identity, err := json.Marshal(c.Identity)
	if err != nil {
		return Connection{}, serrors.Wrap(op, err)
	}
	var saved Connection
	if c.Version == 0 {
		saved, err = s.create(ctx, s.db, c, identity, creds)
	} else {
		saved, err = s.update(ctx, s.db, c, identity, creds)
	}
	if err != nil {
		return Connection{}, serrors.Wrap(op, err)
	}
	return saved, nil
}

func (s *store) SaveWith(ctx context.Context, c Connection, creds Credentials, within func(ctx context.Context, tx pgx.Tx, saved Connection) error) (Connection, error) {
	const op = serrors.Op("automation.connectors.Store.SaveWith")
	if err := s.configured(); err != nil {
		return Connection{}, serrors.Wrap(op, err)
	}
	c, err := checkConnection(c)
	if err != nil {
		return Connection{}, serrors.Wrap(op, err)
	}
	identity, err := json.Marshal(c.Identity)
	if err != nil {
		return Connection{}, serrors.Wrap(op, err)
	}
	var saved Connection
	err = pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var err error
		if c.Version == 0 {
			saved, err = s.create(ctx, tx, c, identity, creds)
		} else {
			saved, err = s.update(ctx, tx, c, identity, creds)
		}
		if err != nil {
			return err
		}
		if within != nil {
			return within(ctx, tx, saved)
		}
		return nil
	})
	if err != nil {
		return Connection{}, serrors.Wrap(op, err)
	}
	return saved, nil
}

var _ GroupSaver = (*store)(nil)

func (s *store) SaveGroup(ctx context.Context, items []GroupItem) ([]Connection, error) {
	const op = serrors.Op("automation.connectors.Store.SaveGroup")
	if err := s.configured(); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	if len(items) == 0 {
		return nil, nil
	}
	out := make([]Connection, 0, len(items))
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		out = out[:0]
		for i, it := range items {
			c, err := checkConnection(it.Connection)
			if err != nil {
				return err
			}
			identity, err := json.Marshal(c.Identity)
			if err != nil {
				return err
			}
			creds := make(Credentials, len(it.Credentials)+1)
			for k, v := range it.Credentials {
				creds[k] = v
			}
			delete(creds, OAuth2TokenFrom)
			if i > 0 {
				creds[OAuth2TokenFrom] = out[0].ID.String()
			}
			var saved Connection
			if c.Version == 0 {
				saved, err = s.create(ctx, tx, c, identity, creds)
			} else {
				saved, err = s.update(ctx, tx, c, identity, creds)
			}
			if err != nil {
				return err
			}
			out = append(out, saved)
		}
		return nil
	})
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return out, nil
}

func (s *store) Fingerprint(data []byte) (string, error) {
	if err := s.configured(); err != nil {
		return "", serrors.Wrap("automation.connectors.Store.Fingerprint", err)
	}
	id, key := s.keys.Active()
	return fingerprint(id, key, data), nil
}

func (s *store) MatchesFingerprint(fp string, data []byte) bool {
	if s.configured() != nil {
		return false
	}
	id, _, ok := strings.Cut(fp, ":")
	if !ok {
		return false
	}
	key, ok := s.keys.Key(id)
	if !ok {
		return false
	}
	return hmac.Equal([]byte(fingerprint(id, key, data)), []byte(fp))
}

func fingerprint(id string, key, data []byte) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("automation-request-fingerprint\x00"))
	_, _ = mac.Write(data)
	return id + ":" + hex.EncodeToString(mac.Sum(nil))
}

func (s *store) create(ctx context.Context, q querier, c Connection, identity []byte, creds Credentials) (Connection, error) {
	if creds == nil {
		return Connection{}, fmt.Errorf("%w: a new connection needs credentials", ErrInvalid)
	}
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	keyID, nonce, secret, err := s.encrypt(c, creds)
	if err != nil {
		return Connection{}, err
	}
	row := q.QueryRow(ctx, `INSERT INTO automation.connections
		(tenant_id, id, kind, name, display_name, identity, status, status_reason, key_id, nonce, secret, version, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 1, $12, now(), now())
		RETURNING `+connectionColumns,
		c.TenantID, c.ID, string(c.Kind), c.Name, c.DisplayName, identity, string(c.Status), c.StatusReason, keyID, nonce, secret, nullableUser(c.CreatedBy))
	saved, err := scanConnection(row)
	return saved, mapWriteError(err)
}

type execQuerier interface {
	querier
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (s *store) update(ctx context.Context, q execQuerier, c Connection, identity []byte, creds Credentials) (Connection, error) {
	var row pgx.Row
	if creds != nil {
		keyID, nonce, secret, err := s.encrypt(c, creds)
		if err != nil {
			return Connection{}, err
		}
		row = q.QueryRow(ctx, `UPDATE automation.connections SET name = $5, display_name = $6, identity = $7, status = $8,
			status_reason = $9, key_id = $10, nonce = $11, secret = $12, version = version + 1, updated_at = now()
			WHERE tenant_id = $1 AND id = $2 AND kind = $3 AND version = $4
			RETURNING `+connectionColumns,
			c.TenantID, c.ID, string(c.Kind), c.Version, c.Name, c.DisplayName, identity, string(c.Status), c.StatusReason, keyID, nonce, secret)
	} else {
		row = q.QueryRow(ctx, `UPDATE automation.connections SET name = $5, display_name = $6, identity = $7, status = $8,
			status_reason = $9, version = version + 1, updated_at = now()
			WHERE tenant_id = $1 AND id = $2 AND kind = $3 AND version = $4
			RETURNING `+connectionColumns,
			c.TenantID, c.ID, string(c.Kind), c.Version, c.Name, c.DisplayName, identity, string(c.Status), c.StatusReason)
	}
	saved, err := scanConnection(row)
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM automation.connections WHERE tenant_id = $1 AND id = $2 AND kind = $3)`,
			c.TenantID, c.ID, string(c.Kind)).Scan(&exists); err != nil {
			return Connection{}, err
		}
		if exists {
			return Connection{}, ErrVersionConflict
		}
		return Connection{}, ErrNotFound
	}
	return saved, mapWriteError(err)
}

func mapWriteError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23505":
			if pg.ConstraintName == "connections_pkey" {
				return fmt.Errorf("%w: the connection id is taken", ErrInvalid)
			}
			return ErrNameTaken
		case "23503":
			return fmt.Errorf("%w: unknown tenant", ErrInvalid)
		}
	}
	return err
}

func (s *store) SetStatus(ctx context.Context, tenantID, id uuid.UUID, status ConnectionStatus, reason string) error {
	const op = serrors.Op("automation.connectors.Store.SetStatus")
	if err := s.configured(); err != nil {
		return serrors.Wrap(op, err)
	}
	if !status.valid() {
		return serrors.Wrap(op, fmt.Errorf("%w: status %q", ErrInvalid, status))
	}
	if utf8.RuneCountInString(reason) > maxStatusReason {
		return serrors.Wrap(op, fmt.Errorf("%w: status reason is longer than %d", ErrInvalid, maxStatusReason))
	}
	tag, err := s.db.Exec(ctx, `UPDATE automation.connections SET status = $3, status_reason = $4, version = version + 1, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`, tenantID, id, string(status), reason)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if tag.RowsAffected() == 0 {
		return serrors.Wrap(op, ErrNotFound)
	}
	return nil
}

func (s *store) SetStatusIf(ctx context.Context, tenantID, id uuid.UUID, status ConnectionStatus, reason string, cond func(state json.RawMessage, version int) bool) (bool, error) {
	const op = serrors.Op("automation.connectors.Store.SetStatusIf")
	if err := s.configured(); err != nil {
		return false, serrors.Wrap(op, err)
	}
	if !status.valid() {
		return false, serrors.Wrap(op, fmt.Errorf("%w: status %q", ErrInvalid, status))
	}
	if utf8.RuneCountInString(reason) > maxStatusReason {
		return false, serrors.Wrap(op, fmt.Errorf("%w: status reason is longer than %d", ErrInvalid, maxStatusReason))
	}
	set := false
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var state []byte
		var version int
		err := tx.QueryRow(ctx, `SELECT inbound, version FROM automation.connections WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, tenantID, id).Scan(&state, &version)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !cond(json.RawMessage(state), version) {
			return nil
		}
		_, err = tx.Exec(ctx, `UPDATE automation.connections SET status = $3, status_reason = $4, version = version + 1, updated_at = now()
			WHERE tenant_id = $1 AND id = $2`, tenantID, id, string(status), reason)
		set = err == nil
		return err
	})
	if err != nil {
		return false, serrors.Wrap(op, err)
	}
	return set, nil
}

func (s *store) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	const op = serrors.Op("automation.connectors.Store.Delete")
	if err := s.configured(); err != nil {
		return serrors.Wrap(op, err)
	}
	// The row stays locked from the check to the delete: whoever starts using the connection (enabling a
	// graph version) locks it FOR SHARE, so a use cannot appear in between. A TxUsageIndex reads the
	// catalog first and checks the uses in this transaction: holding the lock, the delete needs no
	// second pool connection, which an enable waiting for the row may hold.
	// The check reports ErrUsageChanged when a status change between the catalog read and the lock
	// (another connection reauthorized, this one reconnected) changed what a graph resolves to: the
	// catalog is read again.
	var err error
	for attempt := 0; attempt < maxUsageAttempts; attempt++ {
		if err = s.deleteOnce(ctx, tenantID, id); !errors.Is(err, ErrUsageChanged) {
			break
		}
	}
	return serrors.Wrap(op, err)
}

// maxUsageAttempts bounds the Delete retries after ErrUsageChanged.
const maxUsageAttempts = 5

func (s *store) deleteOnce(ctx context.Context, tenantID, id uuid.UUID) error {
	var check UsesCheck
	if tu, ok := s.usage.(TxUsageIndex); ok {
		c, err := tu.PrepareUses(ctx, tenantID)
		if err != nil {
			return err
		}
		check = c
	}
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var locked uuid.UUID
		err := tx.QueryRow(ctx, `SELECT id FROM automation.connections WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, tenantID, id).Scan(&locked)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if s.usage != nil {
			var uses []Use
			var err error
			if check != nil {
				uses, err = check(ctx, tx, id)
			} else {
				uses, err = s.usage.Uses(ctx, tenantID, id)
			}
			if err != nil {
				return err
			}
			if len(uses) > 0 {
				return &InUseError{Uses: uses}
			}
		}
		_, err = tx.Exec(ctx, `DELETE FROM automation.connections WHERE tenant_id = $1 AND id = $2`, tenantID, id)
		return err
	})
}

func (s *store) UpdateInbound(ctx context.Context, tenantID, id uuid.UUID, fn func(state json.RawMessage) (json.RawMessage, error)) error {
	const op = serrors.Op("automation.connectors.Store.UpdateInbound")
	if err := s.configured(); err != nil {
		return serrors.Wrap(op, err)
	}
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var state []byte
		err := tx.QueryRow(ctx, `SELECT inbound FROM automation.connections WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, tenantID, id).Scan(&state)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		next, err := fn(json.RawMessage(state))
		if err != nil {
			return err
		}
		if len(next) == 0 {
			next = json.RawMessage(`{}`)
		}
		if !json.Valid(next) || len(next) > maxInboundBytes {
			return fmt.Errorf("%w: inbound state is not JSON of at most 1 MiB", ErrInvalid)
		}
		_, err = tx.Exec(ctx, `UPDATE automation.connections SET inbound = $3, updated_at = now() WHERE tenant_id = $1 AND id = $2`, tenantID, id, []byte(next))
		return err
	})
	return serrors.Wrap(op, err)
}

// Rewrap walks the rows of older keys in id order, a batch per transaction, and re-seals those it can
// decrypt with the active key, until it rewrapped batch rows or none are left. It waits for rows another
// transaction holds (an OAuth2 refresh) instead of skipping them, so 0 means no row of an older key is
// left, apart from those reported as undecryptable. A row whose key is gone
// or that does not decrypt is left as is and reported in the error after the walk.
func (s *store) Rewrap(ctx context.Context, batch int) (int, error) {
	const op = serrors.Op("automation.connectors.Store.Rewrap")
	if err := s.configured(); err != nil {
		return 0, serrors.Wrap(op, err)
	}
	if batch <= 0 {
		batch = 100
	}
	activeID, _ := s.keys.Active()
	var (
		done, skipped int
		after         = uuid.Nil
	)
	for done < batch {
		var seen, batchDone, batchSkipped int
		err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
			rows, err := tx.Query(ctx, `SELECT `+connectionColumns+`, nonce, secret FROM automation.connections
				WHERE key_id <> $1 AND id > $2 ORDER BY id LIMIT $3 FOR UPDATE`, activeID, after, batch-done)
			if err != nil {
				return err
			}
			type sealed struct {
				c             Connection
				nonce, secret []byte
			}
			var found []sealed
			for rows.Next() {
				var r sealed
				r.c, err = scanConnection(rows, &r.nonce, &r.secret)
				if err != nil {
					rows.Close()
					return err
				}
				found = append(found, r)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
			seen = len(found)
			for _, r := range found {
				after = r.c.ID
				creds, err := s.decrypt(r.c, r.nonce, r.secret)
				if err != nil {
					batchSkipped++
					continue
				}
				keyID, nonce, secret, err := s.encrypt(r.c, creds)
				if err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `UPDATE automation.connections SET key_id = $3, nonce = $4, secret = $5 WHERE tenant_id = $1 AND id = $2`,
					r.c.TenantID, r.c.ID, keyID, nonce, secret); err != nil {
					return err
				}
				batchDone++
			}
			return nil
		})
		if err != nil {
			return done, serrors.Wrap(op, err) // the failed batch rolled back and is not counted
		}
		done += batchDone
		skipped += batchSkipped
		if seen == 0 {
			break
		}
	}
	if skipped > 0 {
		return done, serrors.Wrap(op, fmt.Errorf("%w: %d rows of older keys could not be rewrapped (key missing or row changed)", ErrUndecryptable, skipped))
	}
	return done, nil
}

// Open returns the connection a call runs with: a tenant connection that is connected, or the platform
// credential of a kind available to the tenant by its policy.
func (s *store) Open(ctx context.Context, tenantID uuid.UUID, ref string) (catalog.ConnectionInfo, Credentials, error) {
	const op = serrors.Op("automation.connectors.Store.Open")
	if kind, ok := strings.CutPrefix(ref, catalog.PlatformPrefix); ok {
		k := catalog.ConnectorKind(kind)
		pc, isCatalog := s.platform.(PlatformCatalog)
		if !isCatalog {
			return catalog.ConnectionInfo{}, nil, serrors.Wrap(op, ErrNotFound)
		}
		available, err := s.platformAvailable(ctx, tenantID, pc, k)
		if err != nil {
			return catalog.ConnectionInfo{}, nil, serrors.Wrap(op, err)
		}
		if !available {
			return catalog.ConnectionInfo{}, nil, serrors.Wrap(op, ErrNotFound)
		}
		creds, _, _ := s.platform.Platform(k)
		return catalog.ConnectionInfo{ID: ref, Kind: k, Name: "platform", Platform: true}, creds, nil
	}
	id, err := uuid.Parse(ref)
	if err != nil {
		return catalog.ConnectionInfo{}, nil, serrors.Wrap(op, ErrNotFound)
	}
	c, creds, err := s.load(ctx, s.db, tenantID, id, false)
	if err != nil {
		return catalog.ConnectionInfo{}, nil, serrors.Wrap(op, err)
	}
	switch c.Status {
	case StatusConnected:
	case StatusNeedsReauth:
		return catalog.ConnectionInfo{}, nil, serrors.Wrap(op, ErrNeedsReauth)
	case StatusError, StatusDisabled:
		return catalog.ConnectionInfo{}, nil, serrors.Wrap(op, ErrNotConnected)
	default:
		return catalog.ConnectionInfo{}, nil, serrors.Wrap(op, ErrNotConnected)
	}
	return catalog.ConnectionInfo{ID: c.ID.String(), Kind: c.Kind, Name: c.Name}, creds, nil
}

// now is the clock of token expiry and OAuth2 state.
var now = time.Now
