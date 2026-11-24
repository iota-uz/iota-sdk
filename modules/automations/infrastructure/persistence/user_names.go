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
