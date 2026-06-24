package pykernel_test

import (
	"context"
	"errors"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/pykernel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingCap counts Invoke calls so tests can assert a refused capability is
// never actually executed.
type recordingCap struct {
	name    string
	access  pykernel.Access
	calls   int
	lastCtx context.Context
	result  any
	err     error
}

func (c *recordingCap) Name() string { return c.name }
func (c *recordingCap) Signature() pykernel.CapabilitySignature {
	return pykernel.CapabilitySignature{}
}
func (c *recordingCap) Access() pykernel.Access { return c.access }
func (c *recordingCap) Invoke(ctx context.Context, _ pykernel.CallArgs) (any, error) {
	c.calls++
	c.lastCtx = ctx
	return c.result, c.err
}

func TestNewCapabilitySet(t *testing.T) {
	t.Parallel()

	read := &recordingCap{name: "pg_query", access: pykernel.AccessRead}
	write := &recordingCap{name: "pg_upsert", access: pykernel.AccessWrite}

	set, err := pykernel.NewCapabilitySet(read, write)
	require.NoError(t, err)

	// Lookup resolves registered names and rejects unknown ones.
	got, ok := set.Lookup("pg_query")
	require.True(t, ok)
	assert.Same(t, read, got)
	_, ok = set.Lookup("missing")
	assert.False(t, ok)

	// List preserves registration order and is a defensive copy.
	list := set.List()
	require.Len(t, list, 2)
	assert.Equal(t, "pg_query", list[0].Name())
	assert.Equal(t, "pg_upsert", list[1].Name())
	list[0] = nil
	assert.NotNil(t, set.List()[0], "List must return a copy")
}

func TestNewCapabilitySet_Rejections(t *testing.T) {
	t.Parallel()

	t.Run("duplicate name", func(t *testing.T) {
		t.Parallel()
		a := &recordingCap{name: "sql", access: pykernel.AccessRead}
		b := &recordingCap{name: "sql", access: pykernel.AccessRead}
		_, err := pykernel.NewCapabilitySet(a, b)
		require.ErrorIs(t, err, pykernel.ErrDuplicateCapability)
	})

	t.Run("empty name", func(t *testing.T) {
		t.Parallel()
		_, err := pykernel.NewCapabilitySet(&recordingCap{name: "", access: pykernel.AccessRead})
		require.Error(t, err)
	})

	t.Run("nil capability", func(t *testing.T) {
		t.Parallel()
		_, err := pykernel.NewCapabilitySet(nil)
		require.Error(t, err)
	})
}

func TestAuthorize(t *testing.T) {
	t.Parallel()

	read := &recordingCap{name: "r", access: pykernel.AccessRead}
	write := &recordingCap{name: "w", access: pykernel.AccessWrite}

	// Reads are allowed in both modes.
	assert.NoError(t, pykernel.Authorize(pykernel.ModePlan, read))
	assert.NoError(t, pykernel.Authorize(pykernel.ModeApply, read))

	// Writes are allowed only in apply mode; refused in plan mode.
	assert.NoError(t, pykernel.Authorize(pykernel.ModeApply, write))
	err := pykernel.Authorize(pykernel.ModePlan, write)
	require.ErrorIs(t, err, pykernel.ErrPlanModeWrite)
	assert.Contains(t, err.Error(), "w", "refusal should name the capability")
}

func TestDispatch(t *testing.T) {
	t.Parallel()

	t.Run("unknown capability", func(t *testing.T) {
		t.Parallel()
		set, err := pykernel.NewCapabilitySet()
		require.NoError(t, err)
		_, err = pykernel.Dispatch(context.Background(), set, pykernel.ModeApply, "nope", nil)
		require.ErrorIs(t, err, pykernel.ErrCapabilityNotFound)
	})

	t.Run("write refused in plan mode is never invoked", func(t *testing.T) {
		t.Parallel()
		write := &recordingCap{name: "pg_upsert", access: pykernel.AccessWrite, result: "ok"}
		set, err := pykernel.NewCapabilitySet(write)
		require.NoError(t, err)

		_, err = pykernel.Dispatch(context.Background(), set, pykernel.ModePlan, "pg_upsert", nil)
		require.ErrorIs(t, err, pykernel.ErrPlanModeWrite)
		assert.Zero(t, write.calls, "refused write must not reach the host handler")
	})

	t.Run("write allowed in apply mode", func(t *testing.T) {
		t.Parallel()
		write := &recordingCap{name: "pg_upsert", access: pykernel.AccessWrite, result: int64(3)}
		set, err := pykernel.NewCapabilitySet(write)
		require.NoError(t, err)

		out, err := pykernel.Dispatch(context.Background(), set, pykernel.ModeApply, "pg_upsert", nil)
		require.NoError(t, err)
		assert.Equal(t, int64(3), out)
		assert.Equal(t, 1, write.calls)
	})

	t.Run("read allowed in plan mode", func(t *testing.T) {
		t.Parallel()
		read := &recordingCap{name: "pg_query", access: pykernel.AccessRead, result: []any{1, 2}}
		set, err := pykernel.NewCapabilitySet(read)
		require.NoError(t, err)

		out, err := pykernel.Dispatch(context.Background(), set, pykernel.ModePlan, "pg_query", nil)
		require.NoError(t, err)
		assert.Equal(t, []any{1, 2}, out)
		assert.Equal(t, 1, read.calls)
	})

	t.Run("handler error propagates", func(t *testing.T) {
		t.Parallel()
		boom := errors.New("boom")
		read := &recordingCap{name: "pg_query", access: pykernel.AccessRead, err: boom}
		set, err := pykernel.NewCapabilitySet(read)
		require.NoError(t, err)

		_, err = pykernel.Dispatch(context.Background(), set, pykernel.ModeApply, "pg_query", nil)
		require.ErrorIs(t, err, boom)
	})
}

func TestCapabilityFunc(t *testing.T) {
	t.Parallel()

	sig := pykernel.CapabilitySignature{
		Params:  []pykernel.ParamSpec{{Name: "query", Type: "str", Required: true}},
		Returns: "list[dict]",
		Doc:     "run read-only SQL",
	}
	var gotArgs pykernel.CallArgs
	c := pykernel.CapabilityFunc("sql", pykernel.AccessRead, sig,
		func(_ context.Context, args pykernel.CallArgs) (any, error) {
			gotArgs = args
			return "rows", nil
		})

	assert.Equal(t, "sql", c.Name())
	assert.Equal(t, pykernel.AccessRead, c.Access())
	assert.Equal(t, sig, c.Signature())

	out, err := c.Invoke(context.Background(), pykernel.CallArgs{"query": "select 1"})
	require.NoError(t, err)
	assert.Equal(t, "rows", out)
	assert.Equal(t, "select 1", gotArgs["query"])

	// A capability with no handler fails rather than panicking.
	_, err = pykernel.CapabilityFunc("noop", pykernel.AccessRead, sig, nil).
		Invoke(context.Background(), nil)
	require.Error(t, err)
}

func TestModeAndAccessStrings(t *testing.T) {
	t.Parallel()

	assert.True(t, pykernel.ModeApply.IsWritable())
	assert.False(t, pykernel.ModePlan.IsWritable())
	assert.Equal(t, "plan", pykernel.ModePlan.String())
	assert.Equal(t, "apply", pykernel.ModeApply.String())
	assert.Equal(t, "read", pykernel.AccessRead.String())
	assert.Equal(t, "write", pykernel.AccessWrite.String())
}
