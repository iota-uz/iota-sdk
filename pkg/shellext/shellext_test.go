package shellext_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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

type islands struct{ seen []string }

func (i *islands) Slot(_ context.Context, id string, props json.RawMessage) templ.Component {
	i.seen = append(i.seen, id+" "+string(props))
	if id == "none" {
		return nil
	}
	return templ.Raw("<slot " + id + ">")
}

func (i *islands) Cell(_ context.Context, cell shellext.Cell, fallback templ.Component) templ.Component {
	if cell.Renderer.ID == "unserved" {
		return nil
	}
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if _, err := io.WriteString(w, "<cell>"); err != nil {
			return err
		}
		if err := fallback.Render(ctx, w); err != nil {
			return err
		}
		_, err := io.WriteString(w, "</cell>")
		return err
	})
}

func render(t *testing.T, ctx context.Context, c templ.Component) string {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, c.Render(ctx, &out))
	return out.String()
}

func TestSlotRendersNothingWithoutAnIslandsHostOrContribution(t *testing.T) {
	assert.Empty(t, render(t, context.Background(), shellext.Slot(context.Background(), "crm.slots.x", map[string]int{"clientId": 1})))

	host := &islands{}
	ctx := shellext.WithIslands(context.Background(), host)
	assert.Equal(t, "<slot crm.slots.x>", render(t, ctx, shellext.Slot(ctx, "crm.slots.x", map[string]int{"clientId": 1})))
	assert.Equal(t, []string{`crm.slots.x {"clientId":1}`}, host.seen, "props are passed as JSON")
	assert.Empty(t, render(t, ctx, shellext.Slot(ctx, "none", struct{}{})), "a slot nothing contributes to renders nothing")
}

func TestSlotReportsPropsThatCannotBeEncoded(t *testing.T) {
	ctx := shellext.WithIslands(context.Background(), &islands{})
	var out bytes.Buffer
	err := shellext.Slot(ctx, "crm.slots.x", make(chan int)).Render(ctx, &out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "crm.slots.x")
}

func TestCellIslandWrapsTheStandardCellOrLeavesItAlone(t *testing.T) {
	fallback := templ.Raw("gold")
	cell := shellext.Cell{Renderer: shellext.Module{ID: "acme.tier"}, Value: "gold"}
	assert.Equal(t, "gold", render(t, context.Background(), shellext.CellIsland(context.Background(), cell, fallback)), "no host: the standard cell")
	ctx := shellext.WithIslands(context.Background(), &islands{})
	assert.Equal(t, "<cell>gold</cell>", render(t, ctx, shellext.CellIsland(ctx, cell, fallback)))
	cell.Renderer.ID = "unserved"
	assert.Equal(t, "gold", render(t, ctx, shellext.CellIsland(ctx, cell, fallback)), "a viewer the host cannot serve gets the standard cell")
}
