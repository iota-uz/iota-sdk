package automations_test

import (
	"context"
	"embed"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/actions"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// The built-in node types are in the module's registry and the SDK's actions join it at start.
func TestBuiltinNodesAndSDKActionsAreInTheCatalog(t *testing.T) {
	f := itf.Setup(t, itf.WithComponents(modules.Components()...), itf.WithUser(itf.User()))
	reg, err := composition.Resolve[catalog.Registry](f.Container)
	require.NoError(t, err)
	for _, typ := range []string{"set", "code", "http", "start_automation", "run_automation",
		"action.core.user.block", "action.core.user.unblock", "action.core.group.add_user", "action.core.group.remove_user",
		"action.crm.chat.send_message"} {
		_, ok := reg.Node(typ)
		assert.True(t, ok, "%s is registered", typ)
	}
	// An author without the action's permission does not see it.
	v, err := reg.ForTenant(context.Background(), uuid.New(), catalog.AsAuthor(spi.Subject{Permissions: []string{"Group.Update"}}))
	require.NoError(t, err)
	assert.Equal(t, catalog.ReasonPermission, v.Unavailable["action.core.user.block"])
	_, ok := v.Catalog.Nodes["action.core.group.add_user"]
	assert.True(t, ok)
}

type dupAction struct{}

func (dupAction) Key() string { return "core.user.block" }
func (dupAction) Spec() actions.ActionSpec {
	return actions.ActionSpec{Label: manifest.Label{"en": "Dup"}, Permission: "User.UpdateBlockStatus"}
}
func (dupAction) Run(context.Context, spi.Subject, json.RawMessage) (string, json.RawMessage, error) {
	return "done", nil, nil
}

type dupComponent struct{}

func (dupComponent) Descriptor() composition.Descriptor { return composition.Descriptor{Name: "dup_actions"} }
func (dupComponent) LocaleFS() []*embed.FS { return nil }
func (dupComponent) Build(b *composition.Builder) error {
	actions.Contribute(b, dupAction{})
	return nil
}

// A key contributed twice fails the start of the application.
func TestDuplicateActionKeyFailsTheStart(t *testing.T) {
	db := itf.NewHarness(t, itf.HarnessConfig{Name: "automations_actions"}).Scope(t)
	_, _, err := itf.SetupApplication(db.Pool, append(modules.Components(), dupComponent{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "core.user.block")
}
