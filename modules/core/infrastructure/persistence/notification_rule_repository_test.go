package persistence_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/stretchr/testify/require"
)

func TestNotificationRuleRepository_SemanticLevelAuditAndTenant(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewNotificationRuleRepository()
	before, err := repo.Get(f.Ctx, notifications.TestEventKey)
	require.NoError(t, err)
	require.False(t, before.Configured)
	rule := notifications.Rule{EventKey: notifications.TestEventKey, Enabled: true, RecipientKeys: []string{"actor"}, Level: notification.LevelWarning}
	require.NoError(t, repo.Save(f.Ctx, rule))
	saved, err := repo.Get(f.Ctx, rule.EventKey)
	require.NoError(t, err)
	require.True(t, saved.Configured)
	require.Equal(t, rule.Level, saved.Level)
	require.Equal(t, rule.RecipientKeys, saved.RecipientKeys)
	require.False(t, saved.CreatedAt.IsZero())
	require.False(t, saved.UpdatedAt.IsZero())
	missing, err := repo.Get(composables.WithTenantID(f.Ctx, uuid.New()), rule.EventKey)
	require.NoError(t, err)
	require.False(t, missing.Configured)
	rule.Enabled = false
	rule.Level = ""
	require.NoError(t, repo.Save(f.Ctx, rule))
	updated, err := repo.Get(f.Ctx, rule.EventKey)
	require.NoError(t, err)
	require.False(t, updated.Enabled)
	require.Equal(t, saved.CreatedAt, updated.CreatedAt)
	require.Empty(t, updated.Level)
}
