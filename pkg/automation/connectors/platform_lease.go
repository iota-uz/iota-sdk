package connectors

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
)

// PlatformLease is how long the platform credential's reservation of an account lasts after the last renewal of an
// instance that runs with the credential. Every such instance renews it (RenewPlatformLeases, every quarter of the
// lease), so the reservation outlives a start: during a rolling update that changes the platform credential, the
// instances that still run with the previous account keep it reserved while they serve it, and a new instance's start
// does not release it under them. Once the last of them stops, the account is free after the lease.
const PlatformLease = 2 * time.Minute

// PlatformStartError is the failure of the start of a platform credential (StartPlatformIdentity's start): the
// reservation itself was made, and the credential still sends.
type PlatformStartError struct{ Err error }

func (e *PlatformStartError) Error() string { return "automation connections: platform start: " + e.Err.Error() }
func (e *PlatformStartError) Unwrap() error { return e.Err }

func leaseInterval() string { return fmt.Sprintf("%d milliseconds", PlatformLease.Milliseconds()) }

// releaseExpiredPlatform releases the platform reservation of an account whose lease ran out, in the caller's
// transaction that holds the account's lock.
func releaseExpiredPlatform(ctx context.Context, tx pgx.Tx, kind catalog.ConnectorKind, external string) error {
	_, err := tx.Exec(ctx, `DELETE FROM automation.exclusive_identities WHERE platform AND kind = $1 AND external = $2 AND lease_until < now()`,
		string(kind), external)
	return err
}

// ReservePlatformIdentity gives the exclusive identity to the platform credential, or renews its lease, under the
// same lock as a connection's reservation. A connection that holds it already keeps it (ErrIdentityTaken): the
// platform's credential then cannot be set up for that account. The platform's reservations of other accounts of
// the kind are released only when their lease ran out (PrunePlatformIdentities): an instance that still runs with
// another account (a rolling update) keeps it.
func ReservePlatformIdentity(ctx context.Context, db *pgxpool.Pool, kind catalog.ConnectorKind, external string) error {
	return StartPlatformIdentity(ctx, db, kind, external, nil)
}

// StartPlatformIdentity is ReservePlatformIdentity that runs start (setting up the provider: a webhook) while the
// account's lock is held, after the reservation: two instances that start with the same credential set it up one
// after the other, and a tenant's save of the account (which takes the same lock) waits for it and then finds the
// account reserved. The reservation is kept when start fails; its failure is a *PlatformStartError.
func StartPlatformIdentity(ctx context.Context, db *pgxpool.Pool, kind catalog.ConnectorKind, external string, start func(ctx context.Context) error) error {
	if external == "" {
		if start == nil {
			return nil
		}
		if err := start(ctx); err != nil {
			return &PlatformStartError{Err: err}
		}
		return nil
	}
	var startErr error
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, identityLockKey(kind, external)); err != nil {
			return err
		}
		if err := releaseExpiredPlatform(ctx, tx, kind, external); err != nil {
			return err
		}
		holder, err := holderOf(ctx, tx, kind, external)
		switch {
		case err != nil:
			return err
		case holder.held && holder.platform:
			if _, err := tx.Exec(ctx, `UPDATE automation.exclusive_identities SET lease_until = now() + $3::interval WHERE platform AND kind = $1 AND external = $2`,
				string(kind), external, leaseInterval()); err != nil {
				return err
			}
		case holder.held:
			return ErrIdentityTaken
		default:
			if _, err := tx.Exec(ctx, `INSERT INTO automation.exclusive_identities (kind, external, platform, lease_until) VALUES ($1, $2, TRUE, now() + $3::interval)`,
				string(kind), external, leaseInterval()); err != nil {
				return err
			}
		}
		if start != nil {
			// The reservation is committed whatever start does: the credential is the platform's either way.
			startErr = start(ctx)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if startErr != nil {
		return &PlatformStartError{Err: startErr}
	}
	return nil
}

// PrunePlatformIdentities releases the platform reservations of a kind whose lease ran out: no instance runs with
// their account any more (the credential was changed or removed and every instance that had it stopped), so it is
// free to be connected as a tenant's own. Its webhook cannot be removed without its token, which is gone with the
// configuration; a tenant's connection replaces it.
func PrunePlatformIdentities(ctx context.Context, db *pgxpool.Pool, kind catalog.ConnectorKind) error {
	_, err := db.Exec(ctx, `DELETE FROM automation.exclusive_identities WHERE platform AND kind = $1 AND lease_until < now()`, string(kind))
	return err
}

// ReleasePlatformIdentities releases every platform reservation of a kind at once, whatever its lease: for an
// operator who knows that no instance runs with the platform credential any more.
func ReleasePlatformIdentities(ctx context.Context, db *pgxpool.Pool, kind catalog.ConnectorKind) error {
	_, err := db.Exec(ctx, `DELETE FROM automation.exclusive_identities WHERE platform AND kind = $1`, string(kind))
	return err
}

// IsPlatformStartError reports whether err is the failure of a platform start rather than of its reservation.
func IsPlatformStartError(err error) bool {
	var s *PlatformStartError
	return errors.As(err, &s)
}
