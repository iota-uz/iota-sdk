package automations_test

import (
	"bytes"
	"context"
	"encoding/json"
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
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
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
	_, err = composition.Resolve[catalog.Registry](f.Container)
	require.NoError(t, err)
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
