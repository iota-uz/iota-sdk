package engine_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// The phone of core.user.created.v1 is personal data (#1468): a run that read it shows the phone, and
// everything derived from it, only to a viewer who may read users.
func TestRunHistoryHidesThePhoneOfACreatedUser(t *testing.T) {
	const userRead = "User.Read"
	src := catalog.FromNativeEvents(nativeEvents{coreservices.UserCreatedEvent})
	access, ok := src.(catalog.PermissionedTriggers).Access(coreservices.UserCreatedEventID)
	require.True(t, ok)
	require.Equal(t, []string{userRead}, access.Sensitive["phone"], "the phone is a sensitive field, removed for an author who may not read users")

	e := newEnv(t)
	require.NoError(t, e.reg.RegisterTriggers(src))
	e.subjects.Grant(1, userRead)
	p := e.proc()
	owner := e.actor(e.tenant, 1, engine.OriginHuman, append(append([]string{}, allPerms...), userRead)...)
	e.create(p, owner, `{"name": "New user", "triggers": [{"id": "created", "event": "core.user.created.v1"}],
	 "nodes": [{"id": "note", "type": "acme.note", "params": {"text": "call {{ event.phone }}"}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "created", "outcome": "fired"}, "to": "note"},
	  {"from": {"node": "note", "outcome": "done"}, "to": "fin"}, {"from": {"node": "note", "outcome": "failed"}, "to": "fin"}]}`)

	phone := "+998901234567"
	ctx := context.Background()
	tx, err := e.db.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = events.Publish(ctx, tx, spi.Scope{Subject: spi.Subject{TenantID: e.tenant}}, coreservices.UserCreatedEvent,
		&spi.ObjectRef{Entity: "core.user", Key: "7"},
		coreservices.UserCreatedPayload{UserID: 7, Email: "new@example.com", FirstName: "New", LastName: "User", UserType: "user", Phone: &phone})
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	run := onlyRun(t, e, p, owner)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends")

	mine := e.run(p, owner, run.RunID)
	var b strings.Builder
	for _, st := range mine.Steps {
		assert.False(t, st.Hidden)
		b.Write(st.Output)
	}
	assert.Contains(t, b.String(), phone, "a viewer who may read users reads the phone")
	var ev map[string]any
	require.NoError(t, json.Unmarshal(mine.Event, &ev))
	assert.Equal(t, phone, ev["phone"])

	// A viewer of automations without the read permission of users sees neither the phone nor what the
	// run derived from it.
	reader := e.actor(e.tenant, 2, engine.OriginHuman, engine.PermRead)
	theirs := e.run(p, reader, run.RunID)
	require.Len(t, theirs.Steps, len(mine.Steps))
	for _, st := range theirs.Steps {
		assert.True(t, st.Hidden, "step %s ran after the run read the phone", st.NodeID)
		assert.Nil(t, st.Input)
		assert.Nil(t, st.Output)
	}
	assert.NotContains(t, string(theirs.Event), phone)
}
