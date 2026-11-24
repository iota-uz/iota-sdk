package automations_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/automations"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/sms"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/sms/smstest"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// The module's engine records the spending of calls by default: an SMS the built-in Twilio node sends
// (against a fake) lands in automation.connector_usage with the usage recorder the engine is given.
func TestDefaultEngineRecordsSMSUsage(t *testing.T) {
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "automations_usage",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	db := h.Scope(t).Pool
	ctx := context.Background()

	usage := automations.DefaultEngineUsage(db)
	require.NotNil(t, usage, "the engine's default configuration has a usage recorder")

	fake := smstest.NewTwilio("ACtestaccountxxxxxxxxxxxxxxxxxxxx", "tokenTokenToken0123456789")
	t.Cleanup(fake.Close)
	node := sms.NewTwilio(sms.WithTransport(fake.Transport())).Nodes()[0].(catalog.CallNode)
	tenant, run := uuid.New(), uuid.New()
	_, err := db.Exec(ctx, `INSERT INTO tenants (id, name, domain, created_at, updated_at) VALUES ($1, $2, $3, NOW(), NOW())`,
		tenant, "usage-"+tenant.String(), tenant.String()+".example.com")
	require.NoError(t, err)
	res, err := node.Execute(ctx, catalog.CallInput{
		TenantID: tenant, RunID: run, NodeID: "send", Attempt: 1,
		Params:      map[string]any{"to": "+998901234567", "text": "Hello"},
		Connection:  &catalog.ConnectionInfo{ID: "platform:twilio", Kind: sms.KindTwilio, Platform: true},
		Credentials: catalog.Credentials{"account_sid": "ACtestaccountxxxxxxxxxxxxxxxxxxxx", "auth_token": "tokenTokenToken0123456789", "from": "+15005550006"},
		Usage:       usage, Deadline: time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	assert.Equal(t, "sent", res.Outcome)

	var kind, conn, unit string
	var qty int64
	require.NoError(t, db.QueryRow(ctx, `SELECT kind, connection_id, unit, quantity FROM automation.connector_usage WHERE tenant_id = $1 AND run_id = $2`, tenant, run).
		Scan(&kind, &conn, &unit, &qty))
	assert.Equal(t, "twilio", kind)
	assert.Equal(t, "platform:twilio", conn)
	assert.Equal(t, "sms", unit)
	assert.EqualValues(t, 2, qty)
}
