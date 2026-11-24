package connectors

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// CredentialChange is what CredentialLocker.UpdateCredentials writes: new credentials (nil keeps them)
// and a new status (empty keeps it).
type CredentialChange struct {
	Credentials  Credentials
	Status       ConnectionStatus
	StatusReason string
}

// CredentialLocker changes a connection's credentials under its row lock. A connector whose OAuth2
// endpoints and client are its connection's own (an MCP server) refreshes its token with it, so two calls
// never spend the same refresh token, and one that waited for the lock sees the token the other stored.
// The store from NewStore implements it.
type CredentialLocker interface {
	// UpdateCredentials calls fn with the connection and its credentials as they are under the lock and
	// writes the change it returns (nil: nothing). An error of fn rolls back and is returned as is.
	UpdateCredentials(ctx context.Context, tenantID, id uuid.UUID,
		fn func(c Connection, creds Credentials) (*CredentialChange, error)) (Connection, Credentials, error)
}

// Sealer seals small data with the active key of the keyring, so a connector can hand a browser a state
// (an OAuth2 authorization in progress) that only this application reads back. The store from NewStore
// implements it. purpose binds a sealed value to its use: one sealed for another purpose does not open.
type Sealer interface {
	Seal(purpose string, plain []byte) (string, error)
	Unseal(purpose, sealed string) ([]byte, error)
}

var (
	_ CredentialLocker = (*store)(nil)
	_ Sealer           = (*store)(nil)
)

func (s *store) UpdateCredentials(ctx context.Context, tenantID, id uuid.UUID,
	fn func(c Connection, creds Credentials) (*CredentialChange, error)) (Connection, Credentials, error) {
	const op = serrors.Op("automation.connectors.Store.UpdateCredentials")
	if err := s.configured(); err != nil {
		return Connection{}, nil, serrors.Wrap(op, err)
	}
	var (
		out      Connection
		outCreds Credentials
	)
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		c, creds, err := s.load(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		out, outCreds = c, creds
		change, err := fn(c, cloneCredentials(creds))
		if err != nil || change == nil {
			return err
		}
		next := c
		if change.Status != "" {
			if !change.Status.valid() {
				return fmt.Errorf("%w: status %q", ErrInvalid, change.Status)
			}
			if utf8.RuneCountInString(change.StatusReason) > maxStatusReason {
				return fmt.Errorf("%w: status reason is longer than %d", ErrInvalid, maxStatusReason)
			}
			next.Status, next.StatusReason = change.Status, change.StatusReason
		}
		if change.Credentials != nil {
			// The metadata stays while the tokens change: checked against both sets now, as a refresh of
			// the built-in OAuth2 does.
			next = scrubAcrossChange(next, creds, change.Credentials)
		}
		identity, err := json.Marshal(next.Identity)
		if err != nil {
			return err
		}
		saved, err := s.update(ctx, tx, next, identity, change.Credentials)
		if err != nil {
			return err
		}
		out = saved
		if change.Credentials != nil {
			outCreds = cloneCredentials(change.Credentials)
		}
		return nil
	})
	if err != nil {
		return Connection{}, nil, serrors.Wrap(op, err)
	}
	return out, outCreds, nil
}

func cloneCredentials(c Credentials) Credentials {
	out := make(Credentials, len(c))
	for k, v := range c {
		out[k] = v
	}
	return out
}

const sealAADPrefix = "automation-sealed:"

func (s *store) Seal(purpose string, plain []byte) (string, error) {
	if err := s.configured(); err != nil {
		return "", err
	}
	keyID, key := s.keys.Active()
	nonce, sealed, err := seal(key, plain, []byte(sealAADPrefix+purpose))
	if err != nil {
		return "", err
	}
	return keyID + "." + base64.RawURLEncoding.EncodeToString(append(nonce, sealed...)), nil
}

// errBadSealed is a sealed value that does not open: forged, of another purpose or of a key that is gone.
var errBadSealed = fmt.Errorf("%w: the sealed value does not open", ErrInvalid)

func (s *store) Unseal(purpose, sealed string) ([]byte, error) {
	if err := s.configured(); err != nil {
		return nil, err
	}
	i := strings.LastIndexByte(sealed, '.')
	if i <= 0 {
		return nil, errBadSealed
	}
	key, ok := s.keys.Key(sealed[:i])
	if !ok {
		return nil, errBadSealed
	}
	raw, err := base64.RawURLEncoding.DecodeString(sealed[i+1:])
	if err != nil || len(raw) < 12 {
		return nil, errBadSealed
	}
	plain, err := open(key, raw[:12], raw[12:], []byte(sealAADPrefix+purpose))
	if err != nil {
		return nil, errBadSealed
	}
	return plain, nil
}
