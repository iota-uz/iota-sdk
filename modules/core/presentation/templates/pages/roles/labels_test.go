package roles_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/roles"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/assert"
)

type labelContext struct {
	types.PageContext
	messages map[string]string
}

func (c labelContext) TSafe(key string, _ ...map[string]interface{}) string { return c.messages[key] }

func TestModuleLabel(t *testing.T) {
	t.Parallel()

	ctx := labelContext{messages: map[string]string{"Modules.Core": "Администрирование"}}
	assert.Equal(t, "Администрирование", roles.ModuleLabel(ctx, "Core"))
	assert.Equal(t, "Администрирование", roles.ModuleLabel(ctx, "core"))
	assert.Equal(t, "Logistics", roles.ModuleLabel(ctx, "logistics"))
	assert.Empty(t, roles.ModuleLabel(ctx, ""))
}

func TestResourceLabel(t *testing.T) {
	t.Parallel()

	ctx := labelContext{messages: map[string]string{"Resources.user": "Пользователи"}}
	assert.Equal(t, "Пользователи", roles.ResourceLabel(ctx, "user"))
	assert.Equal(t, "Notification rules", roles.ResourceLabel(ctx, "notification_rules"))
}
