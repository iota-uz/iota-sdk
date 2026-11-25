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

func (e *PlatformStartError) Error() string {
	return "automation connections: platform start: " + e.Err.Error()
}
func (e *PlatformStartError) Unwrap() error { return e.Err }

func leaseInterval() string { return fmt.Sprintf("%d milliseconds", PlatformLease.Milliseconds()) }

// renewLease sets the platform reservation's lease from the database's current time (clock_timestamp: now() is the
// transaction's start). A reservation that is not there is an error: nothing may be set up without it.
func renewLease(ctx context.Context, tx pgx.Tx, kind catalog.ConnectorKind, external string) error {
	tag, err := tx.Exec(ctx, `UPDATE automation.exclusive_identities SET lease_until = clock_timestamp() + $3::interval WHERE platform AND kind = $1 AND external = $2`,
		string(kind), external, leaseInterval())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errLeaseLost
	}
	return nil
}

// errLeaseLost is a platform reservation that disappeared while it was renewed.
var errLeaseLost = errors.New("automation connections: the platform reservation was released while it was renewed")

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

// PlatformStartTimeout bounds the provider setup StartPlatformIdentity runs (setWebhook). It is shorter than the
// lease, so a request the provider receives late still lands while the committed reservation holds the account.
const PlatformStartTimeout = PlatformLease / 2

// StartPlatformIdentity is ReservePlatformIdentity followed by start (setting up the provider: a webhook).
//
// The reservation is committed before start runs, so it does not depend on the database session that runs start: if
// that session ends during the provider call (its advisory lock and transaction go with it), the committed reservation
// still holds the account for the lease, and a tenant's save of the account is refused until then. start runs under
// the account's lock in a second transaction (two instances that start with the same credential set it up one after
// the other, a tenant's save waits) with a deadline of at most PlatformStartTimeout that also ends while a quarter of
// the committed lease is still left: the lock wait counts against it, and with less left the setup is not attempted.
// Telegram has no conditional setWebhook, so this is a bound, not a fence: a request the provider applies more than
// that quarter of the lease after the client's deadline, from an instance that stopped renewing (a crash), could still
// land after a tenant took the account. A live instance renews the lease every quarter of it, so its reservation does
// not run out under a request of its own. The reservation is kept when start fails; its failure (and a setup that was
// not attempted) is a *PlatformStartError.
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
	if err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if err := lockIdentity(ctx, tx, kind, external); err != nil {
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
			return renewLease(ctx, tx, kind, external)
		case holder.held:
			return ErrIdentityTaken
		}
		_, err = tx.Exec(ctx, `INSERT INTO automation.exclusive_identities (kind, external, platform, lease_until) VALUES ($1, $2, TRUE, clock_timestamp() + $3::interval)`,
			string(kind), external, leaseInterval())
		return err
	}); err != nil {
		return err
	}
	if start == nil {
		return nil
	}
	var startErr error
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if err := lockIdentity(ctx, tx, kind, external); err != nil {
			return err
		}
		// The reservation committed above, read again under the lock: nothing is set up without it, and the setup
		// ends while a quarter of the committed lease is still left (the lock wait has used some of it).
		// The deadline counts from before the lease is read: whatever passes between the database's reading and the
		// deadline's start is taken from the budget, so the deadline never passes later than the margin allows.
		readFrom := time.Now()
		var leftMs float64
		err := tx.QueryRow(ctx, `SELECT extract(epoch FROM lease_until - clock_timestamp()) * 1000 FROM automation.exclusive_identities
			WHERE platform AND kind = $1 AND external = $2 FOR UPDATE`, string(kind), external).Scan(&leftMs)
		if errors.Is(err, pgx.ErrNoRows) {
			return errLeaseLost
		}
		if err != nil {
			return err
		}
		budget := startBudget(time.Duration(leftMs) * time.Millisecond)
		if budget <= 0 {
			startErr = errLeaseTooShort
			return nil
		}
		startCtx, cancel := context.WithDeadline(ctx, readFrom.Add(budget))
		startErr = start(startCtx)
		cancel()
		// The lease counts from the moment start returned (the lock wait and the provider call would shorten it).
		return renewLease(ctx, tx, kind, external)
	})
	if err != nil {
		return err
	}
	if startErr != nil {
		return &PlatformStartError{Err: startErr}
	}
	return nil
}

// platformStartMargin is the committed lease that is still left when the provider setup's deadline passes.
const platformStartMargin = PlatformLease / 4

// startBudget is the deadline of the provider setup for the committed lease left: at most PlatformStartTimeout, and
// never past the point where less than platformStartMargin of the lease would be left.
func startBudget(left time.Duration) time.Duration {
	return min(PlatformStartTimeout, left-platformStartMargin)
}

// errLeaseTooShort is a setup that would not end inside the committed lease (the lock wait used it up).
var errLeaseTooShort = errors.New("automation connections: too little of the platform reservation's lease is left to set the provider up")

func lockIdentity(ctx context.Context, tx pgx.Tx, kind catalog.ConnectorKind, external string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, identityLockKey(kind, external))
	return err
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
