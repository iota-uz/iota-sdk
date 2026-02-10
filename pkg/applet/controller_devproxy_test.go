package applet_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/applets"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

// testApplet implements applets.Applet for tests.
type testApplet struct {
	name     string
	basePath string
	config   applets.Config
}

func (a *testApplet) Name() string           { return a.name }
func (a *testApplet) BasePath() string       { return a.basePath }
func (a *testApplet) Config() applets.Config { return a.config }

// appletUserAdapter adapts iota-sdk user.User to applets.AppletUser.
type appletUserAdapter struct {
	u user.User
}

func (a *appletUserAdapter) ID() uint { return a.u.ID() }

func (a *appletUserAdapter) DisplayName() string {
	return strings.TrimSpace(a.u.FirstName() + " " + a.u.LastName())
}

func (a *appletUserAdapter) HasPermission(name string) bool {
	for _, p := range a.u.Permissions() {
		if p.Name() == name {
			return true
		}
	}
	return false
}

func (a *appletUserAdapter) PermissionNames() []string {
	names := make([]string, 0, len(a.u.Permissions()))
	for _, p := range a.u.Permissions() {
		names = append(names, p.Name())
	}
	return names
}

// testHostServices implements applets.HostServices using iota-sdk composables.
type testHostServices struct{}

func (h *testHostServices) ExtractUser(ctx context.Context) (applets.AppletUser, error) {
	u, err := composables.UseUser(ctx)
	if err != nil || u == nil {
		return nil, fmt.Errorf("no user in context")
	}
	return &appletUserAdapter{u: u}, nil
}

func (h *testHostServices) ExtractTenantID(ctx context.Context) (uuid.UUID, error) {
	return composables.UseTenantID(ctx)
}

func (h *testHostServices) ExtractPool(ctx context.Context) (*pgxpool.Pool, error) {
	// No pool in tests; use sentinel to satisfy nilnil (do not return nil, nil for pointer type).
	return nil, errNoPool
}

var errNoPool = fmt.Errorf("no pool in test")

func (h *testHostServices) ExtractPageLocale(ctx context.Context) language.Tag {
	return composables.UsePageCtx(ctx).GetLocale()
}

func TestRegisterDevProxy_502WhenTargetDown(t *testing.T) {
	t.Parallel()

	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())

	targetURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	a := &testApplet{
		name:     "chat",
		basePath: "/bi-chat",
		config: applets.Config{
			WindowGlobal: "__T__",
			Shell:        applets.ShellConfig{Mode: applets.ShellModeStandalone, Title: "t"},
			Assets: applets.AssetConfig{
				BasePath: "/assets",
				Dev: &applets.DevAssetConfig{
					Enabled:   true,
					TargetURL: targetURL,
				},
			},
		},
	}
	c, err := applets.NewAppletController(a, nil, applets.DefaultSessionConfig, nil, nil, &testHostServices{})
	require.NoError(t, err)
	router := mux.NewRouter()
	c.RegisterRoutes(router)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/bi-chat/assets/@vite/client", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadGateway, rec.Code, "proxy should return 502 when target is down")
}

func TestDevProxy_BlackBox_AssetRoutes(t *testing.T) {
	t.Parallel()

	viteBody := []byte("vite client js")
	mainBody := []byte("main tsx")
	// The proxy always forwards the full path — Vite's base is set to the prefix.
	vite := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bi-chat/assets/@vite/client":
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = w.Write(viteBody)
		case "/bi-chat/assets/src/main.tsx":
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = w.Write(mainBody)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer vite.Close()

	a := &testApplet{
		name:     "chat",
		basePath: "/bi-chat",
		config: applets.Config{
			WindowGlobal: "__T__",
			Shell:        applets.ShellConfig{Mode: applets.ShellModeStandalone, Title: "t"},
			Assets: applets.AssetConfig{
				BasePath: "/assets",
				Dev: &applets.DevAssetConfig{
					Enabled:   true,
					TargetURL: vite.URL,
				},
			},
		},
	}
	c, err := applets.NewAppletController(a, nil, applets.DefaultSessionConfig, nil, nil, &testHostServices{})
	require.NoError(t, err)
	router := mux.NewRouter()
	c.RegisterRoutes(router)

	t.Run("vite_client", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/bi-chat/assets/@vite/client", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, viteBody, rec.Body.Bytes())
	})

	t.Run("main_tsx", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/bi-chat/assets/src/main.tsx", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, mainBody, rec.Body.Bytes())
	})
}

func TestDevProxy_HTMLShell(t *testing.T) {
	t.Parallel()

	basePath := "/bi-chat"
	assetsBasePath := basePath + "/assets"
	entryModule := "/src/main.tsx"

	vite := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer vite.Close()

	bundle := i18n.NewBundle(language.Russian)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	a := &testApplet{
		name:     "chat",
		basePath: basePath,
		config: applets.Config{
			WindowGlobal: "__T__",
			Shell:        applets.ShellConfig{Mode: applets.ShellModeStandalone, Title: "Test"},
			Assets: applets.AssetConfig{
				BasePath: "/assets",
				Dev: &applets.DevAssetConfig{
					Enabled:     true,
					TargetURL:   vite.URL,
					EntryModule: entryModule,
				},
			},
		},
	}
	c, err := applets.NewAppletController(a, bundle, applets.DefaultSessionConfig, nil, nil, &testHostServices{})
	require.NoError(t, err)
	router := mux.NewRouter()
	c.RegisterRoutes(router)

	u := user.New("Test", "User", internet.MustParseEmail("test@example.com"), user.UILanguageEN, user.WithID(1))
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	ctx := context.Background()
	ctx = composables.WithUser(ctx, u)
	ctx = composables.WithTenantID(ctx, tenantID)
	ctx = composables.WithPageCtx(ctx, &types.PageContext{Locale: language.English}) //nolint:staticcheck // SA1019: backward compat in test

	req := httptest.NewRequest(http.MethodGet, basePath, nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "GET %s should return 200", basePath)
	body := rec.Body.String()
	require.Contains(t, body, assetsBasePath+"/@vite/client", "HTML should contain script src for Vite client")
	require.Contains(t, body, assetsBasePath+"/src/main.tsx", "HTML should contain script src for entry module")
	require.True(t, strings.Contains(body, "<script") && strings.Contains(body, "src="), "HTML should contain at least one script tag with src")
}

func TestRegisterDevProxy_502WhenUpstreamReturns502(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer upstream.Close()

	a := &testApplet{
		name:     "chat",
		basePath: "/bi-chat",
		config: applets.Config{
			WindowGlobal: "__T__",
			Shell:        applets.ShellConfig{Mode: applets.ShellModeStandalone, Title: "t"},
			Assets: applets.AssetConfig{
				BasePath: "/assets",
				Dev: &applets.DevAssetConfig{
					Enabled:   true,
					TargetURL: upstream.URL,
				},
			},
		},
	}
	c, err := applets.NewAppletController(a, nil, applets.DefaultSessionConfig, nil, nil, &testHostServices{})
	require.NoError(t, err)
	router := mux.NewRouter()
	c.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/bi-chat/assets/@vite/client", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadGateway, rec.Code, "proxy should pass through upstream 502")
}

