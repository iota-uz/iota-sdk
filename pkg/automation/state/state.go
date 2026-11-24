// Package state is the memory of an automation between its runs: key-value pairs per tenant and automation
// (the table automation.state) that the set_state node writes and the CEL variable `state` reads
// (state.get(key) is the value or null, state.has(key) tells whether the key is there).
//
// A key may expire: an expired key is not seen by any read, whether or not the background sweep (Purge) has
// removed its row yet. A write takes the transaction it is given, so a node's write commits atomically with
// the step of the run.
package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Limits of the memory.
const (
	// MaxKeyChars is the longest key, in characters.
	MaxKeyChars = 200
	// MaxValueBytes is the largest value, as JSON.
	MaxValueBytes = 16 << 10
	// MaxKeys is the most keys an automation holds at a time (expired ones do not count).
	MaxKeys = 10_000
	// MinTTL and MaxTTL bound the lifetime of a key that expires.
	MinTTL = time.Minute
	MaxTTL = 365 * 24 * time.Hour
)

// Codes of a refused write. They are the codes of a failed set_state node (output.error.code).
const (
	CodeValueTooLarge = "state_value_too_large"
	CodeKeyInvalid    = "state_key_invalid"
	CodeLimitExceeded = "state_limit_exceeded"
	CodeTTLInvalid    = "state_ttl_invalid"
)

// Error is a write the memory refuses; Code is one of the Code constants.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func refuse(code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Entry is one key of the memory.
type Entry struct {
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value"`
	ExpiresAt *time.Time      `json:"expires_at,omitempty"`
	UpdatedAt time.Time       `json:"updated_at"`
	// UpdatedByRun is the run whose step wrote the value; nil for an edit by a person or an agent.
	UpdatedByRun *uuid.UUID `json:"updated_by_run,omitempty"`
}

// Querier is what a read of one key needs.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// DB is what the memory reads and writes through: a pool, or the transaction of a step. Begin makes a
// transaction (a savepoint inside one), so a write that checks a limit and then inserts is one unit.
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

const op = serrors.Op("automation.state")

// ValidateKey checks a key: 1 to MaxKeyChars characters of valid UTF-8, without NUL.
func ValidateKey(key string) error {
	switch {
	case key == "":
		return refuse(CodeKeyInvalid, "the key is empty")
	case !utf8.ValidString(key) || strings.ContainsRune(key, 0):
		return refuse(CodeKeyInvalid, "the key is not valid text")
	case utf8.RuneCountInString(key) > MaxKeyChars:
		return refuse(CodeKeyInvalid, "the key has %d characters, at most %d", utf8.RuneCountInString(key), MaxKeyChars)
	}
	return nil
}

// ValidateTTL checks a lifetime: zero (the key does not expire), or MinTTL to MaxTTL.
func ValidateTTL(ttl time.Duration) error {
	if ttl != 0 && (ttl < MinTTL || ttl > MaxTTL) {
		return refuse(CodeTTLInvalid, "the lifetime of a key is from 1 minute to 365 days")
	}
	return nil
}

// Encode is the JSON a value is stored as, refused over MaxValueBytes. The value is any JSON-encodable
// Go value or a json.RawMessage.
func Encode(value any) (json.RawMessage, error) {
	raw, ok := value.(json.RawMessage)
	if !ok {
		b, err := json.Marshal(value)
		if err != nil {
			return nil, refuse(CodeValueTooLarge, "the value is not JSON: %v", err)
		}
		raw = b
	}
	if len(raw) > MaxValueBytes {
		return nil, refuse(CodeValueTooLarge, "the value has %d bytes, at most %d", len(raw), MaxValueBytes)
	}
	if strings.Contains(string(raw), `\u0000`) {
		return nil, refuse(CodeValueTooLarge, "the value has a NUL character, which the store does not keep")
	}
	return raw, nil
}

// live is the condition of a row that is not expired.
const live = `(expires_at IS NULL OR expires_at > now())`

// Get reads a key; ok is false when it is absent or expired.
func Get(ctx context.Context, db Querier, tenant, automation uuid.UUID, key string) (Entry, bool, error) {
	var e Entry
	err := db.QueryRow(ctx, `
		SELECT key, value, expires_at, updated_at, updated_by_run FROM automation.state
		WHERE tenant_id = $1 AND automation_id = $2 AND key = $3 AND `+live,
		tenant, automation, key).Scan(&e.Key, &e.Value, &e.ExpiresAt, &e.UpdatedAt, &e.UpdatedByRun)
	if errors.Is(err, pgx.ErrNoRows) {
		return Entry{}, false, nil
	}
	if err != nil {
		return Entry{}, false, serrors.FromDB(op, err)
	}
	return e, true, nil
}

// Put writes a key: the value replaces the one there, expiring after ttl (0: never). byRun is the run
// whose step writes (nil: a person or an agent). The write is refused with *Error for a key, a value, a
// lifetime or a number of keys over the limits.
func Put(ctx context.Context, db DB, tenant, automation uuid.UUID, key string, value any, ttl time.Duration, byRun *uuid.UUID) (Entry, error) {
	if err := ValidateKey(key); err != nil {
		return Entry{}, err
	}
	if err := ValidateTTL(ttl); err != nil {
		return Entry{}, err
	}
	raw, err := Encode(value)
	if err != nil {
		return Entry{}, err
	}
	var out Entry
	err = pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		// An update of a live key needs no check of the count; only a new key does.
		e, ok, err := scanEntry(tx.QueryRow(ctx, `
			UPDATE automation.state
			SET value = $4, expires_at = CASE WHEN $5::text IS NULL THEN NULL ELSE now() + $5::text::interval END,
			    updated_at = now(), updated_by_run = $6
			WHERE tenant_id = $1 AND automation_id = $2 AND key = $3 AND `+live+`
			RETURNING key, value, expires_at, updated_at, updated_by_run`,
			tenant, automation, key, []byte(raw), intervalOf(ttl), byRun))
		if err != nil {
			return err
		}
		if ok {
			out = e
			return nil
		}
		// Writers of new keys of one automation take turns, so the count they check holds.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "automation.state:"+tenant.String()+":"+automation.String()); err != nil {
			return serrors.FromDB(op, err)
		}
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM automation.state WHERE tenant_id = $1 AND automation_id = $2 AND `+live, tenant, automation).Scan(&n); err != nil {
			return serrors.FromDB(op, err)
		}
		if n >= MaxKeys {
			return refuse(CodeLimitExceeded, "the automation holds %d keys, the most it may hold", MaxKeys)
		}
		e, _, err = scanEntry(tx.QueryRow(ctx, `
			INSERT INTO automation.state (tenant_id, automation_id, key, value, expires_at, updated_at, updated_by_run)
			VALUES ($1, $2, $3, $4, CASE WHEN $5::text IS NULL THEN NULL ELSE now() + $5::text::interval END, now(), $6)
			ON CONFLICT (tenant_id, automation_id, key) DO UPDATE
			SET value = EXCLUDED.value, expires_at = EXCLUDED.expires_at, updated_at = EXCLUDED.updated_at, updated_by_run = EXCLUDED.updated_by_run
			RETURNING key, value, expires_at, updated_at, updated_by_run`,
			tenant, automation, key, []byte(raw), intervalOf(ttl), byRun))
		out = e
		return err
	})
	if err != nil {
		return Entry{}, err
	}
	return out, nil
}

func scanEntry(row pgx.Row) (Entry, bool, error) {
	var e Entry
	err := row.Scan(&e.Key, &e.Value, &e.ExpiresAt, &e.UpdatedAt, &e.UpdatedByRun)
	if errors.Is(err, pgx.ErrNoRows) {
		return Entry{}, false, nil
	}
	if err != nil {
		return Entry{}, false, serrors.FromDB(op, err)
	}
	return e, true, nil
}

// intervalOf is a lifetime as a Postgres interval text; nil for none.
func intervalOf(ttl time.Duration) *string {
	if ttl <= 0 {
		return nil
	}
	s := fmt.Sprintf("%d seconds", int64(ttl/time.Second))
	return &s
}

// Delete removes a key; deleted is false when it was not there (or had expired).
func Delete(ctx context.Context, db DB, tenant, automation uuid.UUID, key string) (bool, error) {
	if err := ValidateKey(key); err != nil {
		return false, err
	}
	tag, err := db.Exec(ctx, `DELETE FROM automation.state WHERE tenant_id = $1 AND automation_id = $2 AND key = $3 AND `+live, tenant, automation, key)
	if err != nil {
		return false, serrors.FromDB(op, err)
	}
	return tag.RowsAffected() > 0, nil
}

// DeleteAll removes every key of an automation (the automation is deleted).
func DeleteAll(ctx context.Context, db DB, tenant, automation uuid.UUID) error {
	_, err := db.Exec(ctx, `DELETE FROM automation.state WHERE tenant_id = $1 AND automation_id = $2`, tenant, automation)
	return serrors.FromDB(op, err)
}

// Filter selects the keys of a list.
type Filter struct {
	// Search is a substring of the key, without case.
	Search string
	// After is the key a page continues after (Page.Next).
	After string
	// Limit is the page size: 1 to 200, default 50.
	Limit int
}

// Page is a page of keys, in key order. Next is the After of the next page; empty at the end.
type Page struct {
	Items []Entry `json:"items"`
	Next  string  `json:"next,omitempty"`
	// Total is how many live keys the automation holds (the search ignored).
	Total int `json:"total"`
}

// List lists the live keys of an automation.
func List(ctx context.Context, db DB, tenant, automation uuid.UUID, f Filter) (Page, error) {
	limit := f.Limit
	switch {
	case limit <= 0:
		limit = 50
	case limit > 200:
		limit = 200
	}
	args := []any{tenant, automation}
	where := "tenant_id = $1 AND automation_id = $2 AND " + live
	if q := strings.TrimSpace(f.Search); q != "" {
		args = append(args, "%"+strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q)+"%")
		where += fmt.Sprintf(" AND key ILIKE $%d", len(args))
	}
	if f.After != "" {
		args = append(args, f.After)
		where += fmt.Sprintf(" AND key > $%d", len(args))
	}
	args = append(args, limit+1)
	rows, err := db.Query(ctx, `SELECT key, value, expires_at, updated_at, updated_by_run FROM automation.state WHERE `+where+
		fmt.Sprintf(` ORDER BY key LIMIT $%d`, len(args)), args...)
	if err != nil {
		return Page{}, serrors.FromDB(op, err)
	}
	defer rows.Close()
	page := Page{Items: []Entry{}}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.Key, &e.Value, &e.ExpiresAt, &e.UpdatedAt, &e.UpdatedByRun); err != nil {
			return Page{}, serrors.FromDB(op, err)
		}
		page.Items = append(page.Items, e)
	}
	if err := rows.Err(); err != nil {
		return Page{}, serrors.FromDB(op, err)
	}
	rows.Close()
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		page.Next = page.Items[limit-1].Key
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM automation.state WHERE tenant_id = $1 AND automation_id = $2 AND `+live, tenant, automation).Scan(&page.Total); err != nil {
		return Page{}, serrors.FromDB(op, err)
	}
	return page, nil
}

// purgeBatch is how many expired rows one statement removes.
const purgeBatch = 1000

// Purge removes the expired keys, in bounded batches (each its own statement). It returns how many it
// removed.
func Purge(ctx context.Context, db DB) (int64, error) {
	var total int64
	for {
		tag, err := db.Exec(ctx, `
			DELETE FROM automation.state WHERE (tenant_id, automation_id, key) IN (
				SELECT tenant_id, automation_id, key FROM automation.state
				WHERE expires_at IS NOT NULL AND expires_at <= now() LIMIT $1)`, purgeBatch)
		if err != nil {
			return total, serrors.FromDB(op, err)
		}
		total += tag.RowsAffected()
		if tag.RowsAffected() < purgeBatch || ctx.Err() != nil {
			return total, nil
		}
	}
}
