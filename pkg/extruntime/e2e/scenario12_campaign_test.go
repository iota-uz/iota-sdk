package e2e_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type target struct {
	Number  string  `json:"number"`
	Outcome string  `json:"outcome"`
	Reply   *string `json:"reply"`
}

func (e *env) targets(a *actor, campaign uuid.UUID) []target {
	e.t.Helper()
	var out []target
	e.mustRPC(a, "query", "e2e.renewals.targets", map[string]any{"campaign": campaign.String()}, &out)
	return out
}

func (e *env) inbound(from, text string) resp {
	body, _ := json.Marshal(map[string]string{"from": from, "text": text})
	return e.do(nil, http.MethodPost, "/_ext/endpoints/e2e.renewals/sms/inbound", strings.NewReader(string(body)), "Content-Type", "application/json")
}

// Scenario 12: an SMS renewal campaign written entirely in TypeScript. The
// workflow selects the policies expiring in a window (its own entity), sends
// one SMS per policy through the shared notification system (idempotent per
// policy), waits for replies — which a gateway posts to a public endpoint —
// or the end of the reply window, and finishes with a per-target outcome. A
// restart in the middle neither loses nor duplicates anything.
func TestScenario12SMSRenewalCampaignPilot(t *testing.T) {
	e := newEnv(t)
	admin := e.admin
	const ns = "e2e.renewals"
	e.deploy(admin, ns, "1.0.0", loadTree(t, "renewals"))

	// Synthetic fixtures through the package's own command.
	day := func(n int) string { return time.Now().UTC().AddDate(0, 0, n).Format("2006-01-02") }
	phones := map[string]string{"P-05": "+998901000005", "P-01": "+998901000001", "P-02": "+998901000002"}
	for _, p := range []struct {
		number, phone string
		in            int
		lapsed        bool
	}{
		{"P-01", phones["P-01"], 5, false}, {"P-02", phones["P-02"], 10, false}, {"P-05", phones["P-05"], 3, false},
		{"P-03", "+998901000003", 40, false}, // outside the window
		{"P-04", "+998901000004", 7, true},   // lapsed
		{"P-00", "+998901000000", -2, false}, // already expired
	} {
		e.mustRPC(admin, "command", "e2e.renewals.addPolicy", map[string]any{
			"number": p.number, "holder": "Holder " + p.number, "phone": p.phone, "expiresOn": day(p.in), "lapsed": p.lapsed,
		}, nil)
	}
	// Policy numbers are unique: the duplicate is refused and nothing is written.
	// (The host reports it as an internal error today; see the report.)
	dup := e.rpc(admin, "command", "e2e.renewals.addPolicy", map[string]any{"number": "P-01", "holder": "dup", "phone": "+1", "expiresOn": day(1)})
	require.False(t, dup.OK)
	require.Equal(t, 6, e.scalarInt(`SELECT count(*) FROM x_e2e_renewals.policy WHERE tenant_id = $1`, e.tenant))

	// A public endpoint runs as its trigger owner; without one it refuses.
	require.Equal(t, http.StatusServiceUnavailable, e.inbound(phones["P-01"], "YES").Status)
	gateway := e.newActor("sms-gateway-owner")
	e.assignOwner(admin, ns+".smsInbound", gateway)

	// Start the campaign: window [today, today+14d], replies accepted for 12 seconds.
	var started struct {
		Instance string `json:"instance"`
	}
	in := map[string]any{"from": day(0), "to": day(14), "replyWindowMs": 12000}
	e.mustRPC(admin, "command", "e2e.renewals.startCampaign", in, &started)
	id := uuid.MustParse(started.Instance)
	var again struct {
		Instance string `json:"instance"`
	}
	e.mustRPC(admin, "command", "e2e.renewals.startCampaign", in, &again)
	require.Equal(t, started.Instance, again.Instance, "the same campaign window starts one instance")

	// Three SMS go out, in expiry order, to exactly the policies in the window.
	e.untilDelivered("three reminders are delivered", 60*time.Second, func() bool { return len(e.sms.Sent()) == 3 })
	for _, number := range []string{"P-05", "P-01", "P-02"} {
		msgs := e.sms.SentTo(phones[number])
		require.Len(t, msgs, 1, number)
		require.Contains(t, msgs[0].Text, "policy "+number+" expires on")
	}
	e.eventually("the campaign waits for replies", 30*time.Second, func() bool {
		for _, o := range e.workflow(admin, id).Occurrences {
			if o.Name == "reply" && o.Status == "waiting" {
				return true
			}
		}
		return false
	})
	got := e.targets(admin, id)
	require.Equal(t, []target{{"P-01", "sent", nil}, {"P-02", "sent", nil}, {"P-05", "sent", nil}}, got)

	// The engine restarts in the middle of the wait: nothing is sent twice.
	e.restart()
	e.pumpNotifications()
	require.Len(t, e.sms.Sent(), 3)

	// A reply arrives through the gateway's webhook (and is idempotent per sender).
	r := e.inbound(phones["P-01"], "YES please")
	require.Equal(t, http.StatusAccepted, r.Status, r.String()+" "+e.scalarString(`SELECT error::text FROM extruntime.invocations WHERE contract = $1 ORDER BY started_at DESC LIMIT 1`, ns+".smsInbound"))
	require.JSONEq(t, `{"accepted":true}`, r.String())
	r = e.inbound("+998909999999", "who is this")
	require.Equal(t, http.StatusAccepted, r.Status)
	require.JSONEq(t, `{"accepted":false}`, r.String(), "a number the campaign did not text is ignored")
	e.eventually("the reply is recorded", 30*time.Second, func() bool {
		for _, tg := range e.targets(admin, id) {
			if tg.Number == "P-01" && tg.Outcome == "replied" {
				return true
			}
		}
		return false
	})

	// The window closes: the others are closed as "noReply", and the instance finishes.
	done := e.waitStatus(admin, id, "completed")
	var out struct {
		Outcomes []struct{ Number, Outcome string } `json:"outcomes"`
	}
	require.NoError(t, json.Unmarshal(done.Result, &out), string(done.Result))
	require.Equal(t, []struct{ Number, Outcome string }{{"P-01", "replied"}, {"P-02", "noReply"}, {"P-05", "noReply"}}, out.Outcomes)
	final := e.targets(admin, id)
	require.Equal(t, "replied", final[0].Outcome)
	require.NotNil(t, final[0].Reply)
	require.Equal(t, "YES please", *final[0].Reply)
	require.Equal(t, "noReply", final[1].Outcome)
	require.Equal(t, "noReply", final[2].Outcome)

	// No duplicates, no strays: three sends in total, to the three numbers, once each.
	e.pumpNotifications()
	require.Len(t, e.sms.Sent(), 3)
	for _, number := range []string{"P-05", "P-01", "P-02"} {
		require.Len(t, e.sms.SentTo(phones[number]), 1, number)
	}
	for _, other := range []string{"+998901000003", "+998901000004", "+998901000000"} {
		require.Empty(t, e.sms.SentTo(other), other)
	}
	require.Equal(t, 3, e.scalarInt(`SELECT count(*) FROM x_e2e_renewals.target WHERE tenant_id = $1`, e.tenant), "one target per policy, none recreated by the restart")
	require.Equal(t, 3, e.scalarInt(`SELECT count(*) FROM extruntime.workflow_occurrences WHERE instance_id = $1 AND name LIKE 'send:%' AND status = 'completed'`, id))
}
