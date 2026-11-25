package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
	for range putAttempts {
		var current int
		err := s.db.QueryRow(ctx, `SELECT version FROM automation.secret_values WHERE tenant_id = $1 AND path = $2`,
			ref.TenantID, ref.Path).Scan(&current)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return secrets.Metadata{}, unavailableSecret(err)
		}
		next := current + 1
		nonce, ct, err := seal(key, plain, secretAAD(ref.TenantID, ref.Path, next))
		if err != nil {
			return secrets.Metadata{}, unavailableSecret(err)
		}
		// One statement writes the next version only over the version it was sealed against: a concurrent
		// Put makes it match no row and the attempt starts again from the new version.
		var md secrets.Metadata
		err = s.db.QueryRow(ctx, `INSERT INTO automation.secret_values AS v (tenant_id, path, version, key_id, nonce, ciphertext)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (tenant_id, path) DO UPDATE
			SET version = EXCLUDED.version, key_id = EXCLUDED.key_id, nonce = EXCLUDED.nonce,
				ciphertext = EXCLUDED.ciphertext, updated_at = now()
			WHERE v.version = $3::int - 1
			RETURNING version, created_at, updated_at`,
			ref.TenantID, ref.Path, next, keyID, nonce, ct).Scan(&md.Version, &md.CreatedAt, &md.UpdatedAt)
		switch {
		case err == nil:
			return md, nil
		case errors.Is(err, pgx.ErrNoRows):
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
	err := s.db.QueryRow(ctx, `SELECT version, key_id, nonce, ciphertext FROM automation.secret_values WHERE tenant_id = $1 AND path = $2`,
		ref.TenantID, ref.Path).Scan(&version, &keyID, &nonce, &ciphertx)
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
	err := s.db.QueryRow(ctx, `SELECT version, created_at, updated_at FROM automation.secret_values WHERE tenant_id = $1 AND path = $2`,
		ref.TenantID, ref.Path).Scan(&md.Version, &md.CreatedAt, &md.UpdatedAt)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return secrets.Metadata{}, secrets.ErrNotFound
	case err != nil:
		return secrets.Metadata{}, unavailableSecret(err)
	}
	return md, nil
}

// rewrapSecretValues re-seals up to limit values of older keys with the active one, a batch in one
// transaction; it returns how many it re-sealed and how many it could not (key gone, row changed).
func rewrapSecretValues(ctx context.Context, db *pgxpool.Pool, keys Keyring, limit int) (done, skipped int, err error) {
	activeID, activeKey := keys.Active()
	var afterTenant uuid.UUID
	afterPath := ""
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
				WHERE key_id <> $1 AND ($4 OR (tenant_id, path) > ($2, $3)) ORDER BY tenant_id, path LIMIT $5 FOR UPDATE`,
				activeID, afterTenant, afterPath, first, limit-done)
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
				afterTenant, afterPath = r.tenant, r.path
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
				if _, err := tx.Exec(ctx, `UPDATE automation.secret_values SET key_id = $3, nonce = $4, ciphertext = $5 WHERE tenant_id = $1 AND path = $2`,
					r.tenant, r.path, activeID, nonce, ct); err != nil {
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
