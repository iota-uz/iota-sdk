package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/automation/secrets"
)

// sealedSecrets is secrets.Store over automation.secret_values: the value of a secret, sealed with the
// connections' keyring. The additional data binds a ciphertext to its tenant, path and version, so a row
// moved or rolled back outside the store does not open.
type sealedSecrets struct {
	db   *pgxpool.Pool
	keys Keyring
}

// NewSealedSecrets is the first adapter of the secret storage port (secrets.Store): AES-256-GCM per
// tenant row in PostgreSQL, with the keyring of the connections (Store.Rewrap re-seals the values too).
// keys nil: every call is secrets.ErrUnavailable wrapping ErrNotConfigured.
func NewSealedSecrets(db *pgxpool.Pool, keys Keyring) secrets.Store {
	return &sealedSecrets{db: db, keys: keys}
}

const putAttempts = 8

// errWriteFenced: a Put that reached the store after its ref's NotAfter; nothing was written.
var errWriteFenced = errors.New("the write's deadline has passed")

func secretAAD(tenantID uuid.UUID, path string, version int) []byte {
	return []byte(fmt.Sprintf("secret|%s|%s|%d", tenantID, path, version))
}

func unavailableSecret(err error) error {
	if errors.Is(err, secrets.ErrUnavailable) {
		return err
	}
	// The error of the driver never carries a value: the statements bind it only as a ciphertext.
	return fmt.Errorf("%w: %w", secrets.ErrUnavailable, err)
}

func (s *sealedSecrets) check(ref secrets.Ref) error {
	if ref.TenantID == uuid.Nil || ref.Path == "" {
		return secrets.ErrForbidden
	}
	if s.db == nil {
		return unavailableSecret(errors.New("no database"))
	}
	return nil
}

func (s *sealedSecrets) Put(ctx context.Context, ref secrets.Ref, data map[string]string) (secrets.Metadata, error) {
	if err := s.check(ref); err != nil {
		return secrets.Metadata{}, err
	}
	if s.keys == nil {
		return secrets.Metadata{}, unavailableSecret(ErrNotConfigured)
	}
	plain, err := json.Marshal(data)
	if err != nil {
		return secrets.Metadata{}, fmt.Errorf("%w: value", secrets.ErrInvalid)
	}
	keyID, key := s.keys.Active()
	var fence *time.Time
	if !ref.NotAfter.IsZero() {
		fence = &ref.NotAfter
	}
	for range putAttempts {
		var latest int
		if err := s.db.QueryRow(ctx, `SELECT COALESCE(max(version), 0) FROM automation.secret_values WHERE tenant_id = $1 AND path = $2`,
			ref.TenantID, ref.Path).Scan(&latest); err != nil {
			return secrets.Metadata{}, unavailableSecret(err)
		}
		next := latest + 1
		nonce, ct, err := seal(key, plain, secretAAD(ref.TenantID, ref.Path, next))
		if err != nil {
			return secrets.Metadata{}, unavailableSecret(err)
		}
		// The next version is a row of its own: the earlier ones stay readable until they are pruned, and a
		// concurrent Put of the same version makes this one conflict and start again from the new latest.
		// The fence is checked by the database's clock in the inserting statement itself, which commits as it
		// ends: a version is never written after ref.NotAfter, however late this call runs.
		var at time.Time
		err = s.db.QueryRow(ctx, `INSERT INTO automation.secret_values (tenant_id, path, version, key_id, nonce, ciphertext)
			SELECT $1, $2, $3, $4, $5, $6 WHERE $7::timestamptz IS NULL OR clock_timestamp() < $7::timestamptz
			ON CONFLICT DO NOTHING RETURNING created_at`,
			ref.TenantID, ref.Path, next, keyID, nonce, ct, fence).Scan(&at)
		switch {
		case err == nil:
			// The version this call inserted, never the latest: a concurrent Put may already have added a
			// later one, and the caller publishes exactly what it wrote.
			md := secrets.Metadata{Version: next, CreatedAt: at, UpdatedAt: at}
			var first *time.Time
			if err := s.db.QueryRow(ctx, `SELECT min(created_at) FROM automation.secret_values WHERE tenant_id = $1 AND path = $2`,
				ref.TenantID, ref.Path).Scan(&first); err == nil && first != nil {
				md.CreatedAt = *first
			}
			return md, nil
		case errors.Is(err, pgx.ErrNoRows):
			if fence != nil {
				var passed bool
				if err := s.db.QueryRow(ctx, `SELECT clock_timestamp() >= $1::timestamptz`, fence).Scan(&passed); err != nil {
					return secrets.Metadata{}, unavailableSecret(err)
				}
				if passed {
					return secrets.Metadata{}, unavailableSecret(errWriteFenced)
				}
			}
			continue
		default:
			return secrets.Metadata{}, unavailableSecret(err)
		}
	}
	return secrets.Metadata{}, unavailableSecret(errors.New("the value is written concurrently"))
}

func (s *sealedSecrets) Delete(ctx context.Context, ref secrets.Ref) error {
	if err := s.check(ref); err != nil {
		return err
	}
	if _, err := s.db.Exec(ctx, `DELETE FROM automation.secret_values WHERE tenant_id = $1 AND path = $2`, ref.TenantID, ref.Path); err != nil {
		return unavailableSecret(err)
	}
	return nil
}

func (s *sealedSecrets) List(ctx context.Context, root secrets.Ref) ([]string, error) {
	if err := s.check(root); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT DISTINCT path FROM automation.secret_values WHERE tenant_id = $1 AND starts_with(path, $2) ORDER BY path`,
		root.TenantID, root.Path)
	if err != nil {
		return nil, unavailableSecret(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, unavailableSecret(err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, unavailableSecret(err)
	}
	return out, nil
}

func (s *sealedSecrets) Prune(ctx context.Context, ref secrets.Ref, keepFrom int) error {
	if err := s.check(ref); err != nil {
		return err
	}
	if _, err := s.db.Exec(ctx, `DELETE FROM automation.secret_values WHERE tenant_id = $1 AND path = $2 AND version < $3`,
		ref.TenantID, ref.Path, keepFrom); err != nil {
		return unavailableSecret(err)
	}
	return nil
}

func (s *sealedSecrets) Read(ctx context.Context, ref secrets.Ref) (secrets.Value, error) {
	if err := s.check(ref); err != nil {
		return secrets.Value{}, err
	}
	if s.keys == nil {
		return secrets.Value{}, unavailableSecret(ErrNotConfigured)
	}
	// One statement: the version and the ciphertext sealed for it are one row.
	var (
		version         int
		keyID           string
		nonce, ciphertx []byte
	)
	err := s.db.QueryRow(ctx, `SELECT version, key_id, nonce, ciphertext FROM automation.secret_values
		WHERE tenant_id = $1 AND path = $2 AND ($3 = 0 OR version = $3) ORDER BY version DESC LIMIT 1`,
		ref.TenantID, ref.Path, ref.Version).Scan(&version, &keyID, &nonce, &ciphertx)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return secrets.Value{}, secrets.ErrNotFound
	case err != nil:
		return secrets.Value{}, unavailableSecret(err)
	}
	key, ok := s.keys.Key(keyID)
	if !ok {
		return secrets.Value{}, unavailableSecret(fmt.Errorf("%w: key %q is not in the keyring", ErrUndecryptable, keyID))
	}
	plain, err := open(key, nonce, ciphertx, secretAAD(ref.TenantID, ref.Path, version))
	if err != nil {
		return secrets.Value{}, unavailableSecret(err)
	}
	var data map[string]string
	if err := json.Unmarshal(plain, &data); err != nil {
		return secrets.Value{}, unavailableSecret(ErrUndecryptable)
	}
	return secrets.Value{Data: data, Version: version}, nil
}

func (s *sealedSecrets) Metadata(ctx context.Context, ref secrets.Ref) (secrets.Metadata, error) {
	if err := s.check(ref); err != nil {
		return secrets.Metadata{}, err
	}
	var md secrets.Metadata
	var version *int
	var created, updated *time.Time
	err := s.db.QueryRow(ctx, `SELECT max(version), min(created_at), max(created_at) FROM automation.secret_values WHERE tenant_id = $1 AND path = $2`,
		ref.TenantID, ref.Path).Scan(&version, &created, &updated)
	switch {
	case err != nil:
		return secrets.Metadata{}, unavailableSecret(err)
	case version == nil:
		return secrets.Metadata{}, secrets.ErrNotFound
	}
	md.Version, md.CreatedAt, md.UpdatedAt = *version, *created, *updated
	return md, nil
}

// rewrapSecretValues re-seals up to limit values of older keys with the active one, a batch in one
// transaction; it returns how many it re-sealed and how many it could not (key gone, row changed).
func rewrapSecretValues(ctx context.Context, db *pgxpool.Pool, keys Keyring, limit int) (int, int, error) {
	var done, skipped int
	activeID, activeKey := keys.Active()
	var afterTenant uuid.UUID
	afterPath := ""
	afterVersion := 0
	first := true
	for done < limit {
		var seen, batchDone, batchSkipped int
		err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
			type row struct {
				tenant        uuid.UUID
				path, keyID   string
				version       int
				nonce, cipher []byte
			}
			rows, err := tx.Query(ctx, `SELECT tenant_id, path, version, key_id, nonce, ciphertext FROM automation.secret_values
				WHERE key_id <> $1 AND ($5 OR (tenant_id, path, version) > ($2, $3, $4)) ORDER BY tenant_id, path, version LIMIT $6 FOR UPDATE`,
				activeID, afterTenant, afterPath, afterVersion, first, limit-done)
			if err != nil {
				return err
			}
			var found []row
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.tenant, &r.path, &r.version, &r.keyID, &r.nonce, &r.cipher); err != nil {
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
				afterTenant, afterPath, afterVersion = r.tenant, r.path, r.version
				old, ok := keys.Key(r.keyID)
				if !ok {
					batchSkipped++
					continue
				}
				aad := secretAAD(r.tenant, r.path, r.version)
				plain, err := open(old, r.nonce, r.cipher, aad)
				if err != nil {
					batchSkipped++
					continue
				}
				nonce, ct, err := seal(activeKey, plain, aad)
				if err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `UPDATE automation.secret_values SET key_id = $4, nonce = $5, ciphertext = $6 WHERE tenant_id = $1 AND path = $2 AND version = $3`,
					r.tenant, r.path, r.version, activeID, nonce, ct); err != nil {
					return err
				}
				batchDone++
			}
			return nil
		})
		if err != nil {
			return done, skipped, err
		}
		first = false
		done += batchDone
		skipped += batchSkipped
		if seen == 0 {
			break
		}
	}
	return done, skipped, nil
}
