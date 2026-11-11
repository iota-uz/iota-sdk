package shellext_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/shellext"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

type nav struct{}

func (nav) Items(_ context.Context, in []types.NavigationItem) []types.NavigationItem {
	return append(in, types.NavigationItem{Key: "x"})
}
func (nav) Workspaces(_ context.Context, in []types.NavWorkspace) []types.NavWorkspace { return in }

func TestComponentRendersContributorsInOrderAndSkipsNil(t *testing.T) {
	ctx := shellext.WithMarkup(context.Background(), func(context.Context) templ.Component { return templ.Raw("<a>") })
	ctx = shellext.WithMarkup(ctx, func(context.Context) templ.Component { return nil })
	ctx = shellext.WithMarkup(ctx, func(context.Context) templ.Component { return templ.Raw("<b>") })
	var out bytes.Buffer
	require.NoError(t, shellext.Component().Render(ctx, &out))
	assert.Equal(t, "<a><b>", out.String())

	out.Reset()
	require.NoError(t, shellext.Component().Render(context.Background(), &out))
	assert.Empty(t, out.String(), "no contributors, no markup")
}

func TestContributorsDoNotLeakAcrossContexts(t *testing.T) {
	base := context.Background()
	derived := shellext.WithMarkup(base, func(context.Context) templ.Component { return templ.Raw("x") })
	var out bytes.Buffer
	require.NoError(t, shellext.Component().Render(base, &out))
	assert.Empty(t, out.String())
	_ = derived

	_, ok := shellext.NavigationFrom(base)
	assert.False(t, ok)
	with := shellext.WithNavigation(base, nav{})
	n, ok := shellext.NavigationFrom(with)
	require.True(t, ok)
	assert.Len(t, n.Items(with, nil), 1)
}
