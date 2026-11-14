package e2e_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// Scenario 4: an ordinary command calls an external HTTP service outside any
// transaction, then opens ctx.transaction and answers synchronously — no
// workflow instance is involved. An atomic command is one transaction that
// rolls back fully on error.
func TestScenario04OrdinaryCommandExternalCallAndAtomicRollback(t *testing.T) {
	e := newEnv(t)
	admin := e.admin
	e.deploy(admin, "e2e.quotes", "1.0.0", loadTree(t, "quotes"))

	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if strings.HasSuffix(r.URL.Path, "/FAIL") {
			http.Error(w, "boom", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"premium":"123.45"}`))
	}))
	defer upstream.Close()
	count := func() int {
		var out struct {
			Count int `json:"count"`
		}
		e.mustRPC(admin, "query", "e2e.quotes.countPolicies", map[string]any{}, &out)
		return out.Count
	}

	// The external answer is stored and returned in the same call.
	var q struct {
		Number  string `json:"number"`
		Premium string `json:"premium"`
	}
	e.mustRPC(admin, "command", "e2e.quotes.quote", map[string]any{"number": "P-1", "upstream": upstream.URL}, &q)
	require.Equal(t, "P-1", q.Number)
	require.Equal(t, "123.45", q.Premium)
	require.EqualValues(t, 1, hits.Load())
	require.Equal(t, 1, count())
	require.Zero(t, e.scalarInt(`SELECT count(*) FROM extruntime.workflow_instances`), "an ordinary command starts no workflow")
	require.Equal(t, 1, e.scalarInt(`SELECT count(*) FROM extruntime.invocations WHERE contract = 'e2e.quotes.quote' AND outcome = 'ok' AND committed_tx_count = 1 AND NOT partially_committed`))

	// A failing upstream fails the command before any transaction opens.
	res := e.rpc(admin, "command", "e2e.quotes.quote", map[string]any{"number": "FAIL", "upstream": upstream.URL})
	require.False(t, res.OK)
	require.Equal(t, http.StatusInternalServerError, res.Status)
	require.Equal(t, "internal", res.Error.Kind)
	require.Equal(t, 1, count(), "nothing was written")

	// The first transaction of an ordinary command stays committed when a later
	// step fails: the invocation records the partial commit.
	res = e.rpc(admin, "command", "e2e.quotes.twoStep", map[string]any{"number": "FAIL", "upstream": upstream.URL})
	require.False(t, res.OK)
	require.Equal(t, 2, count(), "the committed step is not undone; the command is not retried")
	require.Equal(t, 1, e.scalarInt(`SELECT count(*) FROM extruntime.invocations WHERE contract = 'e2e.quotes.twoStep' AND outcome <> 'ok' AND committed_tx_count = 1 AND partially_committed`))

	// Atomic command: all rows or none.
	var bulk struct {
		Created int `json:"created"`
	}
	e.mustRPC(admin, "atomic", "e2e.quotes.bulkCreate", map[string]any{"numbers": []string{"A", "B", "C"}}, &bulk)
	require.Equal(t, 3, bulk.Created)
	require.Equal(t, 5, count())
	res = e.rpc(admin, "atomic", "e2e.quotes.bulkCreate", map[string]any{"numbers": []string{"D", "E", "BAD-1", "F"}})
	require.False(t, res.OK)
	require.Equal(t, http.StatusUnprocessableEntity, res.Status)
	require.Equal(t, "domain", res.Error.Kind)
	require.Equal(t, "e2e.quotes.badNumber", res.Error.Code)
	require.JSONEq(t, `{"number":"BAD-1"}`, string(res.Error.Params))
	require.Equal(t, 5, count(), "D and E were rolled back with the failing call")
	require.Zero(t, e.scalarInt(`SELECT count(*) FROM x_e2e_quotes.policy WHERE number IN ('D','E','F')`))
	// An atomic command is not an ordinary one: the kind in the call must match.
	require.Equal(t, http.StatusForbidden, e.rpc(admin, "command", "e2e.quotes.bulkCreate", map[string]any{"numbers": []string{}}).Status)
	require.Zero(t, e.scalarInt(`SELECT count(*) FROM extruntime.workflow_instances`))
}
