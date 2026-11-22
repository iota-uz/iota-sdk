// Package audience implements workflow.TaskAudience over the core's users,
// roles and groups: who a human task's role or group assignee covers, judged
// by the rows as they are now (an assignment removed after the task was
// created no longer counts).
package audience

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const op = serrors.Op("extruntime.workflow.audience")

// Core is the TaskAudience of the core module. It reads through the
// transaction in the context (composables.WithTx) when there is one, as the
// engine passes while it holds a task lock, and through DB otherwise. A member is an active (not
// blocked) user of the tenant who:
//   - user: is that user;
//   - permission: holds the permission directly, through a role or through a
//     group's role;
//   - role: has the role of the tenant directly or through a group of the
//     tenant;
//   - group: is in the group of the tenant.
type Core struct {
	DB *pgxpool.Pool
}

var _ workflow.TaskAudience = (*Core)(nil)

type queryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (c *Core) q(ctx context.Context) queryer {
	if tx, ok := ctx.Value(constants.TxKey).(pgx.Tx); ok && tx != nil {
		return tx
	}
	return c.DB
}

// New returns the core audience over db.
func New(db *pgxpool.Pool) *Core { return &Core{DB: db} }

// members is the set of active users of tenant $1 an assignee covers; $2 is
// the reference (a user id, a permission name, a role id or a group uuid).
func members(kind string) (string, error) {
	const active = `SELECT u.id FROM users u WHERE u.tenant_id = $1 AND NOT u.is_blocked AND `
	switch kind {
	case workflow.TaskAssignUser:
		return active + `u.id = $2::bigint`, nil
	case workflow.TaskAssignGroup:
		return active + `EXISTS (
			SELECT 1 FROM group_users gu JOIN user_groups g ON g.id = gu.group_id
			WHERE gu.user_id = u.id AND g.tenant_id = $1 AND g.id = $2::uuid)`, nil
	case workflow.TaskAssignRole:
		return active + `(
			EXISTS (SELECT 1 FROM user_roles ur JOIN roles r ON r.id = ur.role_id
			        WHERE ur.user_id = u.id AND r.tenant_id = $1 AND r.id = $2::bigint)
			OR EXISTS (SELECT 1 FROM group_users gu JOIN user_groups g ON g.id = gu.group_id
			           JOIN group_roles gr ON gr.group_id = g.id JOIN roles r ON r.id = gr.role_id
			           WHERE gu.user_id = u.id AND g.tenant_id = $1 AND r.tenant_id = $1 AND r.id = $2::bigint))`, nil
	case workflow.TaskAssignPermission:
		return active + `(
			EXISTS (SELECT 1 FROM user_permissions up JOIN permissions p ON p.id = up.permission_id
			        WHERE up.user_id = u.id AND p.name = $2)
			OR EXISTS (SELECT 1 FROM user_roles ur JOIN roles r ON r.id = ur.role_id
			           JOIN role_permissions rp ON rp.role_id = r.id JOIN permissions p ON p.id = rp.permission_id
			           WHERE ur.user_id = u.id AND r.tenant_id = $1 AND p.name = $2)
			OR EXISTS (SELECT 1 FROM group_users gu JOIN user_groups g ON g.id = gu.group_id
			           JOIN group_roles gr ON gr.group_id = g.id JOIN roles r ON r.id = gr.role_id
			           JOIN role_permissions rp ON rp.role_id = r.id JOIN permissions p ON p.id = rp.permission_id
			           WHERE gu.user_id = u.id AND g.tenant_id = $1 AND r.tenant_id = $1 AND p.name = $2))`, nil
	}
	return "", serrors.New(serrors.Invalid, fmt.Sprintf("extruntime: task assignee kind %q is not supported", kind))
}

// ref is the query argument of an assignee; ok is false for a reference that
// cannot name anything (then nobody is a member).
func ref(a workflow.TaskAssignee) (string, bool) {
	switch a.Kind {
	case workflow.TaskAssignUser:
		if a.UserID == nil {
			return "", false
		}
		return strconv.FormatInt(*a.UserID, 10), true
	case workflow.TaskAssignRole:
		_, err := strconv.ParseInt(a.Ref, 10, 64)
		return a.Ref, err == nil
	case workflow.TaskAssignGroup:
		// The canonical form: uuid.Parse also takes forms PostgreSQL does not
		// (urn:uuid:…).
		id, err := uuid.Parse(a.Ref)
		if err != nil {
			return "", false
		}
		return id.String(), true
	}
	return a.Ref, a.Ref != ""
}

// IsMember implements workflow.TaskAudience.
func (c *Core) IsMember(ctx context.Context, tenantID uuid.UUID, userID int64, a workflow.TaskAssignee) (bool, error) {
	q, err := members(a.Kind)
	if err != nil {
		return false, err
	}
	r, ok := ref(a)
	if !ok {
		return false, nil
	}
	var member bool
	if err := c.q(ctx).QueryRow(ctx, `SELECT EXISTS (`+q+` AND u.id = $3)`, tenantID, r, userID).Scan(&member); err != nil {
		return false, serrors.FromDB(op, err)
	}
	return member, nil
}

// Users implements workflow.TaskAudience: members by id, at most limit.
func (c *Core) Users(ctx context.Context, tenantID uuid.UUID, a workflow.TaskAssignee, limit int) ([]int64, error) {
	q, err := members(a.Kind)
	if err != nil {
		return nil, err
	}
	r, ok := ref(a)
	if !ok || limit <= 0 {
		return []int64{}, nil
	}
	rows, err := c.q(ctx).Query(ctx, q+` ORDER BY u.id LIMIT $3`, tenantID, r, limit)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	return ids, nil
}
