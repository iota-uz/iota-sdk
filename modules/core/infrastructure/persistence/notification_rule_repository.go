package persistence

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5"
)

type NotificationRuleRepository struct{}

func NewNotificationRuleRepository() notifications.RuleRepository {
	return &NotificationRuleRepository{}
}
func (r *NotificationRuleRepository) Get(ctx context.Context, key string) (notifications.Rule, error) {
	rule := notifications.Rule{EventKey: key, UserIDs: []uint{}}
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return rule, serrors.Wrap("NotificationRuleRepository.Get", err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return rule, serrors.Wrap("NotificationRuleRepository.Get", err)
	}
	var ids, groups, roles, recipients []byte
	err = db.QueryRow(ctx, "SELECT enabled,user_ids,group_ids,role_ids,recipient_keys,level,created_at,updated_at FROM core.notification_rules WHERE tenant_id=$1 AND event_key=$2", tenant, key).Scan(&rule.Enabled, &ids, &groups, &roles, &recipients, &rule.Level, &rule.CreatedAt, &rule.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return rule, nil
	}
	if err != nil {
		return rule, serrors.Wrap("NotificationRuleRepository.Get", err)
	}
	rule.Configured = true
	if err = json.Unmarshal(recipients, &rule.RecipientKeys); err != nil {
		return rule, serrors.Wrap("NotificationRuleRepository.Get", err)
	}
	if err = json.Unmarshal(ids, &rule.UserIDs); err != nil {
		return rule, serrors.Wrap("NotificationRuleRepository.Get", err)
	}
	if err = json.Unmarshal(groups, &rule.GroupIDs); err != nil {
		return rule, serrors.Wrap("NotificationRuleRepository.Get", err)
	}
	if err = json.Unmarshal(roles, &rule.RoleIDs); err != nil {
		return rule, serrors.Wrap("NotificationRuleRepository.Get", err)
	}
	return rule, nil
}
func (r *NotificationRuleRepository) Save(ctx context.Context, rule notifications.Rule) error {
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap("NotificationRuleRepository.Save", err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap("NotificationRuleRepository.Save", err)
	}
	if rule.UserIDs == nil {
		rule.UserIDs = []uint{}
	}
	if rule.GroupIDs == nil {
		rule.GroupIDs = []uuid.UUID{}
	}
	if rule.RoleIDs == nil {
		rule.RoleIDs = []uint{}
	}
	if rule.RecipientKeys == nil {
		rule.RecipientKeys = []string{}
	}
	recipients, err := json.Marshal(rule.RecipientKeys)
	if err != nil {
		return serrors.Wrap("NotificationRuleRepository.Save", err)
	}
	groups, err := json.Marshal(rule.GroupIDs)
	if err != nil {
		return serrors.Wrap("NotificationRuleRepository.Save", err)
	}
	roles, err := json.Marshal(rule.RoleIDs)
	if err != nil {
		return serrors.Wrap("NotificationRuleRepository.Save", err)
	}
	ids, err := json.Marshal(rule.UserIDs)
	if err != nil {
		return serrors.Wrap("NotificationRuleRepository.Save", err)
	}
	_, err = db.Exec(ctx, `INSERT INTO core.notification_rules(tenant_id,event_key,enabled,user_ids,group_ids,role_ids,recipient_keys,level) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(tenant_id,event_key) DO UPDATE SET enabled=EXCLUDED.enabled,user_ids=EXCLUDED.user_ids,group_ids=EXCLUDED.group_ids,role_ids=EXCLUDED.role_ids,recipient_keys=EXCLUDED.recipient_keys,level=EXCLUDED.level,updated_at=NOW()`, tenant, rule.EventKey, rule.Enabled, ids, groups, roles, recipients, rule.Level)
	if err != nil {
		return serrors.Wrap("NotificationRuleRepository.Save", err)
	}
	return nil
}
