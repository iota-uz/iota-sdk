package e2e_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// Scenario 24: write rules guard every write the data layer makes for a caller
// (a command, an administrator's ext_invoke, a workflow), and purge is not one:
// it drops a deactivated package's tables as one administrative teardown (the
// publish permission, audited, refused while the package is active or pinned
// by live workflow instances), so no rule runs for the rows that go with it.
// That is intended, and pinned here: a rule that refuses every delete does not
// stop a purge, and the package's data and schema state are gone afterwards.
func TestScenario24PurgeIsATeardownNotAWriteAndRunsNoWriteRule(t *testing.T) {
	e := newEnv(t)
	admin := e.admin
	const ns = "e2e.ledger"
	e.deploy(admin, ns, "1.0.0", loadTree(t, "ledger"))

	var added struct {
		Entry map[string]string `json:"entry"`
	}
	e.mustRPC(admin, "command", "e2e.ledger.append", map[string]any{"memo": "first"}, &added)
	table := e.scalarString(`SELECT schema_name || '.' || table_name FROM extruntime.entity_schemas WHERE package = $1 AND entity_id = 'e2e.ledger.entry'`, ns)
	require.Equal(t, 1, e.scalarInt(`SELECT count(*) FROM `+table))

	// The rule refuses a delete from the package's own command, whoever runs it.
	refused := e.rpc(admin, "command", "e2e.ledger.remove", map[string]any{"entry": added.Entry})
	require.False(t, refused.OK, "the write rule refuses the delete")
	require.NotNil(t, refused.Error)
	require.Equal(t, "runtime.guardRejected", refused.Error.Code)
	require.Equal(t, 1, e.scalarInt(`SELECT count(*) FROM `+table), "the entry is still there")

	// Purge needs the package deactivated, then drops the table whatever the rule says.
	e.mgmt(admin, http.MethodPost, "/packages/"+ns+"/purge", nil, http.StatusConflict, nil)
	e.mgmt(admin, http.MethodPost, "/packages/"+ns+"/deactivate", nil, http.StatusNoContent, nil)
	e.mgmt(admin, http.MethodPost, "/packages/"+ns+"/purge", nil, http.StatusNoContent, nil)
	require.Zero(t, e.scalarInt(`SELECT count(*) FROM pg_tables WHERE schemaname || '.' || tablename = $1`, table), "the table is gone")
	require.Zero(t, e.scalarInt(`SELECT count(*) FROM extruntime.entity_schemas WHERE package = $1`, ns), "and the recorded schema state")
	require.Equal(t, "purged", e.scalarString(`SELECT state FROM extruntime.activations WHERE namespace = $1`, ns))
}
