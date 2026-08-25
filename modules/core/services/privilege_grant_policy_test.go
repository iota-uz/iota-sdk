package services_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/stretchr/testify/assert"
)

func TestPrivilegeGrantPolicy_DominatesModifiers(t *testing.T) {
	t.Parallel()

	all := permission.New(
		permission.WithID(uuid.New()),
		permission.WithName("Test.Read.All"),
		permission.WithResource("Test"),
		permission.WithAction(permission.ActionRead),
		permission.WithModifier(permission.ModifierAll),
	)
	own := permission.New(
		permission.WithID(uuid.New()),
		permission.WithName("Test.Read.Own"),
		permission.WithResource("Test"),
		permission.WithAction(permission.ActionRead),
		permission.WithModifier(permission.ModifierOwn),
	)
	updateOwn := permission.New(
		permission.WithID(uuid.New()),
		permission.WithName("Test.Update.Own"),
		permission.WithResource("Test"),
		permission.WithAction(permission.ActionUpdate),
		permission.WithModifier(permission.ModifierOwn),
	)

	// Falsely green if dominance compares only resource or only permission IDs.
	assert.True(t, services.Dominates([]permission.Permission{all}, []permission.Permission{own}))
	assert.False(t, services.Dominates([]permission.Permission{own}, []permission.Permission{all}))
	assert.False(t, services.Dominates([]permission.Permission{all}, []permission.Permission{updateOwn}))
}
