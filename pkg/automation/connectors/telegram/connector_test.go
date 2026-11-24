package telegram_test

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
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
	for name, tc := range map[string]struct {
		params map[string]any
		event  map[string]any
		want   bool
	}{
		"plain message, default ignore": {nil, map[string]any{"isCommand": false, "chatType": "private", "chatId": "1"}, true},
		"command, default ignore":       {nil, map[string]any{"isCommand": true, "command": "start", "chatType": "private", "chatId": "1"}, false},
		"command, only":                 {map[string]any{"commands": "only"}, map[string]any{"isCommand": true, "command": "start"}, true},
		"plain message, only":           {map[string]any{"commands": "only"}, map[string]any{"isCommand": false}, false},
		"include takes both":            {map[string]any{"commands": "include"}, map[string]any{"isCommand": false}, true},
		"command name, any case":        {map[string]any{"commands": "only", "command_names": []any{"Help"}}, map[string]any{"isCommand": true, "command": "help"}, true},
		"other command name":            {map[string]any{"commands": "only", "command_names": []any{"help"}}, map[string]any{"isCommand": true, "command": "start"}, false},
		"chat id":                       {map[string]any{"chat_ids": []any{"7"}}, map[string]any{"isCommand": false, "chatId": "8"}, false},
		"private only, a group":         {map[string]any{"private_only": true}, map[string]any{"isCommand": false, "chatType": "group"}, false},
		"private only, private":         {map[string]any{"private_only": true}, map[string]any{"isCommand": false, "chatType": "private"}, true},
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

func TestWebhookSecretShape(t *testing.T) {
	s := telegram.WebhookSecret(tenantToken, uuid.NewString())
	assert.Regexp(t, regexp.MustCompile(`^[A-Za-z0-9_-]{32}$`), s)
	assert.NotEqual(t, s, telegram.WebhookSecret(tenantToken, uuid.NewString()), "per connection")
	assert.NotEqual(t, s, telegram.WebhookSecret(platformToken, uuid.NewString()), "per token")
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
	require.NoError(t, r.c.Provision(ctx, r.conn, r.creds()))
	hook := r.bot.Webhook()
	assert.Equal(t, origin+"/automations/hooks/"+r.conn.ID.String(), hook.URL)
	assert.Equal(t, r.secret(r.conn, tenantToken), hook.Secret)
	assert.Regexp(t, regexp.MustCompile(`^[A-Za-z0-9_-]{32}$`), hook.Secret)

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
	require.NoError(t, r.c.StartPlatform(context.Background()))
	assert.Equal(t, origin+"/automations/hooks/platform/telegram_bot", r.platform.Webhook().URL)
	assert.Equal(t, r.secret(r.platformConn(), platformToken), r.platform.Webhook().Secret)

	none := telegram.New(telegram.Config{})
	_, ok = none.PlatformCredential()
	assert.False(t, ok, "without APP_TELEGRAMBOTTOKEN there is no platform credential")
	assert.Equal(t, connectors.PlatformAuto, r.c.Descriptor().Platform)
}

func TestAdviseWarnsAboutAForeignWebhookOrAPoller(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	w, err := r.c.Advise(ctx, r.creds())
	require.NoError(t, err)
	assert.Empty(t, w, "a bot nobody uses")

	r.bot.SetWebhook("https://other.example.org/hook", "s")
	w, err = r.c.Advise(ctx, r.creds())
	require.NoError(t, err)
	assert.Equal(t, []connectors.Warning{{Code: telegram.WarnForeignWebhook}}, w)

	r.bot.SetWebhook(origin+"/automations/hooks/"+uuid.NewString(), "s")
	w, err = r.c.Advise(ctx, r.creds())
	require.NoError(t, err)
	assert.Empty(t, w, "this application's own webhook is not foreign")

	r.bot.SetWebhook("", "")
	r.bot.SetPolling(true)
	w, err = r.c.Advise(ctx, r.creds())
	require.NoError(t, err)
	assert.Equal(t, []connectors.Warning{{Code: telegram.WarnPollingInUse}}, w)
	assert.Empty(t, r.bot.Webhook().URL, "advice changes nothing at Telegram")
}

func TestVerifyComparesTheSecret(t *testing.T) {
	r := newRig(t)
	good := r.secret(r.conn, tenantToken)
	upd := r.bot.Update(900, alice, "hi")
	code, _ := r.post(r.conn, r.creds(), "", upd)
	assert.Equal(t, http.StatusNotFound, code, "no secret")
	code, _ = r.post(r.conn, r.creds(), good[:31]+"x", upd)
	assert.Equal(t, http.StatusNotFound, code, "a wrong secret")
	code, _ = r.post(r.conn, r.creds(), r.secret(r.platformConn(), tenantToken), upd)
	assert.Equal(t, http.StatusNotFound, code, "the secret of another connection")
	code, evs := r.post(r.conn, r.creds(), good, upd)
	assert.Equal(t, http.StatusOK, code)
	assert.Len(t, evs, 1)
}
