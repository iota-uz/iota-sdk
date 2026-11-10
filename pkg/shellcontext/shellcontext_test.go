package shellcontext_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/shellcontext"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

func TestFlatten_KeysParentsAndWorkspaces(t *testing.T) {
	items := []types.NavigationItem{
		{Key: "crm", Name: "CRM", Workspace: "sales", Children: []types.NavigationItem{
			{Name: "Clients", Href: "/clients"},
			{Name: "Deals", Href: "/deals", Workspace: "ops", IsBeta: true},
		}},
		{Name: "Settings"},
		{Name: "Settings"},
	}
	got := shellcontext.Flatten(items)
	require.Len(t, got, 5)
	assert.Equal(t, shellcontext.NavigationItem{Key: "crm", Title: "CRM", Workspace: "sales", Order: 0}, got[0])
	assert.Equal(t, shellcontext.NavigationItem{Key: "/clients", Title: "Clients", Href: "/clients", Parent: "crm", Workspace: "sales", Order: 0}, got[1])
	assert.Equal(t, "ops", got[2].Workspace)
	assert.True(t, got[2].Beta)
	assert.Equal(t, "item:1", got[3].Key)
	assert.Equal(t, "item:2", got[4].Key)
}

func TestFlatten_DuplicateKeysStayUnique(t *testing.T) {
	got := shellcontext.Flatten([]types.NavigationItem{{Key: "a", Name: "A"}, {Key: "a", Name: "B"}})
	require.Len(t, got, 2)
	assert.NotEqual(t, got[0].Key, got[1].Key)
}

func TestComponent_RendersEscapedJSON(t *testing.T) {
	ctx := context.WithValue(context.Background(), constants.NavItemsKey, []types.NavigationItem{{Key: "x", Name: "</script><b>"}})
	var out bytes.Buffer
	require.NoError(t, shellcontext.Component().Render(ctx, &out))
	html := out.String()
	assert.True(t, strings.HasPrefix(html, `<script id="iota-shell-context" type="application/json">`))
	assert.Equal(t, 1, strings.Count(html, "</script>"))
	payload := strings.TrimSuffix(strings.TrimPrefix(html, `<script id="iota-shell-context" type="application/json">`), "</script>")
	var decoded shellcontext.Context
	require.NoError(t, json.Unmarshal([]byte(payload), &decoded))
	assert.Equal(t, "</script><b>", decoded.Navigation[0].Title)
	assert.Equal(t, shellcontext.Version, decoded.Version)
}

func TestComponent_WithoutNavigation(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, shellcontext.Component().Render(context.Background(), &out))
	assert.Contains(t, out.String(), `"navigation":[]`)
}
