package persistence

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// UserNames reads the display names of a tenant's users.
type UserNames struct{ db *pgxpool.Pool }

// NewUserNames is the names of the users table.
func NewUserNames(db *pgxpool.Pool) *UserNames { return &UserNames{db: db} }

// Names are the first and last names of ids that are users of the tenant; other ids are left out.
func (u *UserNames) Names(ctx context.Context, tenantID uuid.UUID, ids []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := u.db.Query(ctx, `SELECT id, coalesce(first_name, ''), coalesce(last_name, '') FROM users WHERE tenant_id = $1 AND id = ANY($2)`, tenantID, ids)
	if err != nil {
		return nil, serrors.FromDB("automations.UserNames", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var first, last string
		if err := rows.Scan(&id, &first, &last); err != nil {
			return nil, serrors.FromDB("automations.UserNames", err)
		}
		out[id] = strings.TrimSpace(first + " " + last)
	}
	return out, serrors.FromDB("automations.UserNames", rows.Err())
}

// UserOption is a user a picker offers.
type UserOption struct {
	ID    int64
	Name  string
	Email string
}

// SearchActive lists active (not blocked) users of the tenant whose name or email contains query,
// without the excluded ids, by name, at most limit.
func (u *UserNames) SearchActive(ctx context.Context, tenantID uuid.UUID, query string, exclude []int64, limit int) ([]UserOption, error) {
	const op = "automations.UserNames.SearchActive"
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if exclude == nil {
		exclude = []int64{}
	}
	pattern := "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(strings.TrimSpace(query)) + "%"
	rows, err := u.db.Query(ctx, `
		SELECT id, coalesce(first_name, ''), coalesce(last_name, ''), coalesce(email, '') FROM users
		WHERE tenant_id = $1 AND NOT is_blocked AND type <> 'system' AND NOT (id = ANY($2))
		  AND (coalesce(first_name, '') || ' ' || coalesce(last_name, '') ILIKE $3 OR coalesce(email, '') ILIKE $3)
		ORDER BY lower(coalesce(first_name, '') || ' ' || coalesce(last_name, '')), id
		LIMIT $4`, tenantID, exclude, pattern, limit)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	defer rows.Close()
	out := []UserOption{}
	for rows.Next() {
		var o UserOption
		var first, last string
		if err := rows.Scan(&o.ID, &first, &last, &o.Email); err != nil {
			return nil, serrors.FromDB(op, err)
		}
		o.Name = strings.TrimSpace(first + " " + last)
		out = append(out, o)
	}
	return out, serrors.FromDB(op, rows.Err())
}
