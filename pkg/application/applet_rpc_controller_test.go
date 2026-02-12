package application

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/applets"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

type rpcTestApplet struct {
	name     string
	basePath string
	method   string
}

func (a *rpcTestApplet) Name() string     { return a.name }
func (a *rpcTestApplet) BasePath() string { return a.basePath }
func (a *rpcTestApplet) Config() applets.Config {
	return applets.Config{
		WindowGlobal: "__DEMO_CONTEXT__",
		Shell:        applets.ShellConfig{Mode: applets.ShellModeStandalone, Title: "Demo"},
		Assets: applets.AssetConfig{
			FS: fstest.MapFS{
				"manifest.json":  {Data: []byte(`{"index.html":{"file":"assets/main.js","isEntry":true}}`)},
				"assets/main.js": {Data: []byte("console.log('demo')")},
			},
			BasePath:     "/assets",
			ManifestPath: "manifest.json",
			Entrypoint:   "index.html",
		},
		RPC: &applets.RPCConfig{
			Path: "/rpc",
			Methods: map[string]applets.RPCMethod{
				a.method: {
					Handler: func(_ context.Context, _ json.RawMessage) (any, error) {
						return map[string]any{"ok": true}, nil
					},
				},
			},
		},
	}
}

type rpcTestUser struct{}

func (u *rpcTestUser) ID() uint                    { return 1 }
func (u *rpcTestUser) DisplayName() string         { return "Demo User" }
func (u *rpcTestUser) HasPermission(_ string) bool { return true }
func (u *rpcTestUser) PermissionNames() []string   { return []string{"Demo.Access"} }

type rpcTestHostServices struct{}

func (h *rpcTestHostServices) ExtractUser(context.Context) (applets.AppletUser, error) {
	return &rpcTestUser{}, nil
}

func (h *rpcTestHostServices) ExtractTenantID(context.Context) (uuid.UUID, error) {
	return uuid.MustParse("00000000-0000-0000-0000-000000000001"), nil
}

func (h *rpcTestHostServices) ExtractPool(context.Context) (*pgxpool.Pool, error) {
	return nil, fmt.Errorf("no pool in tests")
}

func (h *rpcTestHostServices) ExtractPageLocale(context.Context) language.Tag {
	return language.English
}

func TestCreateAppletControllers_GlobalRPCRouteOnly(t *testing.T) {
	t.Parallel()

	app := New(&ApplicationOptions{Bundle: LoadBundle(), SupportedLanguages: []string{"en"}})
	require.NoError(t, app.RegisterApplet(&rpcTestApplet{name: "demo", basePath: "/demo", method: "demo.ping"}))

	controllers, err := app.CreateAppletControllers(
		&rpcTestHostServices{},
		applets.DefaultSessionConfig,
		logrus.New(),
		nil,
	)
	require.NoError(t, err)

	hasAppletRPC := false
	for _, c := range controllers {
		if c.Key() == "applet_rpc" {
			hasAppletRPC = true
			break
		}
	}
	assert.True(t, hasAppletRPC, "global /rpc controller should be auto-registered")

	r := mux.NewRouter()
	for _, c := range controllers {
		c.Register(r)
	}

	globalReq := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewBufferString(`{"id":"1","method":"demo.ping","params":{}}`))
	globalRes := httptest.NewRecorder()
	r.ServeHTTP(globalRes, globalReq)
	require.Equal(t, http.StatusOK, globalRes.Code)
	assert.Contains(t, globalRes.Body.String(), `"jsonrpc":"2.0"`)
	assert.Contains(t, globalRes.Body.String(), `"ok":true`)

	perAppletReq := httptest.NewRequest(http.MethodPost, "/demo/rpc", bytes.NewBufferString(`{"id":"1","method":"demo.ping","params":{}}`))
	perAppletRes := httptest.NewRecorder()
	r.ServeHTTP(perAppletRes, perAppletReq)
	assert.Equal(t, http.StatusMethodNotAllowed, perAppletRes.Code)
}

func TestCreateAppletControllers_GlobalRPCServesBiChatNamespacedMethod(t *testing.T) {
	t.Parallel()

	app := New(&ApplicationOptions{Bundle: LoadBundle(), SupportedLanguages: []string{"en"}})
	require.NoError(t, app.RegisterApplet(&rpcTestApplet{name: "bichat", basePath: "/bi-chat", method: "bichat.ping"}))

	controllers, err := app.CreateAppletControllers(
		&rpcTestHostServices{},
		applets.DefaultSessionConfig,
		logrus.New(),
		nil,
	)
	require.NoError(t, err)

	r := mux.NewRouter()
	for _, c := range controllers {
		c.Register(r)
	}

	req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewBufferString(`{"id":"1","method":"bichat.ping","params":{}}`))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	require.Equal(t, http.StatusOK, res.Code)
	assert.Contains(t, res.Body.String(), `"jsonrpc":"2.0"`)
	assert.Contains(t, res.Body.String(), `"ok":true`)
}

func TestCreateAppletControllers_BiChatRedisKVRequiresURL(t *testing.T) {
	t.Setenv("IOTA_APPLET_ENGINE_BICHAT_KV_BACKEND", "redis")
	t.Setenv("IOTA_APPLET_ENGINE_REDIS_URL", "")

	app := New(&ApplicationOptions{Bundle: LoadBundle(), SupportedLanguages: []string{"en"}})
	require.NoError(t, app.RegisterApplet(&rpcTestApplet{name: "bichat", basePath: "/bi-chat", method: "bichat.ping"}))

	_, err := app.CreateAppletControllers(
		&rpcTestHostServices{},
		applets.DefaultSessionConfig,
		logrus.New(),
		nil,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "configure redis kv store for bichat")
}

func TestCreateAppletControllers_BiChatPostgresDBRequiresPool(t *testing.T) {
	t.Setenv("IOTA_APPLET_ENGINE_BICHAT_DB_BACKEND", "postgres")

	app := New(&ApplicationOptions{Bundle: LoadBundle(), SupportedLanguages: []string{"en"}})
	require.NoError(t, app.RegisterApplet(&rpcTestApplet{name: "bichat", basePath: "/bi-chat", method: "bichat.ping"}))

	_, err := app.CreateAppletControllers(
		&rpcTestHostServices{},
		applets.DefaultSessionConfig,
		logrus.New(),
		nil,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "configure postgres db store for bichat")
}

func TestCreateAppletControllers_BiChatPostgresJobsRequiresPool(t *testing.T) {
	t.Setenv("IOTA_APPLET_ENGINE_BICHAT_JOBS_BACKEND", "postgres")

	app := New(&ApplicationOptions{Bundle: LoadBundle(), SupportedLanguages: []string{"en"}})
	require.NoError(t, app.RegisterApplet(&rpcTestApplet{name: "bichat", basePath: "/bi-chat", method: "bichat.ping"}))

	_, err := app.CreateAppletControllers(
		&rpcTestHostServices{},
		applets.DefaultSessionConfig,
		logrus.New(),
		nil,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "configure postgres jobs store for bichat")
}
