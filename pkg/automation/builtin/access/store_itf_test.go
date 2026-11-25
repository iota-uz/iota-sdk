package access_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/access"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/code"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/email"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/httpapi"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/openai"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram/telegramtest"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

const (
	itfAPIToken = "api-token-8d1c-very-secret"
	itfBotToken = "123456:AAH-itf-bot-token-very-secret"
)

// storeRig is a tenant's connections in PostgreSQL, opened for code through the SDK's connectors.
type storeRig struct {
	ctx    context.Context
	tenant uuid.UUID
	store  connectors.Store
	conns  access.Connections
	tg     *telegramtest.Server
}

func newStoreRig(t *testing.T) *storeRig {
	t.Helper()
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "automation_access",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	keys, err := connectors.NewKeyring("k1", map[string][]byte{"k1": bytes.Repeat([]byte{7}, 32)})
	require.NoError(t, err)
	tg := telegramtest.New()
	t.Cleanup(tg.Close)
	dir := connectors.NewDirectory(nil)
	list := []connectors.Connector{httpapi.New(), telegram.New(telegram.Config{APIURL: tg.URL()}), openai.New()}
	list = append(list, email.New(email.Config{})...)
	for _, c := range list {
		require.NoError(t, dir.Register(c))
	}
	st := connectors.NewStore(scope.Pool, keys, nil, nil, nil)
	return &storeRig{ctx: context.Background(), tenant: scope.Tenant.ID, store: st, conns: access.StoreConnections(st, dir, nil), tg: tg}
}

func (r *storeRig) save(t *testing.T, kind catalog.ConnectorKind, name string, creds connectors.Credentials) connectors.Connection {
	t.Helper()
	c, err := r.store.Save(r.ctx, connectors.Connection{TenantID: r.tenant, Kind: kind, Name: name, DisplayName: name, Status: connectors.StatusConnected}, creds)
	require.NoError(t, err)
	return c
}

// Every SDK connector type is opened from the store for code, and refused once disabled or broken.
func TestStoreConnectionsOfEverySDKConnector(t *testing.T) {
	r := newStoreRig(t)
	api := r.save(t, httpapi.Kind, "crm", connectors.Credentials{"base_url": "https://crm.example.com/v1", "auth": "bearer", "token": itfAPIToken})
	bot := r.save(t, telegram.Kind, "bot", connectors.Credentials{"token": itfBotToken})
	ai := r.save(t, openai.Kind, "llm", connectors.Credentials{"api_key": "sk-itf-key-1234567890"})
	mail := r.save(t, email.KindSMTP, "mail", connectors.Credentials{"host": "smtp.example.com", "port": "465", "username": "u", "password": "p-secret", "from": "u@example.com"})
	inbox := r.save(t, email.KindIMAP, "inbox", connectors.Credentials{"host": "imap.example.com", "port": "993", "username": "u", "password": "p-secret", "mailbox": "INBOX"})
	r.tg.Bot(itfBotToken, "itf_bot")

	t.Run("HTTP/API: base URL and a fetch with the token injected", func(t *testing.T) {
		opened, err := access.OpenAll(r.ctx, r.conns, r.tenant, []string{"crm"})
		require.NoError(t, err)
		c := opened["crm"]
		assert.Equal(t, "https://crm.example.com/v1", c.Access.BaseURL)
		assert.True(t, c.Access.Fetch)
		rec := &recorder{}
		g := &access.Gateway{Fetcher: rec, Connections: opened}
		_, err = g.CallConnection(r.ctx, code.ConnectionRequest{Connection: "crm", Method: "fetch", Fetch: true, HTTPMethod: "GET", Path: "/deals?x=1"})
		require.NoError(t, err)
		assert.Equal(t, "https://crm.example.com/v1/deals?x=1", rec.got.URL)
		assert.Equal(t, "Bearer "+itfAPIToken, rec.got.Header.Get("Authorization"))
		assert.Contains(t, c.Values, itfAPIToken, "the token is masked in what the node returns")
	})

	t.Run("Telegram: bot metadata through the store's credentials", func(t *testing.T) {
		opened, err := access.OpenAll(r.ctx, r.conns, r.tenant, []string{"bot"})
		require.NoError(t, err)
		g := &access.Gateway{Connections: opened, Input: catalog.CallInput{TenantID: r.tenant, RunID: uuid.New()}}
		res, err := g.CallOperation(r.ctx, code.OperationRequest{Connection: "bot", Operation: "getMe", Args: []byte("null")})
		require.NoError(t, err)
		var me map[string]any
		require.NoError(t, json.Unmarshal(res, &me))
		assert.Equal(t, "itf_bot", me["username"])
		assert.NotContains(t, string(res), itfBotToken)
	})

	t.Run("AI and Email expose their operations", func(t *testing.T) {
		opened, err := access.OpenAll(r.ctx, r.conns, r.tenant, []string{"llm", "mail"})
		require.NoError(t, err)
		assert.Equal(t, []string{"complete"}, opened["llm"].Access.OperationNames())
		assert.Equal(t, []string{"send"}, opened["mail"].Access.OperationNames())
		assert.True(t, opened["llm"].HasOperator())
		assert.True(t, opened["mail"].HasOperator())
	})

	t.Run("IMAP exposes its read operations", func(t *testing.T) {
		opened, err := access.OpenAll(r.ctx, r.conns, r.tenant, []string{"inbox"})
		require.NoError(t, err)
		assert.Equal(t, []string{"listMessages", "getMessage"}, opened["inbox"].Access.OperationNames())
		assert.True(t, opened["inbox"].HasOperator())
		assert.Empty(t, opened["inbox"].Access.Methods)
		assert.False(t, opened["inbox"].Access.Fetch)
	})

	for _, c := range []connectors.Connection{api, bot, ai, mail, inbox} {
		require.NoError(t, r.store.SetStatus(r.ctx, r.tenant, c.ID, connectors.StatusDisabled, ""))
		_, err := access.OpenAll(r.ctx, r.conns, r.tenant, []string{c.Name})
		var ce *catalog.CallError
		require.ErrorAs(t, err, &ce, c.Kind)
		assert.Equal(t, access.CodeConnectionDisabled, ce.Code, c.Kind)
		assert.False(t, strings.Contains(ce.Message, itfAPIToken) || strings.Contains(ce.Message, itfBotToken))

		require.NoError(t, r.store.SetStatus(r.ctx, r.tenant, c.ID, connectors.StatusNeedsReauth, ""))
		_, err = access.OpenAll(r.ctx, r.conns, r.tenant, []string{c.Name})
		require.ErrorAs(t, err, &ce, c.Kind)
		assert.Equal(t, access.CodeConnectionBroken, ce.Code, c.Kind)
	}

	// Another tenant's code never reaches these connections.
	other := uuid.New()
	_, err := access.OpenAll(r.ctx, r.conns, other, []string{"crm"})
	var ce *catalog.CallError
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, access.CodeConnectionMissing, ce.Code)
}
