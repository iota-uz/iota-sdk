package persistence

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const notificationRecipientColumns = "SELECT u.id, u.first_name, u.last_name, u.email FROM users u"

var notificationRecipientSearchEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

type NotificationRecipientDirectory struct {
	groupIDs []uuid.UUID
}

// NewNotificationRecipientDirectory lists the tenant's active users. With
// groupIDs it lists only members of at least one of those groups.
func NewNotificationRecipientDirectory(groupIDs ...uuid.UUID) notifications.RecipientDirectory {
	return &NotificationRecipientDirectory{groupIDs: groupIDs}
}

func (r *NotificationRecipientDirectory) Search(ctx context.Context, query notifications.RecipientQuery) ([]notifications.Recipient, error) {
	const op = "NotificationRecipientDirectory.Search"
	where, args, err := r.scope(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	for _, term := range strings.Fields(query.Search) {
		args = append(args, "%"+notificationRecipientSearchEscaper.Replace(term)+"%")
		where = append(where, fmt.Sprintf("concat_ws(' ', u.first_name, u.middle_name, u.last_name, u.email) ILIKE $%d", len(args)))
	}
	args = append(args, query.Limit, query.Offset)
	sql := fmt.Sprintf("%s WHERE %s ORDER BY u.first_name, u.last_name, u.id LIMIT $%d OFFSET $%d", notificationRecipientColumns, strings.Join(where, " AND "), len(args)-1, len(args))
	recipients, err := queryNotificationRecipients(ctx, sql, args)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return recipients, nil
}

func (r *NotificationRecipientDirectory) GetByIDs(ctx context.Context, ids []uint) ([]notifications.Recipient, error) {
	const op = "NotificationRecipientDirectory.GetByIDs"
	if len(ids) == 0 {
		return []notifications.Recipient{}, nil
	}
	where, args, err := r.scope(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	args = append(args, ids)
	where = append(where, fmt.Sprintf("u.id = ANY($%d)", len(args)))
	recipients, err := queryNotificationRecipients(ctx, notificationRecipientColumns+" WHERE "+strings.Join(where, " AND "), args)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return recipients, nil
}

func (r *NotificationRecipientDirectory) scope(ctx context.Context) ([]string, []any, error) {
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, nil, err
	}
	where := []string{"u.tenant_id = $1", "NOT u.is_blocked", "u.type = $2", "u.status = $3"}
	args := []any{tenant, string(user.TypeUser), string(user.StatusActive)}
	if len(r.groupIDs) > 0 {
		args = append(args, r.groupIDs)
		where = append(where, fmt.Sprintf("EXISTS (SELECT 1 FROM group_users gu WHERE gu.user_id = u.id AND gu.group_id = ANY($%d))", len(args)))
	}
	return where, args, nil
}

func queryNotificationRecipients(ctx context.Context, sql string, args []any) ([]notifications.Recipient, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	recipients := []notifications.Recipient{}
	for rows.Next() {
		var recipient notifications.Recipient
		var firstName, lastName, email string
		if err := rows.Scan(&recipient.ID, &firstName, &lastName, &email); err != nil {
			return nil, err
		}
		recipient.Name = strings.TrimSpace(firstName + " " + lastName)
		recipient.Hint = email
		recipients = append(recipients, recipient)
	}
	return recipients, rows.Err()
}
