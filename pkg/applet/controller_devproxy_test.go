package applet

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
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

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
				config: Config{
					WindowGlobal: "__T__",
					Shell:        ShellConfig{Mode: ShellModeStandalone, Title: "t"},
					Assets: AssetConfig{
						BasePath: assetsPath,
						Dev: &DevAssetConfig{
							Enabled:     true,
							TargetURL:   backend.URL,
							StripPrefix: strip,
						},
					},
				},
			}
			c := NewAppletController(a, nil, DefaultSessionConfig, nil, nil)
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

	// Start a server, get URL, then close it so the target is deterministically unreachable (connection refused).
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())

	targetURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	a := &testApplet{
		name:     "chat",
		basePath: "/bi-chat",
		config: Config{
			WindowGlobal: "__T__",
			Shell:        ShellConfig{Mode: ShellModeStandalone, Title: "t"},
			Assets: AssetConfig{
				BasePath: "/assets",
				Dev: &DevAssetConfig{
					Enabled:   true,
					TargetURL: targetURL,
				},
			},
		},
	}
	c := NewAppletController(a, nil, DefaultSessionConfig, nil, nil)
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
		config: Config{
			WindowGlobal: "__T__",
			Shell:        ShellConfig{Mode: ShellModeStandalone, Title: "t"},
			Assets: AssetConfig{
				BasePath: "/assets",
				Dev: &DevAssetConfig{
					Enabled:   true,
					TargetURL: vite.URL,
				},
			},
		},
	}
	c := NewAppletController(a, nil, DefaultSessionConfig, nil, nil)
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

// TestDevProxy_HTMLShell asserts that GET basePath returns HTML containing the expected script tags for dev (Vite client + entry).
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
		config: Config{
			WindowGlobal: "__T__",
			Shell:        ShellConfig{Mode: ShellModeStandalone, Title: "Test"},
			Assets: AssetConfig{
				BasePath: "/assets",
				Dev: &DevAssetConfig{
					Enabled:     true,
					TargetURL:   vite.URL,
					EntryModule: entryModule,
				},
			},
		},
	}
	c := NewAppletController(a, bundle, DefaultSessionConfig, nil, nil)
	router := mux.NewRouter()
	c.RegisterRoutes(router)

	u := user.New("Test", "User", internet.MustParseEmail("test@example.com"), user.UILanguageEN, user.WithID(1))
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	ctx := context.Background()
	ctx = composables.WithUser(ctx, u)
	ctx = composables.WithTenantID(ctx, tenantID)
	ctx = composables.WithPageCtx(ctx, &types.PageContext{Locale: language.English})

	req := httptest.NewRequest(http.MethodGet, basePath, nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "GET %s should return 200", basePath)
	body := rec.Body.String()
	require.Contains(t, body, assetsBasePath+"/@vite/client", "HTML should contain script src for Vite client")
	require.Contains(t, body, assetsBasePath+"/src/main.tsx", "HTML should contain script src for entry module")
	require.True(t, strings.Contains(body, "<script") && strings.Contains(body, "src="), "HTML should contain at least one script tag with src")
}

// TestRegisterDevProxy_502WhenUpstreamReturns502 asserts the proxy returns 502 when the upstream responds with 502.
func TestRegisterDevProxy_502WhenUpstreamReturns502(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer upstream.Close()

	a := &testApplet{
		name:     "chat",
		basePath: "/bi-chat",
		config: Config{
			WindowGlobal: "__T__",
			Shell:        ShellConfig{Mode: ShellModeStandalone, Title: "t"},
			Assets: AssetConfig{
				BasePath: "/assets",
				Dev: &DevAssetConfig{
					Enabled:   true,
					TargetURL: upstream.URL,
				},
			},
		},
	}
	c := NewAppletController(a, nil, DefaultSessionConfig, nil, nil)
	router := mux.NewRouter()
	c.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/bi-chat/assets/@vite/client", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadGateway, rec.Code, "proxy should pass through upstream 502")
}

func ptr(b bool) *bool { return &b }
