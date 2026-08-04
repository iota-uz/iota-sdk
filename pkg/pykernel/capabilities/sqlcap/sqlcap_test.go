package sqlcap_test

import (
	"context"
	"errors"
	"testing"
	"time"

	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/pykernel"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/capabilities/sqlcap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeExecutor struct {
	gotSQL     string
	gotParams  []any
	gotTimeout time.Duration
	res        *bichatsql.QueryResult
	err        error
}

func (f *fakeExecutor) ExecuteQuery(_ context.Context, sql string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error) {
	f.gotSQL = sql
	f.gotParams = params
	f.gotTimeout = timeout
	return f.res, f.err
}

func TestSQLCapability_ReturnsRowMaps(t *testing.T) {
	t.Parallel()

	fe := &fakeExecutor{res: &bichatsql.QueryResult{
		Columns:  []string{"id", "name"},
		Rows:     [][]any{{int64(1), "a"}, {int64(2), "b"}},
		RowCount: 2,
	}}
	c := sqlcap.New(fe, sqlcap.WithTimeout(5*time.Second))

	assert.Equal(t, "sql", c.Name())
	assert.Equal(t, pykernel.AccessRead, c.Access())

	out, err := c.Invoke(context.Background(), pykernel.CallArgs{
		"query":  "select id, name from t where id = $1",
		"params": []any{int64(1)},
	})
	require.NoError(t, err)

	rows, ok := out.([]map[string]any)
	require.True(t, ok)
	require.Len(t, rows, 2)
	assert.Equal(t, int64(1), rows[0]["id"])
	assert.Equal(t, "a", rows[0]["name"])

	assert.Equal(t, "select id, name from t where id = $1", fe.gotSQL)
	assert.Equal(t, []any{int64(1)}, fe.gotParams)
	assert.Equal(t, 5*time.Second, fe.gotTimeout)
}

func TestSQLCapability_AllowedInPlanMode(t *testing.T) {
	t.Parallel()

	fe := &fakeExecutor{res: &bichatsql.QueryResult{Columns: []string{"n"}, Rows: [][]any{{int64(1)}}}}
	set, err := pykernel.NewCapabilitySet(sqlcap.New(fe))
	require.NoError(t, err)

	// Read-only: a plan run must NOT refuse it.
	out, err := pykernel.Dispatch(context.Background(), set, pykernel.ModePlan, "sql",
		pykernel.CallArgs{"query": "select 1"})
	require.NoError(t, err)
	assert.Len(t, out, 1)
	assert.Nil(t, fe.gotParams, "absent params must pass nil to the executor")
}

func TestSQLCapability_Errors(t *testing.T) {
	t.Parallel()

	t.Run("missing query", func(t *testing.T) {
		t.Parallel()
		c := sqlcap.New(&fakeExecutor{res: &bichatsql.QueryResult{}})
		_, err := c.Invoke(context.Background(), pykernel.CallArgs{})
		require.Error(t, err)
	})

	t.Run("non-list params", func(t *testing.T) {
		t.Parallel()
		c := sqlcap.New(&fakeExecutor{res: &bichatsql.QueryResult{}})
		_, err := c.Invoke(context.Background(), pykernel.CallArgs{"query": "select 1", "params": "nope"})
		require.Error(t, err)
	})

	t.Run("executor error propagates", func(t *testing.T) {
		t.Parallel()
		boom := errors.New("db down")
		c := sqlcap.New(&fakeExecutor{err: boom})
		_, err := c.Invoke(context.Background(), pykernel.CallArgs{"query": "select 1"})
		require.ErrorIs(t, err, boom)
	})
}

func TestSQLCapability_WithName(t *testing.T) {
	t.Parallel()
	c := sqlcap.New(&fakeExecutor{res: &bichatsql.QueryResult{}}, sqlcap.WithName("query_db"))
	assert.Equal(t, "query_db", c.Name())
}
