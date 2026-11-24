package automations

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/email"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/smtpconfig"
)

// The application's SMTP_* is the platform credential platform:smtp, offered to every tenant.
func TestSMTPConfigIsThePlatformCredentialOfKindSMTP(t *testing.T) {
	dir := connectors.NewDirectory(nil)
	for _, c := range email.New(email.Config{Platform: &smtpconfig.Config{Host: "smtp.corp", Port: 465, Username: "bot", Password: "pw", From: "noreply@corp.example"}}) {
		require.NoError(t, dir.Register(c))
	}
	platform := connectors.NewPlatformCredentials(dir, withConnectorCredentials(nil, dir.List()))

	creds, id, ok := platform.Platform(email.KindSMTP)
	require.True(t, ok)
	assert.Equal(t, "smtp.corp", creds["host"])
	assert.Equal(t, "465", creds["port"])
	assert.Equal(t, "noreply@corp.example", id.Display)
	_, _, ok = platform.Platform(email.KindIMAP)
	assert.False(t, ok, "the application's SMTP only sends")
	cat, ok := platform.(connectors.PlatformCatalog)
	require.True(t, ok)
	assert.Equal(t, []catalog.ConnectorKind{email.KindSMTP}, cat.Kinds())
	assert.Equal(t, connectors.PlatformAuto, cat.Policy(email.KindSMTP))

	// A credential the automation configuration names wins over the one the connector brings.
	named := map[catalog.ConnectorKind]connectors.PlatformCredential{email.KindSMTP: {Credentials: connectors.Credentials{"host": "other.example", "port": "25", "from_address": "a@other.example"}}}
	platform = connectors.NewPlatformCredentials(dir, withConnectorCredentials(named, dir.List()))
	creds, _, _ = platform.Platform(email.KindSMTP)
	assert.Equal(t, "other.example", creds["host"])

	// Without SMTP_HOST there is none.
	empty := connectors.NewDirectory(nil)
	for _, c := range email.New(email.Config{}) {
		require.NoError(t, empty.Register(c))
	}
	_, _, ok = connectors.NewPlatformCredentials(empty, withConnectorCredentials(nil, empty.List())).Platform(email.KindSMTP)
	assert.False(t, ok)
}
