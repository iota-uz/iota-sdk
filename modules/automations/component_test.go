package automations_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/automations"
	"github.com/iota-uz/iota-sdk/modules/automations/services"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/email"
	"github.com/iota-uz/iota-sdk/pkg/clienthost"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/sdkidentity"
)

// Every key the screen reads is in the module's locale (the host passes MessageKeys to it).
func TestScreenKeysAreInTheLocale(t *testing.T) {
	t.Parallel()
	keys, err := automations.MessageKeys()
	require.NoError(t, err)
	have := map[string]bool{}
	for _, k := range keys {
		have[k] = true
	}
	src, err := os.ReadFile(filepath.Join("modules", "automations", "presentation", "integrations", "web", "Integrations.tsx"))
	require.NoError(t, err)
	model, err := os.ReadFile(filepath.Join("modules", "automations", "presentation", "integrations", "web", "model.ts"))
	require.NoError(t, err)
	re := regexp.MustCompile(`\bt\('([A-Za-z_.]+)'`)
	found := 0
	for _, m := range re.FindAllSubmatch(append(src, model...), -1) {
		found++
		assert.True(t, have["Automations.Integrations."+string(m[1])], "missing locale key %s", m[1])
	}
	assert.Greater(t, found, 50)
	// Keys built from values (status, auth, category, OAuth errors).
	for _, k := range []string{
		"Status.connected", "Status.needs_reauth", "Status.error", "Status.disabled",
		"Auth.api_key", "Auth.basic", "Auth.oauth2", "Auth.custom",
		"Categories.Messaging", "Categories.Email", "Categories.AI", "Categories.Documents", "Categories.Misc",
		"OAuth.denied", "OAuth.state", "OAuth.exchange", "OAuth.unavailable",
		"UsedBy.Singular", "UsedBy.Paucal", "UsedBy.Multiple", "UsedBy.Plural", "UsedBy.None",
	} {
		assert.True(t, have["Automations.Integrations."+k], "missing locale key %s", k)
	}
}

type noop struct{}

func (noop) Kind() catalog.ConnectorKind       { return "noop" }
func (noop) Nodes() []catalog.NodeType         { return nil }
func (noop) Triggers() []catalog.TriggerSource { return nil }
func (noop) Descriptor() connectors.Descriptor {
	return connectors.Descriptor{Kind: "noop", Title: map[string]string{"en": "Noop"}, Auth: connectors.AuthAPIKey,
		Credentials: json.RawMessage(`{"type":"object","properties":{"key":{"type":"string","x-secret":true}}}`)}
}
func (noop) Test(context.Context, connectors.Credentials) (connectors.Identity, error) {
	return connectors.Identity{Display: "ok"}, nil
}

// The module provides the directory, the registry, the store and the secret resolver the extension
// runtime serves secrets.get with; with a host manifest the settings hub lists the page.
func TestComponentWiresConnectionsAndTheSettingsPage(t *testing.T) {
	src, err := config.Build(static.New(map[string]any{"automation.connections.keys": "k1:" + string(bytes.Repeat([]byte("ab"), 32))}))
	require.NoError(t, err)
	manifest := clienthost.Manifest{
		Package: "app", PackageVersion: "1.0.0", SDKCommit: strings.Repeat("a", 40), SDKReleaseVersion: sdkidentity.ReleaseVersion,
		ProtocolVersion: sdkidentity.ProtocolVersion, Entry: "/assets/app.js",
	}
	components := []composition.Component{}
	for _, c := range modules.Components() {
		if c.Descriptor().Name != "automations" {
			components = append(components, c)
		}
	}
	components = append(components, automations.NewComponent(automations.WithConnectors(noop{}), automations.WithHostManifest(manifest)))
	f := itf.Setup(t, itf.WithComponents(components...), itf.WithUser(itf.User()), itf.WithSource(src))

	dir, err := composition.Resolve[connectors.Directory](f.Container)
	require.NoError(t, err)
	_, ok := dir.Get("noop")
	require.True(t, ok)
	reg, err := composition.Resolve[catalog.Registry](f.Container)
	require.NoError(t, err)
	// The notification nodes of graphs and their definition come with the module.
	for _, node := range []string{"notify", "send_to_contact"} {
		_, ok := reg.Node(node)
		assert.True(t, ok, "node %s is registered", node)
	}
	definitions, err := composition.Resolve[*notifications.Catalog](f.Container)
	require.NoError(t, err)
	def, ok := definitions.Get("automations.notify.v1")
	require.True(t, ok)
	assert.Equal(t, "automations", def.Module)
	store, err := composition.Resolve[connectors.ConnectionStore](f.Container)
	require.NoError(t, err)
	_, err = composition.Resolve[host.SecretResolver](f.Container)
	require.NoError(t, err)
	_, err = store.List(f.Ctx, uuid.New(), "")
	require.NoError(t, err)
	// The configured keyring reaches the store: fingerprinting needs the active key.
	full, err := composition.Resolve[connectors.Store](f.Container)
	require.NoError(t, err)
	fp, err := full.Fingerprint([]byte("payload"))
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(fp, "k1:"), fp)

	var pages []string
	for _, s := range f.Container.SettingsSections() {
		for _, p := range s.Pages {
			if p.Key == automations.SettingsPageIntegrations {
				pages = append(pages, s.Key+":"+p.Href)
			}
		}
	}
	assert.Equal(t, []string{"organization:/settings/integrations"}, pages)
}

// Without connection keys the module still builds; the store refuses with ErrNotConfigured (the page
// shows a warning) instead of failing the application.
func TestComponentWithoutKeysIsUnconfigured(t *testing.T) {
	src, err := config.Build(static.New(map[string]any{}))
	require.NoError(t, err)
	components := []composition.Component{}
	for _, c := range modules.Components() {
		if c.Descriptor().Name != "automations" {
			components = append(components, c)
		}
	}
	components = append(components, automations.NewComponent(automations.WithConnectors(noop{})))
	f := itf.Setup(t, itf.WithComponents(components...), itf.WithUser(itf.User()), itf.WithSource(src))
	store, err := composition.Resolve[connectors.ConnectionStore](f.Container)
	require.NoError(t, err)
	_, err = store.List(f.Ctx, uuid.New(), "")
	require.ErrorIs(t, err, connectors.ErrNotConfigured)
}

// The SDK's AI connectors are in the directory, and the platform keys the environment configures are
// offered by opt-in; an explicit platform credential of the kind wins over the environment.
func TestComponentRegistersTheAIConnectors(t *testing.T) {
	src, err := config.Build(static.New(map[string]any{
		"automation.openai.apikey":        "sk-openai",
		"automation.openai.baseurl":       "https://llm.example.com/v1",
		"automation.jev.apikey":           "sk-jev",
		"automation.jev.provider":         "openai",
		"automation.platform.credentials": `{"jev": {"provider": "openrouter", "api_key": "sk-explicit"}}`,
	}))
	require.NoError(t, err)
	components := []composition.Component{}
	for _, c := range modules.Components() {
		if c.Descriptor().Name != "automations" {
			components = append(components, c)
		}
	}
	components = append(components, automations.NewComponent())
	f := itf.Setup(t, itf.WithComponents(components...), itf.WithUser(itf.User()), itf.WithSource(src))

	dir, err := composition.Resolve[connectors.Directory](f.Container)
	require.NoError(t, err)
	for _, kind := range []catalog.ConnectorKind{catalog.KindOpenAI, catalog.KindJEV} {
		c, ok := dir.Get(kind)
		require.True(t, ok, kind)
		assert.Equal(t, connectors.PlatformOptIn, c.Descriptor().Platform, kind)
		assert.Equal(t, connectors.CategoryAI, c.Descriptor().Category, kind)
	}
	reg, err := composition.Resolve[catalog.Registry](f.Container)
	require.NoError(t, err)
	_, hasLLM := reg.Node("ai.llm")
	_, hasJev := reg.Node("ai.jev")
	assert.True(t, hasLLM)
	assert.True(t, hasJev)

	platform, err := composition.Resolve[connectors.PlatformCredentials](f.Container)
	require.NoError(t, err)
	creds, _, ok := platform.Platform(catalog.KindOpenAI)
	require.True(t, ok)
	assert.Equal(t, connectors.Credentials{"api_key": "sk-openai", "base_url": "https://llm.example.com/v1"}, creds)
	creds, _, ok = platform.Platform(catalog.KindJEV)
	require.True(t, ok)
	assert.Equal(t, connectors.Credentials{"provider": "openrouter", "api_key": "sk-explicit"}, creds, "an explicit platform credential wins")
	assert.Equal(t, []catalog.ConnectorKind{catalog.KindJEV, catalog.KindOpenAI}, platform.(connectors.PlatformCatalog).Kinds())
}

// The registry the module provides learns from the store which platform keys a tenant cannot use: a node that
// leaves its connection out ends with unavailable, not connection_unavailable, when the key is switched off.
func TestModuleRegistryKnowsWithheldPlatformKeys(t *testing.T) {
	src, err := config.Build(static.New(map[string]any{"automation.openai.apikey": "sk-openai"}))
	require.NoError(t, err)
	components := []composition.Component{}
	for _, c := range modules.Components() {
		if c.Descriptor().Name != "automations" {
			components = append(components, c)
		}
	}
	components = append(components, automations.NewComponent())
	f := itf.Setup(t, itf.WithComponents(components...), itf.WithUser(itf.User()), itf.WithSource(src))
	reg, err := composition.Resolve[catalog.Registry](f.Container)
	require.NoError(t, err)
	grants, err := composition.Resolve[connectors.PlatformGrants](f.Container)
	require.NoError(t, err)
	tenant := f.Tenant.ID

	view, err := reg.ForTenant(f.Ctx, tenant)
	require.NoError(t, err)
	assert.True(t, view.PlatformWithheld[catalog.KindOpenAI], "offered by the platform, not enabled by the tenant")
	assert.NotContains(t, view.Catalog.Nodes, "ai.llm")

	require.NoError(t, grants.SetEnabled(f.Ctx, tenant, catalog.KindOpenAI, true, 0))
	view, err = reg.ForTenant(f.Ctx, tenant)
	require.NoError(t, err)
	assert.False(t, view.PlatformWithheld[catalog.KindOpenAI])
	assert.Contains(t, view.Catalog.Nodes, "ai.llm")

	require.NoError(t, grants.SetEnabled(f.Ctx, tenant, catalog.KindOpenAI, false, 0))
	view, err = reg.ForTenant(f.Ctx, tenant)
	require.NoError(t, err)
	assert.True(t, view.PlatformWithheld[catalog.KindOpenAI], "switched off again")
}

// The SDK's email connectors are in the directory and the catalog of the module without any option: the
// application's SMTP_* is the platform credential of kind smtp, the trigger's matching rule is the engine's,
// and both sign-ins (Google and Microsoft 365) are offered and start at their provider once the application
// has their OAuth2 clients.
func TestComponentRegistersTheEmailConnectors(t *testing.T) {
	src, err := config.Build(static.New(map[string]any{
		"automation.connections.keys": "k1:" + string(bytes.Repeat([]byte("ab"), 32)),
		"smtp.host":                   "smtp.corp.example", "smtp.port": 465, "smtp.username": "bot", "smtp.password": "pw", "smtp.from": "noreply@corp.example",
		"automation.platform.credentials": `{"google_oauth": {"client_id": "g-client", "client_secret": "g-secret"},
			"microsoft_oauth": {"client_id": "m-client", "client_secret": "m-secret"}}`,
	}))
	require.NoError(t, err)
	components := []composition.Component{}
	for _, c := range modules.Components() {
		if c.Descriptor().Name != "automations" {
			components = append(components, c)
		}
	}
	components = append(components, automations.NewComponent())
	f := itf.Setup(t, itf.WithComponents(components...), itf.WithUser(itf.User()), itf.WithSource(src))

	dir, err := composition.Resolve[connectors.Directory](f.Container)
	require.NoError(t, err)
	for _, kind := range []catalog.ConnectorKind{email.KindSMTP, email.KindIMAP} {
		c, ok := dir.Get(kind)
		require.True(t, ok, kind)
		assert.Equal(t, connectors.CategoryEmail, c.Descriptor().Category, kind)
		assert.ElementsMatch(t, []string{email.PresetGmail, email.PresetM365}, keysOf(c.Descriptor().OAuth2Presets), kind)
	}
	reg, err := composition.Resolve[catalog.Registry](f.Container)
	require.NoError(t, err)
	for _, node := range []string{"email.send", "email.wait_reply", "email.label"} {
		_, ok := reg.Node(node)
		assert.True(t, ok, "node %s is registered", node)
	}

	platform, err := composition.Resolve[connectors.PlatformCredentials](f.Container)
	require.NoError(t, err)
	creds, _, ok := platform.Platform(email.KindSMTP)
	require.True(t, ok, "SMTP_* is the platform credential of kind smtp")
	assert.Equal(t, "smtp.corp.example", creds["host"])

	svc, err := composition.Resolve[*services.IntegrationsService](f.Container)
	require.NoError(t, err)
	// The sign-in's state is written through the pool, outside the test's transaction: its user is committed.
	var userID int64
	require.NoError(t, f.Pool.QueryRow(f.Ctx, `INSERT INTO users (tenant_id, type, first_name, last_name, email, ui_language)
		VALUES ($1, 'user', 'Ann', 'Admin', $2, 'en') RETURNING id`, f.Tenant.ID, "admin-"+uuid.NewString()+"@example.com").Scan(&userID))
	t.Cleanup(func() {
		_, _ = f.Pool.Exec(context.Background(), `DELETE FROM automation.oauth_states WHERE user_id = $1`, userID)
		_, _ = f.Pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})
	actor := services.Actor{TenantID: f.Tenant.ID, UserID: userID}
	entries, err := svc.List(f.Ctx, actor, services.ListOptions{})
	require.NoError(t, err)
	presets := map[catalog.ConnectorKind][]string{}
	for _, e := range entries {
		presets[e.Connector.Kind()] = e.OAuthPresets
	}
	assert.Equal(t, []string{email.PresetGmail, email.PresetM365}, presets[email.KindIMAP], "the page offers both sign-ins")
	assert.Equal(t, []string{email.PresetGmail, email.PresetM365}, presets[email.KindSMTP])

	for preset, want := range map[string]struct{ host, client string }{
		email.PresetGmail: {"accounts.google.com", "g-client"},
		email.PresetM365:  {"login.microsoftonline.com", "m-client"},
	} {
		authURL, err := svc.OAuthStartPreset(f.Ctx, actor, email.KindIMAP, preset, "", "", uuid.Nil, "https://app.example/settings/integrations/oauth/callback")
		require.NoError(t, err, preset)
		u, err := url.Parse(authURL)
		require.NoError(t, err)
		assert.Equal(t, want.host, u.Host, preset)
		assert.Equal(t, want.client, u.Query().Get("client_id"), preset)
		assert.NotEmpty(t, u.Query().Get("state"), preset)
		assert.Contains(t, u.Query().Get("scope"), "openid", preset)
	}
}

func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
