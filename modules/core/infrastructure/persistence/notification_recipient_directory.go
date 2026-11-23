package persistence

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/filterq"
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
	conditions, args := filterq.AppendSQL(query.Filters, notificationRecipientFilterSchema, notificationRecipientFilterColumns, time.Now(), args)
	args = append(args, query.Limit, query.Offset)
	sql := fmt.Sprintf("%s WHERE %s%s ORDER BY u.first_name, u.last_name, u.id LIMIT $%d OFFSET $%d", notificationRecipientColumns, strings.Join(where, " AND "), conditions, len(args)-1, len(args))
	recipients, err := queryNotificationRecipients(ctx, sql, args)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return recipients, nil
}

// The recipient filters are memberships, so each is an EXISTS over the user.
// IDs compare as text: a hand-edited value matches nobody instead of failing
// the query.
var notificationRecipientFilterSchema = filterq.Schema{Fields: []filterq.Field{
	{Key: notifications.RecipientFilterRole, Type: filterq.FieldTypeReference},
	{Key: notifications.RecipientFilterGroup, Type: filterq.FieldTypeReference},
	{Key: notifications.RecipientFilterOptedOut, Type: filterq.FieldTypeReference},
}}

var notificationRecipientFilterColumns = map[string]filterq.Column{
	// A role counts when held directly or through a group, as when a rule
	// names the role (NotificationAudienceRepository.Resolve).
	notifications.RecipientFilterRole: {Custom: notificationRecipientMembership(func(list string) string {
		return "SELECT 1 FROM user_roles ur JOIN roles r ON r.id = ur.role_id WHERE ur.user_id = u.id AND r.tenant_id = u.tenant_id AND r.id::text = ANY(" + list + ")" +
			" UNION ALL SELECT 1 FROM group_users gu JOIN user_groups g ON g.id = gu.group_id JOIN group_roles gr ON gr.group_id = g.id JOIN roles r ON r.id = gr.role_id" +
			" WHERE gu.user_id = u.id AND g.tenant_id = u.tenant_id AND r.tenant_id = u.tenant_id AND r.id::text = ANY(" + list + ")"
	})},
	notifications.RecipientFilterGroup: {Custom: notificationRecipientMembership(func(list string) string {
		return "SELECT 1 FROM group_users gu JOIN user_groups g ON g.id = gu.group_id WHERE gu.user_id = u.id AND g.tenant_id = u.tenant_id AND g.id::text = ANY(" + list + ")"
	})},
	notifications.RecipientFilterOptedOut: {Custom: notificationRecipientMembership(func(list string) string {
		return "SELECT 1 FROM core.notification_channel_preferences p WHERE p.tenant_id = u.tenant_id AND p.user_id = u.id AND p.channel = ANY(" + list + ")"
	})},
}

// notificationRecipientMembership renders "is" as EXISTS and "is not" as NOT
// EXISTS of the subquery built for the condition's values.
func notificationRecipientMembership(subquery func(list string) string) func(filterq.Condition, *filterq.Params) (string, bool) {
	return func(c filterq.Condition, p *filterq.Params) (string, bool) {
		if len(c.Values) == 0 {
			return "", false
		}
		switch c.Op {
		case filterq.OpIs:
			return "EXISTS (" + subquery(p.Add(c.Values)+"::text[]") + ")", true
		case filterq.OpIsNot:
			return "NOT EXISTS (" + subquery(p.Add(c.Values)+"::text[]") + ")", true
		case filterq.OpBefore, filterq.OpAfter, filterq.OpOn, filterq.OpBetween, filterq.OpEq, filterq.OpGt, filterq.OpLt, filterq.OpEmpty:
			// Not membership operators.
		}
		return "", false
	}
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
