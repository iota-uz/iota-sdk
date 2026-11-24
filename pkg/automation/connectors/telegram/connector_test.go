package telegram_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

type avail map[catalog.ConnectorKind][]catalog.ConnectionRef

func (a avail) Connected(context.Context, uuid.UUID) (map[catalog.ConnectorKind][]catalog.ConnectionRef, error) {
	return a, nil
}

// The connector registers: its nodes, events and descriptor are valid, and a graph with them validates.
func TestRegistersAndValidatesGraphs(t *testing.T) {
	r := newRig(t)
	reg := catalog.NewRegistry(avail{telegram.Kind: {{ID: r.conn.ID.String(), Name: "bot"}}})
	dir := connectors.NewDirectory(reg)
	require.NoError(t, dir.Register(r.c))
	v, err := reg.ForTenant(context.Background(), r.tenant)
	require.NoError(t, err)
	require.Contains(t, v.Catalog.Nodes, telegram.NodeSend)
	require.Contains(t, v.Catalog.Nodes, telegram.NodeAsk)
	require.Contains(t, v.Catalog.Nodes, telegram.NodeRead)
	require.Contains(t, v.Catalog.Events, telegram.EventMessage)
	assert.True(t, v.Catalog.Events[telegram.EventCallback].NoTrigger, "a press starts no graph")
	require.NoError(t, v.Catalog.Check(nil))

	const g = `{"name":"tg","triggers":[{"id":"msg","event":"telegram.message_received.v1","params":{"commands":"include","command_names":["Help"],"private_only":true}}],
	"nodes":[
	 {"id":"ask","type":"telegram.ask","params":{"chat_id":"event.chatId","text":"Order {{ event.text }}?",
	   "options":[{"outcome":"yes","label":"Yes {{ event.from.firstName }}"},{"outcome":"no","label":"No"}],"columns":2,"timeout":"2h"}},
	 {"id":"read","type":"telegram.read","params":{"chat_id":"event.chatId","from_user_id":"nodes.ask.output.from.user_id","timeout":"1h"}},
	 {"id":"send","type":"telegram.send","params":{"chat_id":"event.chatId","thread_id":"event.threadId","text":"Got {{ nodes.read.output.event.text }}",
	   "url_buttons":[{"text":"Open {{ event.chatId }}","url":"https://erp.example.com/x/{{ event.chatId }}"}]}}
	],
	"edges":[{"from":{"node":"msg","outcome":"fired"},"to":"ask"},
	 {"from":{"node":"ask","outcome":"yes"},"to":"read"},{"from":{"node":"ask","outcome":"no"},"to":"send"},
	 {"from":{"node":"ask","outcome":"timeout"},"to":"ignore"},{"from":{"node":"ask","outcome":"failed"},"to":"ignore"},
	 {"from":{"node":"read","outcome":"received"},"to":"send"},{"from":{"node":"read","outcome":"timeout"},"to":"ignore"},
	 {"from":{"node":"send","outcome":"sent"},"to":"ignore"},{"from":{"node":"send","outcome":"failed"},"to":"ignore"}]}`
	parsed, errs := graph.Decode([]byte(g))
	require.Empty(t, errs)
	assert.Empty(t, graph.Validate(v.Catalog, parsed, graph.Options{Rules: catalog.ConnectionRule(v)}))
}

// The trigger runs on plain messages unless it takes commands; the author's options are matched.
func TestTriggerParamsMatching(t *testing.T) {
	r := newRig(t)
	reg := catalog.NewRegistry(avail{telegram.Kind: {{ID: r.conn.ID.String()}}})
	require.NoError(t, connectors.NewDirectory(reg).Register(r.c))
	v, err := reg.ForTenant(context.Background(), r.tenant)
	require.NoError(t, err)
	spec := v.Catalog.Events[telegram.EventMessage]
	msg := func(command string, extra map[string]any) map[string]any {
		e := map[string]any{"isCommand": command != "", "command": command, "route": "text", "chatType": "private", "chatId": "1"}
		if command != "" {
			e["route"] = "command:" + command
		}
		for k, v := range extra {
			e[k] = v
		}
		return e
	}
	for name, tc := range map[string]struct {
		params map[string]any
		event  map[string]any
		want   bool
	}{
		"plain message, default ignore": {nil, msg("", nil), true},
		"command, default ignore":       {nil, msg("start", nil), false},
		"command, only":                 {map[string]any{"commands": "only"}, msg("start", nil), true},
		"plain message, only":           {map[string]any{"commands": "only"}, msg("", nil), false},
		"include takes both":            {map[string]any{"commands": "include"}, msg("", nil), true},
		"include takes a command":       {map[string]any{"commands": "include"}, msg("start", nil), true},
		"only, command name any case":   {map[string]any{"commands": "only", "command_names": []any{"Help"}}, msg("help", nil), true},
		"only, other command name":      {map[string]any{"commands": "only", "command_names": []any{"help"}}, msg("start", nil), false},
		"include with names: plain":     {map[string]any{"commands": "include", "command_names": []any{"help"}}, msg("", nil), true},
		"include with names: listed":    {map[string]any{"commands": "include", "command_names": []any{"help"}}, msg("help", nil), true},
		"include with names: other":     {map[string]any{"commands": "include", "command_names": []any{"help"}}, msg("start", nil), false},
		"ignore: names narrow nothing":  {map[string]any{"commands": "ignore", "command_names": []any{"start"}}, msg("", nil), true},
		"ignore: a command stays out":   {map[string]any{"commands": "ignore", "command_names": []any{"start"}}, msg("start", nil), false},
		"chat id":                       {map[string]any{"chat_ids": []any{"7"}}, msg("", map[string]any{"chatId": "8"}), false},
		"private only, a group":         {map[string]any{"private_only": true}, msg("", map[string]any{"chatType": "group"}), false},
		"private only, private":         {map[string]any{"private_only": true}, msg("", nil), true},
	} {
		t.Run(name, func(t *testing.T) {
			parsed, issues := spec.TriggerParams.Parse(tc.params)
			require.Empty(t, issues)
			got := true
			for key, f := range spec.Match {
				wants, _ := parsed[key].([]any)
				if len(wants) == 0 {
					continue
				}
				value := tc.event[f.Payload]
				hit := false
				for _, w := range wants {
					a, b := fmt.Sprint(w), fmt.Sprint(value)
					if f.Fold == graph.FoldLower {
						a, b = strings.ToLower(a), strings.ToLower(b)
					}
					hit = hit || a == b
				}
				got = got && hit
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestPlatformWebhookSecretShape(t *testing.T) {
	s := telegram.WebhookSecret(platformToken, "platform:telegram_bot")
	assert.Regexp(t, `^[A-Za-z0-9_-]{32}$`, s)
	assert.NotEqual(t, s, telegram.WebhookSecret(tenantToken, "platform:telegram_bot"), "per token")
}

// A connection's secret is generated when it is first provisioned and kept in its inbound state: 32
// characters of the Bot API's alphabet, stable afterwards, and never derived from the token.
func TestTenantWebhookSecretIsGeneratedAndStored(t *testing.T) {
	r := newRig(t)
	first := r.bot.Webhook().Secret
	assert.Regexp(t, `^[A-Za-z0-9_-]{32}$`, first)
	state, err := r.prov(context.Background(), r.conn, r.creds(), nil)
	require.NoError(t, err)
	var held struct {
		Secret string `json:"secret"`
	}
	require.NoError(t, json.Unmarshal(state["telegram_webhook"], &held))
	assert.Equal(t, r.bot.Webhook().Secret, held.Secret, "what was set at Telegram is what is handed back to be kept")
	assert.Equal(t, first, held.Secret, "provisioning again keeps the secret")
	assert.NotEqual(t, first, telegram.WebhookSecret(tenantToken, r.conn.ID.String()), "not derived from the token")
}

func TestTestReturnsTheBotsIdentity(t *testing.T) {
	r := newRig(t)
	id, err := r.c.Test(context.Background(), r.creds())
	require.NoError(t, err)
	assert.Equal(t, "@tenant_bot", id.Display)
	assert.Equal(t, "111111", id.External)
	assert.Equal(t, connectors.IdentityHandle, id.Kind)

	_, err = r.c.Test(context.Background(), connectors.Credentials{"token": "999999:CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 401")
	assert.NotContains(t, err.Error(), "CCCCCC", "the token is never in an error")
}

func TestProvisionSetsTheWebhookAndDeprovisionRemovesIt(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	hook := r.bot.Webhook()
	assert.Equal(t, origin+"/automations/hooks/"+r.conn.ID.String(), hook.URL)
	assert.Regexp(t, `^[A-Za-z0-9_-]{32}$`, hook.Secret)

	require.NoError(t, r.c.Deprovision(ctx, r.conn, r.creds()))
	assert.Empty(t, r.bot.Webhook().URL)
	assert.Len(t, r.bot.CallsOf("deleteWebhook"), 1)
}

// A webhook another service set is left as it is when this connection goes away.
func TestDeprovisionLeavesAForeignWebhook(t *testing.T) {
	r := newRig(t)
	r.bot.SetWebhook("https://other.example.org/hook", "s")
	require.NoError(t, r.c.Deprovision(context.Background(), r.conn, r.creds()))
	assert.Equal(t, "https://other.example.org/hook", r.bot.Webhook().URL)
	assert.Empty(t, r.bot.CallsOf("deleteWebhook"))
}

func TestPlatformBotIsRegisteredAtStart(t *testing.T) {
	r := newRig(t)
	pc, ok := r.c.PlatformCredential()
	require.True(t, ok)
	assert.Equal(t, platformToken, pc.Credentials["token"])
	require.NoError(t, r.c.StartPlatform(context.Background(), r.platformCreds()))
	assert.Equal(t, origin+"/automations/hooks/platform/telegram_bot", r.platform.Webhook().URL)
	assert.Equal(t, r.secret(r.platformConn(), platformToken), r.platform.Webhook().Secret)

	none := telegram.New(telegram.Config{})
	_, ok = none.PlatformCredential()
	assert.False(t, ok, "without APP_TELEGRAMBOTTOKEN there is no platform credential")
	assert.Equal(t, connectors.PlatformAuto, r.c.Descriptor().Platform)
}

func TestAdviseWarnsAboutAForeignWebhookOrAnUnknownReader(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	r.bot.SetWebhook("", "") // a bot with no webhook at all
	// Whether another service polls the bot cannot be told without taking over its long poll: a new
	// connection needs the person's word, and the connector never asks for updates.
	r.bot.SetPolling(true)
	w, err := r.c.Advise(ctx, nil, r.creds())
	require.NoError(t, err)
	assert.Equal(t, []connectors.Warning{{Code: telegram.WarnPollingUnknown}}, w)
	assert.Empty(t, r.bot.CallsOf("getUpdates"), "a read of the bot's updates would interrupt a poller")
	w, err = r.c.Advise(ctx, &r.conn, r.creds())
	require.NoError(t, err)
	assert.Equal(t, []connectors.Warning{{Code: telegram.WarnPollingUnknown}}, w, "nor is a bot a connection had: it may be polled since")

	r.bot.SetWebhook("https://other.example.org/hook", "s")
	w, err = r.c.Advise(ctx, nil, r.creds())
	require.NoError(t, err)
	assert.Equal(t, []connectors.Warning{{Code: telegram.WarnForeignWebhook}}, w)
	w, err = r.c.Advise(ctx, &r.conn, r.creds())
	require.NoError(t, err)
	assert.Equal(t, []connectors.Warning{{Code: telegram.WarnForeignWebhook}}, w, "re-enabling asks about a webhook taken meanwhile")

	// A webhook of this application for a connection that is gone is nobody's: no warning.
	r.bot.SetWebhook(origin+"/automations/hooks/"+uuid.NewString(), "s")
	w, err = r.c.Advise(ctx, nil, r.creds())
	require.NoError(t, err)
	assert.Empty(t, w)
	assert.Equal(t, origin+"/automations/hooks/", r.bot.Webhook().URL[:len(origin+"/automations/hooks/")], "advice changes nothing at Telegram")
}

// Provision looks at the webhook as it is when it changes it: advice that was stale (the webhook was free
// when the person was asked and is foreign now) never lets a foreign webhook be replaced unconfirmed.
func TestProvisionChecksConsentAgainstTheCurrentWebhook(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	r.bot.SetWebhook("https://other.example.org/hook", "s")
	_, err := r.prov(ctx, r.conn, r.creds(), nil)
	var need *connectors.ConfirmationRequiredError
	require.ErrorAs(t, err, &need)
	assert.Equal(t, "https://other.example.org/hook", r.bot.Webhook().URL, "nothing was changed")
	_, err = r.prov(ctx, r.conn, r.creds(), []string{"something_else"})
	require.ErrorAs(t, err, &need)
	_, err = r.prov(ctx, r.conn, r.creds(), []string{telegram.WarnForeignWebhook})
	require.NoError(t, err)
	assert.Equal(t, origin+"/automations/hooks/"+r.conn.ID.String(), r.bot.Webhook().URL)
}

// A connection keeps its secret across provisioning: a save that did not commit, or a competing one, never
// leaves Telegram with a secret the SDK does not hold.
func TestProvisionKeepsTheStoredSecret(t *testing.T) {
	r := newRig(t)
	first := r.bot.Webhook().Secret
	state, err := r.prov(context.Background(), r.conn, r.creds(), nil)
	require.NoError(t, err)
	var held struct {
		Secret string `json:"secret"`
	}
	require.NoError(t, json.Unmarshal(state["telegram_webhook"], &held))
	assert.Equal(t, first, held.Secret)
	assert.Equal(t, first, r.bot.Webhook().Secret)
	// A new connection (nothing stored) gets a generated one.
	fresh := connectors.Connection{ID: uuid.New(), TenantID: r.tenant, Kind: telegram.Kind, Name: "fresh"}
	r.st.exists[fresh.ID] = true
	r.bot.SetWebhook("", "")
	_, err = r.prov(context.Background(), fresh, r.creds(), nil)
	var need *connectors.ConfirmationRequiredError
	require.ErrorAs(t, err, &need, "a bot with no webhook is taken over only with the person's word")
	state, err = r.prov(context.Background(), fresh, r.creds(), []string{telegram.WarnPollingUnknown})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(state["telegram_webhook"], &held))
	assert.NotEqual(t, first, held.Secret)
}

// The platform bot is registered for the credential in effect, which is the one the application hands over.
func TestStartPlatformUsesTheEffectiveCredential(t *testing.T) {
	r := newRig(t)
	other := r.api.Bot("333333:CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC", "configured_bot")
	require.NoError(t, r.c.StartPlatform(context.Background(), connectors.Credentials{"token": "333333:CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"}))
	assert.Equal(t, origin+"/automations/hooks/platform/telegram_bot", other.Webhook().URL)
	assert.Empty(t, r.platform.Webhook().URL, "not the connector's own token when the configuration names another")
	assert.Equal(t, "333333", r.c.ExternalID(connectors.Credentials{"token": "333333:CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"}))

	none := telegram.New(telegram.Config{PublicOrigin: origin, APIURL: r.api.URL()})
	require.NoError(t, none.StartPlatform(context.Background(), connectors.Credentials{"token": platformToken}), "an explicit credential alone is enough")
	assert.Equal(t, origin+"/automations/hooks/platform/telegram_bot", r.platform.Webhook().URL)
}

// After a rotation of the platform credential the retired bot keeps a webhook on the platform's route that
// nobody serves for it: it is free to be connected as a tenant's own bot, while the bot that is the
// platform's now stays taken.
func TestARetiredPlatformBotCanBeConnectedByATenant(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	require.NoError(t, r.c.StartPlatform(ctx, r.platformCreds()))
	assert.Equal(t, origin+"/automations/hooks/platform/telegram_bot", r.platform.Webhook().URL)

	next := "333333:CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"
	r.api.Bot(next, "next_platform_bot")
	require.NoError(t, r.c.StartPlatform(ctx, connectors.Credentials{"token": next}))

	w, err := r.c.Advise(ctx, nil, r.platformCreds())
	require.NoError(t, err)
	assert.Empty(t, w, "the retired bot's webhook on the platform route is stale: no block, no confirmation")
	_, err = r.prov(ctx, r.conn, r.platformCreds(), nil)
	require.NoError(t, err, "and it can be connected")

	w, err = r.c.Advise(ctx, nil, connectors.Credentials{"token": next})
	require.NoError(t, err)
	require.Len(t, w, 1)
	assert.True(t, w[0].Blocking, "the current platform bot stays the platform's")
}

// One bot has one webhook: a bot another live connection owns (or the platform's) cannot be connected
// twice, and removing the older connection never removes the newer one's webhook.
func TestABotBelongsToOneConnection(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	other := connectors.Connection{ID: uuid.New(), TenantID: uuid.New(), Kind: telegram.Kind, Name: "second"}
	r.st.exists[other.ID] = true

	w, err := r.c.Advise(ctx, nil, r.creds())
	require.NoError(t, err)
	require.Len(t, w, 1)
	assert.Equal(t, telegram.WarnBotInUse, w[0].Code)
	assert.True(t, w[0].Blocking, "no confirmation lifts it")
	w, err = r.c.Advise(ctx, &r.conn, r.creds())
	require.NoError(t, err)
	assert.Empty(t, w, "the connection that owns the bot may be saved again")
	w, err = r.c.Advise(ctx, &other, r.creds())
	require.NoError(t, err)
	assert.Len(t, w, 1, "another connection may not")

	require.NoError(t, r.c.Deprovision(ctx, other, r.creds()))
	assert.Equal(t, origin+"/automations/hooks/"+r.conn.ID.String(), r.bot.Webhook().URL, "an exact URL only is removed")

	// The platform bot's token cannot be taken by a tenant connection either.
	require.NoError(t, r.c.StartPlatform(ctx, r.platformCreds()))
	w, err = r.c.Advise(ctx, nil, r.platformCreds())
	require.NoError(t, err)
	require.Len(t, w, 1)
	assert.True(t, w[0].Blocking)
	_, err = r.prov(ctx, r.conn, r.platformCreds(), nil)
	require.Error(t, err, "the platform's route is taken at the provider too")

	// Once the owner is gone its webhook is stale and the bot is free to be taken over.
	delete(r.st.exists, r.conn.ID)
	w, err = r.c.Advise(ctx, &other, r.creds())
	require.NoError(t, err)
	assert.Empty(t, w)
	_, err = r.prov(ctx, other, r.creds(), nil)
	require.NoError(t, err)
}

func TestVerifyComparesTheSecret(t *testing.T) {
	r := newRig(t)
	good := r.secret(r.conn, tenantToken)
	upd := r.bot.Update(900, alice, "hi")
	code, _ := r.post(r.conn, r.creds(), "", upd)
	assert.Equal(t, http.StatusNotFound, code, "no secret")
	last := good[len(good)-1]
	wrong := good[:len(good)-1] + string(map[bool]byte{true: 'b', false: 'a'}[last == 'a'])
	code, _ = r.post(r.conn, r.creds(), wrong, upd)
	assert.Equal(t, http.StatusNotFound, code, "a wrong secret")
	code, _ = r.post(r.conn, r.creds(), telegram.WebhookSecret(tenantToken, r.conn.ID.String()), upd)
	assert.Equal(t, http.StatusNotFound, code, "a secret derived from the token is not the stored one")
	code, evs := r.post(r.conn, r.creds(), good, upd)
	assert.Equal(t, http.StatusOK, code)
	assert.Len(t, evs, 1)
}

// A message to the platform bot belongs to no tenant: a tenant that has only the platform credential may
// not author the message trigger, and one with its own bot may, only with that bot.
func TestMessageTriggerNeedsATenantsOwnBot(t *testing.T) {
	r := newRig(t)
	platformOnly := catalog.NewRegistry(avail{telegram.Kind: {{ID: "platform:telegram_bot", Name: "platform"}}})
	require.NoError(t, connectors.NewDirectory(platformOnly).Register(r.c))
	v, err := platformOnly.ForTenant(context.Background(), r.tenant)
	require.NoError(t, err)
	assert.Equal(t, "connection_unavailable", v.Unavailable[telegram.EventMessage], "no trigger on the platform bot")
	assert.NotContains(t, v.Catalog.Events, telegram.EventMessage)
	assert.Contains(t, v.Catalog.Events, telegram.EventCallback, "the press of a button is still known")
	assert.Contains(t, v.Catalog.Nodes, telegram.NodeSend, "sending through the platform bot stays")

	both := catalog.NewRegistry(avail{telegram.Kind: {{ID: r.conn.ID.String(), Name: "bot"}, {ID: "platform:telegram_bot", Name: "platform"}}})
	require.NoError(t, connectors.NewDirectory(both).Register(telegram.New(telegram.Config{})))
	v, err = both.ForTenant(context.Background(), r.tenant)
	require.NoError(t, err)
	const g = `{"name":"t","triggers":[{"id":"m","event":"telegram.message_received.v1","params":{"connection":"platform:telegram_bot"}}],"nodes":[],"edges":[{"from":{"node":"m","outcome":"fired"},"to":"ignore"}]}`
	parsed, derrs := graph.Decode([]byte(g))
	require.Empty(t, derrs)
	errs := graph.Validate(v.Catalog, parsed, graph.Options{Rules: catalog.ConnectionRule(v)})
	var found bool
	for _, e := range errs {
		found = found || e.Code == catalog.CodeConnectionUnavailable
	}
	assert.True(t, found, "the platform credential is not a connection of the trigger: %v", errs)
	ok := strings.Replace(g, "platform:telegram_bot", r.conn.ID.String(), 1)
	parsed, derrs = graph.Decode([]byte(ok))
	require.Empty(t, derrs)
	for _, e := range graph.Validate(v.Catalog, parsed, graph.Options{Rules: catalog.ConnectionRule(v)}) {
		assert.NotEqual(t, catalog.CodeConnectionUnavailable, e.Code)
	}
}

// A read waits for a message that only a tenant's own bot receives: the platform credential is neither
// offered for it nor accepted, though sending and asking through it stays.
func TestReadNeedsATenantsOwnBot(t *testing.T) {
	r := newRig(t)
	reg := catalog.NewRegistry(avail{telegram.Kind: {{ID: r.conn.ID.String(), Name: "bot"}, {ID: "platform:telegram_bot", Name: "platform"}}})
	require.NoError(t, connectors.NewDirectory(reg).Register(telegram.New(telegram.Config{})))
	v, err := reg.ForTenant(context.Background(), r.tenant)
	require.NoError(t, err)
	assert.True(t, v.TenantOnlyNodes[telegram.NodeRead])
	assert.False(t, v.TenantOnlyNodes[telegram.NodeSend])
	platformOnly := catalog.NewRegistry(avail{telegram.Kind: {{ID: "platform:telegram_bot", Name: "platform"}}})
	require.NoError(t, connectors.NewDirectory(platformOnly).Register(telegram.New(telegram.Config{})))
	pv, err := platformOnly.ForTenant(context.Background(), r.tenant)
	require.NoError(t, err)
	assert.Equal(t, "connection_unavailable", pv.Unavailable[telegram.NodeRead])
	assert.Contains(t, pv.Catalog.Nodes, telegram.NodeSend)
	assert.Contains(t, pv.Catalog.Nodes, telegram.NodeAsk)
}

// The secret is bound to the bot's account: the same bot keeps it, another bot (a replaced token) gets a new
// one, so an update in flight from the former bot cannot pass for the replacement's.
func TestTheSecretIsBoundToTheBot(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	first := r.bot.Webhook().Secret
	other := r.api.Bot("444444:DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD", "other_bot")
	state, err := r.prov(ctx, r.conn, connectors.Credentials{"token": "444444:DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD"}, []string{telegram.WarnPollingUnknown})
	require.NoError(t, err)
	var held struct {
		Secret  string `json:"secret"`
		Account string `json:"account"`
	}
	require.NoError(t, json.Unmarshal(state["telegram_webhook"], &held))
	assert.NotEqual(t, first, held.Secret, "another bot: another secret")
	assert.Equal(t, "444444", held.Account)
	assert.Equal(t, held.Secret, other.Webhook().Secret)

	// A state that cannot be read is an error, not a reason to rotate.
	_, err = r.c.Provision(ctx, r.conn, r.creds(), []string{telegram.WarnPollingUnknown}, json.RawMessage(`not json`))
	require.Error(t, err)
}
