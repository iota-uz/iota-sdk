package e2e_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/services"
)

type outcomeRow struct {
	Kind, Channel, Target, Status, Code string
	Inbox                               bool
}

// outcomes lists every delivery row of the tenant as a comparable value keyed by
// "recipient/channel/target": the per-target outcome the epic promises.
func (e *env) outcomes() map[string]outcomeRow {
	e.t.Helper()
	rows, err := e.pool.Query(context.Background(), `SELECT recipient_kind, channel, target_address, status, error_code,
		inbox_notification_id IS NOT NULL, coalesce(recipient_key, '') FROM core.notification_deliveries WHERE tenant_id = $1`, e.tenant)
	require.NoError(e.t, err)
	defer rows.Close()
	out := map[string]outcomeRow{}
	for rows.Next() {
		var r outcomeRow
		var key string
		require.NoError(e.t, rows.Scan(&r.Kind, &r.Channel, &r.Target, &r.Status, &r.Code, &r.Inbox, &key))
		out[fmt.Sprintf("%s/%s/%s", key, r.Channel, r.Target)] = r
	}
	require.NoError(e.t, rows.Err())
	return out
}

func (e *env) addCustomer(name string, emailConsent bool) map[string]string {
	e.t.Helper()
	var out struct {
		Customer map[string]string `json:"customer"`
	}
	e.mustRPC(e.admin, "atomic", "e2e.alerts.addCustomer", map[string]any{"name": name, "emailConsent": emailConsent}, &out)
	return out.Customer
}

// Scenario 14: the whole channel stack on fake providers. A package notice
// reaches (1) an external customer record on SMS, email and two push devices
// resolved by the package's contact resolver, one device failing and the other
// delivered; (2) an ERP user with an inbox row and push to the user's registered
// devices, one of which the provider reports gone; and (3) a package delivery
// policy denies exactly one target (the email of a customer who withdrew
// consent) without touching its siblings. Every outcome is a row of its own.
func TestScenario14NoticeChannelsAndPackageDeliveryPolicy(t *testing.T) {
	e := newEnv(t)
	admin := e.admin
	const ns = "e2e.alerts"
	e.deploy(admin, ns, "1.0.0", loadTree(t, "alerts"))
	owner := e.newActor("alerts-owner", "Extensions.Read")
	e.assignOwner(admin, ns+".customerContacts", owner)
	e.assignOwner(admin, ns+".customerConsent", owner)

	una := e.createUser(admin, "Una", "User", "una@example.com")
	unaID := itoa(una.ID())
	channels := resolve[*services.NotificationChannelService](t, e.scope.Container)
	for _, device := range []string{"phone", "tablet"} {
		require.NoError(t, channels.RegisterDevice(e.ctx, notifications.PushDevice{UserID: una.ID(), DeviceID: "una-" + device, Token: "tok-una-" + device, Platform: notifications.PushWeb}))
	}
	// the customer's tablet is rejected by the provider; the user's tablet registration expired
	e.push.FailAddress("ada-tablet", fmt.Errorf("%w: rejected", notifications.ErrPermanent))
	e.push.FailAddress("tok-una-tablet", fmt.Errorf("%w: %w: expired", notifications.ErrPermanent, notifications.ErrTargetGone))

	ada, bob := e.addCustomer("ada", true), e.addCustomer("bob", false)
	send := func(customer map[string]string, amount string) {
		e.mustRPC(admin, "atomic", "e2e.alerts.sendStatement", map[string]any{
			"customer": customer, "user": map[string]string{"entity": "core.user", "key": unaID}, "amount": amount,
		}, nil)
	}
	send(ada, "100.00")
	send(bob, "200.00")

	e.untilDelivered("every target of both statements has an outcome", 60*time.Second, func() bool {
		rows := e.outcomes()
		for _, r := range rows {
			if r.Status == "unresolved" || r.Status == "pending" {
				return false
			}
		}
		return e.scalarInt(`SELECT count(*) FROM core.notification_deliveries WHERE tenant_id = $1`, e.tenant) == 14
	})

	// ---- external recipient: SMS + email + two push devices, nothing sent in the producer's transaction
	adaKey, bobKey := "record:e2e.alerts.customer/"+ada["key"], "record:e2e.alerts.customer/"+bob["key"]
	rows := e.outcomes()
	want := func(key string, status, code string) {
		t.Helper()
		got, ok := rows[key]
		require.True(t, ok, "no delivery row %s in %v", key, rows)
		require.Equal(t, status, got.Status, key)
		require.Equal(t, code, got.Code, key)
	}
	want(adaKey+"/sms/+998-ada", "sent", "")
	want(adaKey+"/email/ada@customer.example", "sent", "")
	want(adaKey+"/push/ada-phone", "sent", "")
	want(adaKey+"/push/ada-tablet", "failed", "provider_rejected")
	require.Len(t, e.sms.SentTo("+998-ada"), 1)
	require.Equal(t, "Statement: 100.00", e.sms.SentTo("+998-ada")[0].Text)
	require.Len(t, e.email.SentTo("ada@customer.example"), 1)
	require.Len(t, e.push.SentTo("ada-phone"), 1)
	require.Equal(t, "100.00", e.push.SentTo("ada-phone")[0].Data["amount"])
	require.Empty(t, e.push.SentTo("ada-tablet"), "the failing device did not stop its sibling, and was not delivered")
	require.NotEmpty(t, e.push.Calls())
	require.False(t, e.push.CalledInTransaction() || e.sms.CalledInTransaction() || e.email.CalledInTransaction())

	// ---- the package policy: Bob withdrew email consent; only that target is skipped, with a reason
	want(bobKey+"/email/bob@customer.example", "skipped", "policy_denied:email_opt_out")
	want(bobKey+"/sms/+998-bob", "sent", "")
	want(bobKey+"/push/bob-phone", "sent", "")
	want(bobKey+"/push/bob-tablet", "sent", "")
	require.Empty(t, e.email.SentTo("bob@customer.example"), "a denied target never reaches the provider")
	require.Equal(t, 0, e.scalarInt(`SELECT count(*) FROM core.notification_delivery_attempts a JOIN core.notification_deliveries d ON d.id = a.delivery_id
		WHERE d.tenant_id = $1 AND d.status = 'skipped'`, e.tenant))

	// ---- ERP user: inbox row + push to the registered devices, nothing for the user's email/sms
	userKey := "user:" + unaID
	require.True(t, rows[userKey+"/in_app/"].Inbox, "the in-app delivery links the inbox row")
	want(userKey+"/in_app/", "sent", "")
	want(userKey+"/push/tok-una-phone", "sent", "")
	want(userKey+"/push/tok-una-tablet", "failed", "provider_rejected")
	require.Equal(t, 2, e.scalarInt(`SELECT count(*) FROM core.notifications WHERE tenant_id = $1 AND user_id = $2 AND title = 'Statement'`, e.tenant, una.ID()))
	require.Len(t, e.push.SentTo("tok-una-phone"), 2)
	require.Equal(t, "Statement", e.push.SentTo("tok-una-phone")[0].Title)

	// ---- a device the provider reports gone is revoked, so the next notice does not try it again
	require.Equal(t, 1, e.scalarInt(`SELECT count(*) FROM core.notification_push_devices WHERE tenant_id = $1 AND user_id = $2 AND device_id = 'una-phone' AND revoked_at IS NULL`, e.tenant, una.ID()))
	require.Equal(t, 1, e.scalarInt(`SELECT count(*) FROM core.notification_push_devices WHERE tenant_id = $1 AND user_id = $2 AND device_id = 'una-tablet' AND revoked_at IS NOT NULL`, e.tenant, una.ID()))
	callsBefore := len(e.push.Calls())
	send(ada, "300.00")
	e.untilDelivered("the third statement is pushed", 60*time.Second, func() bool { return len(e.push.SentTo("tok-una-phone")) == 3 })
	for _, m := range e.push.Calls()[callsBefore:] {
		require.NotEqual(t, "tok-una-tablet", m.Address, "the revoked device is not tried again")
	}

	// ---- unbinding the policy owner makes deliveries wait instead of skipping the consent check
	_, err := e.pool.Exec(context.Background(), `DELETE FROM extruntime.trigger_owners WHERE trigger_id = $1`, ns+".customerConsent")
	require.NoError(t, err)
	smsBefore := len(e.sms.Sent())
	send(bob, "400.00")
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		e.pumpNotifications()
		time.Sleep(200 * time.Millisecond)
	}
	require.Len(t, e.sms.Sent(), smsBefore+0, "without an owner the policy cannot run: record deliveries wait (fail closed)")
	require.NotZero(t, e.scalarInt(`SELECT count(*) FROM core.notification_deliveries WHERE tenant_id = $1 AND recipient_kind = 'record' AND status = 'unresolved'`, e.tenant))
}
