package drillreturn_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/lens/drillreturn"
)

type fixture struct {
	Param     string `json:"param"`
	HostParam string `json:"hostParam"`
	Contexts  []struct {
		Context drillreturn.Context `json:"context"`
		Token   string              `json:"token"`
	} `json:"contexts"`
	HostStates []struct {
		State drillreturn.HostState `json:"state"`
		Token string                `json:"token"`
	} `json:"hostStates"`
	Invalid []string `json:"invalid"`
}

func loadFixture(t *testing.T) fixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "web", "lens", "fixtures", "return-context.json"))
	require.NoError(t, err)
	var value fixture
	require.NoError(t, json.Unmarshal(raw, &value))
	return value
}

// The same fixture is read by web/lens/src/runtime/returnContext.test.ts:
// both sides must produce and accept identical tokens.
func TestSharedFixture(t *testing.T) {
	t.Parallel()
	value := loadFixture(t)
	require.Equal(t, drillreturn.Param, value.Param)
	require.Equal(t, drillreturn.HostParam, value.HostParam)
	require.NotEmpty(t, value.Contexts)
	for _, item := range value.Contexts {
		token, err := drillreturn.Encode(item.Context)
		require.NoError(t, err)
		require.Equal(t, item.Token, token)
		decoded, err := drillreturn.Decode(item.Token)
		require.NoError(t, err)
		require.Equal(t, item.Context, decoded)
	}
	for _, item := range value.HostStates {
		token, err := drillreturn.EncodeHostState(item.State)
		require.NoError(t, err)
		require.Equal(t, item.Token, token)
		decoded, err := drillreturn.DecodeHostState(item.Token)
		require.NoError(t, err)
		require.Equal(t, item.State, decoded)
	}
	for _, token := range value.Invalid {
		_, err := drillreturn.Decode(token)
		require.ErrorIs(t, err, drillreturn.ErrInvalid, token)
	}
}

func TestFromRequestAndBackURL(t *testing.T) {
	t.Parallel()
	context := drillreturn.Context{Href: "/sales?path=root&path=root%2Fnorth&period=2026-06", Title: "Sales", Trail: []string{"Region", "North"}}
	token, err := drillreturn.Encode(context)
	require.NoError(t, err)

	request := httptest.NewRequest(http.MethodGet, "/records?branch=north&"+drillreturn.Param+"="+token, nil)
	received, ok := drillreturn.FromRequest(request)
	require.True(t, ok)
	require.Equal(t, []string{"Region", "North"}, received.Trail)

	back, err := received.BackURL("/records", `{"page":2}`)
	require.NoError(t, err)
	parsed, err := url.Parse(back)
	require.NoError(t, err)
	require.Equal(t, "/sales", parsed.Path)
	require.Equal(t, []string{"root", "root/north"}, parsed.Query()["path"], "levels keep their order")
	require.Equal(t, "2026-06", parsed.Query().Get("period"), "filters survive")
	state, err := drillreturn.DecodeHostState(parsed.Query().Get(drillreturn.HostParam))
	require.NoError(t, err)
	require.Equal(t, drillreturn.HostState{V: 1, Leaf: "/records", State: `{"page":2}`}, state)

	plain, err := received.BackURL("/records", "")
	require.NoError(t, err)
	require.NotContains(t, plain, drillreturn.HostParam)

	_, ok = drillreturn.FromRequest(httptest.NewRequest(http.MethodGet, "/records", nil))
	require.False(t, ok, "a page not opened from Lens has no return context")
}

func TestRejectsForeignDestinations(t *testing.T) {
	t.Parallel()
	for _, href := range []string{"https://evil.example/", "//evil.example/", "javascript:alert(1)", "relative/path", "/\\evil"} {
		_, err := drillreturn.Encode(drillreturn.Context{Href: href, Title: "T"})
		require.ErrorIs(t, err, drillreturn.ErrInvalid, href)
	}
	_, err := drillreturn.EncodeHostState(drillreturn.HostState{Leaf: "https://evil.example/", State: "x"})
	require.ErrorIs(t, err, drillreturn.ErrInvalid)
}
