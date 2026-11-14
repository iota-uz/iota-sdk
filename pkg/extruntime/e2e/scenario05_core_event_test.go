package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

func (e *env) createUser(by *actor, first, last, email string) user.User {
	e.t.Helper()
	addr, err := internet.NewEmail(email)
	require.NoError(e.t, err)
	created, err := e.users.Create(e.userContext(by), user.New(first, last, addr, user.UILanguageEN, user.WithTenantID(e.tenant)))
	require.NoError(e.t, err)
	return created
}

// Scenario 5: the core event core.user.created.v1, published by the core user
// service in the creating transaction, drives a subscription of a package,
// which starts a workflow, which sends a notice to an external contact (no ERP
// user) resolved by the package's contact resolver and delivered by the fake
// SMS and email providers. A rolled back creation delivers nothing; a
// redelivered event starts no second workflow.
func TestScenario05CoreEventToExternalNotice(t *testing.T) {
	e := newEnv(t)
	admin := e.admin
	const ns = "e2e.welcome"
	e.deploy(admin, ns, "1.0.0", loadTree(t, "welcome"))
	// The contact resolver runs as an explicit owner when no user is involved.
	owner := e.newActor("resolver-owner", "Extensions.Read")
	e.assignOwner(admin, ns+".contactDetails", owner)
	e.assignOwner(admin, ns+".onUserCreated", owner)

	// ---- a committed creation
	created := e.createUser(admin, "Nina", "Newcomer", "nina@example.com")
	id := itoa(created.ID())
	phone := "+99890" + strings10(id)
	e.untilDelivered("welcome SMS and email reach the external contact", 60*time.Second, func() bool {
		return len(e.sms.SentTo(phone)) == 1 && len(e.email.SentTo("sponsor-"+id+"@partner.example")) == 1
	})
	require.Equal(t, "Welcome, Nina Newcomer!", e.sms.SentTo(phone)[0].Text)
	mail := e.email.SentTo("sponsor-" + id + "@partner.example")[0]
	require.Equal(t, "Welcome Nina Newcomer", mail.EmailSubject)
	require.Empty(t, e.email.SentTo("nina@example.com"), "the account's own address is not a recipient")
	require.Empty(t, e.sms.Sent()[1:], "exactly one SMS")

	list := e.instancesOf(admin, ns+".onboard")
	require.Len(t, list, 1)
	inst := e.waitStatus(admin, list[0].ID, "completed")
	require.Equal(t, ns, inst.Package)
	require.NotNil(t, inst.StartKey, "started by the subscription with a derived key")
	require.Contains(t, *inst.StartKey, ns+".onUserCreated")
	require.Equal(t, 1, e.scalarInt(`SELECT count(*) FROM extruntime.subscription_receipts WHERE subscription = $1 AND status = 'done'`, ns+".onUserCreated"))
	require.Equal(t, 1, e.scalarInt(`SELECT count(*) FROM x_e2e_welcome.contact WHERE tenant_id = $1`, e.tenant))

	// ---- a rolled back creation delivers nothing
	smsBefore, mailBefore := len(e.sms.Sent()), len(e.email.Sent())
	outboxBefore := e.scalarInt(`SELECT count(*) FROM extruntime.outbox_events WHERE event = 'core.user.created.v1'`)
	tx, err := e.pool.Begin(context.Background())
	require.NoError(t, err)
	addr, err := internet.NewEmail("ghost@example.com")
	require.NoError(t, err)
	txCtx := composables.WithTx(e.userContext(admin), tx)
	_, err = e.users.Create(txCtx, user.New("Gus", "Ghost", addr, user.UILanguageEN, user.WithTenantID(e.tenant)))
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(context.Background()))
	// Give the dispatcher, the engine and the notification passes time to (not) act.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		e.pumpNotifications()
		time.Sleep(200 * time.Millisecond)
	}
	require.Equal(t, outboxBefore, e.scalarInt(`SELECT count(*) FROM extruntime.outbox_events WHERE event = 'core.user.created.v1'`), "the event rolled back with the user")
	require.Len(t, e.instancesOf(admin, ns+".onboard"), 1)
	require.Len(t, e.sms.Sent(), smsBefore)
	require.Len(t, e.email.Sent(), mailBefore)
	require.Zero(t, e.scalarInt(`SELECT count(*) FROM users WHERE email = 'ghost@example.com'`))

	// ---- redelivery of an event already handled starts no second workflow
	e.rt.Dispatcher.Stop()
	_, err = e.pool.Exec(context.Background(), `DELETE FROM extruntime.subscription_receipts WHERE subscription = $1`, ns+".onUserCreated")
	require.NoError(t, err)
	delivered, err := e.rt.Dispatcher.RunOnce(context.Background())
	require.NoError(t, err)
	require.Positive(t, delivered, "the handler ran again for the redelivered event")
	require.Equal(t, 1, e.scalarInt(`SELECT count(*) FROM extruntime.subscription_receipts WHERE subscription = $1 AND status = 'done'`, ns+".onUserCreated"))
	require.Len(t, e.instancesOf(admin, ns+".onboard"), 1, "start_key = subscription:event dedupes the start")
	e.pumpNotifications()
	require.Len(t, e.sms.SentTo(phone), 1, "no second welcome")
}

// strings10 left-pads an id like the package does (7 digits).
func strings10(id string) string {
	for len(id) < 7 {
		id = "0" + id
	}
	return id
}
