package applet_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/applets/pkg/applet"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

// testApplet implements applet.Applet for tests.
type testApplet struct {
	name     string
	basePath string
	config   applet.Config
}

func (a *testApplet) Name() string          { return a.name }
func (a *testApplet) BasePath() string      { return a.basePath }
func (a *testApplet) Config() applet.Config { return a.config }

// appletUserAdapter adapts iota-sdk user.User to applet.AppletUser.
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

// testHostServices implements applet.HostServices using iota-sdk composables.
type testHostServices struct{}

func (h *testHostServices) ExtractUser(ctx context.Context) (applet.AppletUser, error) {
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
	return nil, nil
}

func (h *testHostServices) ExtractPageLocale(ctx context.Context) language.Tag {
	return composables.UsePageCtx(ctx).GetLocale()
}

func TestRegisterDevProxy_StripPrefix(t *testing.T) {
	t.Parallel()

	basePath := "/bi-chat"
	assetsPath := "/assets"
	fullAssetsPath := basePath + assetsPath

	var receivedPath string
	var mu sync.Mutex
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		receivedPath = r.URL.Path
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	for _, strip := range []*bool{nil, ptr(true), ptr(false)} {
		name := "StripPrefix_default_true"
		if strip != nil && !*strip {
			name = "StripPrefix_false"
		} else if strip != nil && *strip {
			name = "StripPrefix_true"
		}
		t.Run(name, func(t *testing.T) {
			mu.Lock()
			receivedPath = ""
			mu.Unlock()

			a := &testApplet{
				name:     "chat",
				basePath: basePath,
				config: applet.Config{
					WindowGlobal: "__T__",
					Shell:        applet.ShellConfig{Mode: applet.ShellModeStandalone, Title: "t"},
					Assets: applet.AssetConfig{
						BasePath: assetsPath,
						Dev: &applet.DevAssetConfig{
							Enabled:     true,
							TargetURL:   backend.URL,
							StripPrefix: strip,
						},
					},
				},
			}
			c, err := applet.NewAppletController(a, nil, applet.DefaultSessionConfig, nil, nil, &testHostServices{})
			require.NoError(t, err)
			router := mux.NewRouter()
			c.RegisterRoutes(router)

			req := httptest.NewRequest(http.MethodGet, fullAssetsPath+"/@vite/client", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			mu.Lock()
			got := receivedPath
			mu.Unlock()

			expectStrip := strip == nil || (strip != nil && *strip)
			if expectStrip {
				require.Equal(t, "/@vite/client", got, "with StripPrefix true, backend should receive path without assets prefix")
			} else {
				require.Equal(t, fullAssetsPath+"/@vite/client", got, "with StripPrefix false, backend should receive full path")
			}
		})
	}
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
		config: applet.Config{
			WindowGlobal: "__T__",
			Shell:        applet.ShellConfig{Mode: applet.ShellModeStandalone, Title: "t"},
			Assets: applet.AssetConfig{
				BasePath: "/assets",
				Dev: &applet.DevAssetConfig{
					Enabled:   true,
					TargetURL: targetURL,
				},
			},
		},
	}
	c, err := applet.NewAppletController(a, nil, applet.DefaultSessionConfig, nil, nil, &testHostServices{})
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
	vite := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/@vite/client":
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = w.Write(viteBody)
		case "/src/main.tsx":
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
		config: applet.Config{
			WindowGlobal: "__T__",
			Shell:        applet.ShellConfig{Mode: applet.ShellModeStandalone, Title: "t"},
			Assets: applet.AssetConfig{
				BasePath: "/assets",
				Dev: &applet.DevAssetConfig{
					Enabled:   true,
					TargetURL: vite.URL,
				},
			},
		},
	}
	c, err := applet.NewAppletController(a, nil, applet.DefaultSessionConfig, nil, nil, &testHostServices{})
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
		config: applet.Config{
			WindowGlobal: "__T__",
			Shell:        applet.ShellConfig{Mode: applet.ShellModeStandalone, Title: "Test"},
			Assets: applet.AssetConfig{
				BasePath: "/assets",
				Dev: &applet.DevAssetConfig{
					Enabled:     true,
					TargetURL:   vite.URL,
					EntryModule: entryModule,
				},
			},
		},
	}
	c, err := applet.NewAppletController(a, bundle, applet.DefaultSessionConfig, nil, nil, &testHostServices{})
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
		config: applet.Config{
			WindowGlobal: "__T__",
			Shell:        applet.ShellConfig{Mode: applet.ShellModeStandalone, Title: "t"},
			Assets: applet.AssetConfig{
				BasePath: "/assets",
				Dev: &applet.DevAssetConfig{
					Enabled:   true,
					TargetURL: upstream.URL,
				},
			},
		},
	}
	c, err := applet.NewAppletController(a, nil, applet.DefaultSessionConfig, nil, nil, &testHostServices{})
	require.NoError(t, err)
	router := mux.NewRouter()
	c.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/bi-chat/assets/@vite/client", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadGateway, rec.Code, "proxy should pass through upstream 502")
}

func ptr(b bool) *bool { return &b }
