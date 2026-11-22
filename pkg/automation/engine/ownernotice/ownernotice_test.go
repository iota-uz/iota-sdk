package ownernotice_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine/ownernotice"
)

func TestTheOwnerOfAFailedRunIsTheRecipient(t *testing.T) {
	def := ownernotice.Definition()
	cat := notifications.NewCatalog()
	require.NoError(t, cat.Register(def), "the core catalog accepts the definition")
	got, ok := cat.Get(engine.EventRunFailed)
	require.True(t, ok, "the outbox bridge finds the definition by the event key")
	payload, err := json.Marshal(engine.RunFailedPayload{RunID: "r-1", AutomationID: "a-1", AutomationName: "Orders", Version: 3, OwnerUserID: 42, Code: "timeout"})
	require.NoError(t, err)
	data, err := got.DataOf(payload)
	require.NoError(t, err)
	ev := notifications.Event{Key: engine.EventRunFailed, Data: data, Payload: payload}
	assert.Equal(t, []string{ownernotice.RecipientOwner}, got.DefaultRecipientKeys, "delivered to the owner without a tenant rule")
	var users []uint
	for _, r := range got.RecipientKeys {
		if r.Key == ownernotice.RecipientOwner {
			users, err = r.Resolve(context.Background(), ev)
			require.NoError(t, err)
		}
	}
	assert.Equal(t, []uint{42}, users)
	assert.Equal(t, "automation.run.failed:r-1", got.DedupeKey(ev))
	content, err := got.Render(ev, "en")
	require.NoError(t, err)
	assert.Equal(t, "Automation run failed", content.Title)
	assert.Contains(t, content.Body, "Orders (v3): timeout")
	assert.Equal(t, "/automations/a-1/runs/r-1", content.ActionURL)
}
