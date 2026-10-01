package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	handler := func(ctx context.Context, params map[string]any, progress ProgressReporter) (Result, error) {
		return Result{}, nil
	}

	r.Register("export", handler)

	got, ok := r.Get("export")
	require.True(t, ok)
	assert.NotNil(t, got)
	_, ok = r.Get("missing")
	assert.False(t, ok)
}

func TestRegistry_KindsSorted(t *testing.T) {
	r := NewRegistry()
	noop := func(ctx context.Context, params map[string]any, progress ProgressReporter) (Result, error) {
		return Result{}, nil
	}
	r.Register("b", noop)
	r.Register("a", noop)

	assert.Equal(t, []string{"a", "b"}, r.Kinds())
}

func TestRegistry_DuplicateKindPanics(t *testing.T) {
	r := NewRegistry()
	noop := func(ctx context.Context, params map[string]any, progress ProgressReporter) (Result, error) {
		return Result{}, nil
	}
	r.Register("export", noop)

	require.Panics(t, func() { r.Register("export", noop) })
}

func TestRegistry_EmptyKindPanics(t *testing.T) {
	r := NewRegistry()

	require.Panics(t, func() {
		r.Register("", func(ctx context.Context, params map[string]any, progress ProgressReporter) (Result, error) {
			return Result{}, nil
		})
	})
}

func TestRegistry_NilHandlerPanics(t *testing.T) {
	r := NewRegistry()

	require.Panics(t, func() { r.Register("export", nil) })
}
