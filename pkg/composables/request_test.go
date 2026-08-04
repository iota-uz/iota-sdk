package composables

import (
	"context"
	"net/url"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

// TestTryUsePageCtx guards the non-panicking page-context accessor used by the
// Base layout so error pages (403/404) can render on code paths where the
// WithPageContext middleware has not run.
func TestTryUsePageCtx(t *testing.T) {
	t.Parallel()

	t.Run("absent returns false without panicking", func(t *testing.T) {
		t.Parallel()
		pageCtx, ok := TryUsePageCtx(context.Background())
		require.False(t, ok)
		require.Nil(t, pageCtx)
	})

	t.Run("present returns the stored page context", func(t *testing.T) {
		t.Parallel()
		want := types.NewPageContext(language.English, &url.URL{Path: "/"}, nil)
		ctx := WithPageCtx(context.Background(), want)
		got, ok := TryUsePageCtx(ctx)
		require.True(t, ok)
		require.Equal(t, want, got)
	})
}
