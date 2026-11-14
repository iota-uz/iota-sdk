package e2e_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/benbjohnson/hashfs"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	internalassets "github.com/iota-uz/iota-sdk/internal/assets"
	coreassets "github.com/iota-uz/iota-sdk/modules/core/presentation/assets"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
)

// The browser suite (pkg/extruntime/e2e/browser) drives a real Chromium against the application this package
// composes. TestBrowserServer is its server: it is skipped unless the suite's runner asks for it, and then it
//
//   - composes the same application as every scenario (real modules, session auth, real Node workers, real build
//     worker) and activates the fixture packages through the management API;
//
//   - mints a signed-in session for a handful of users and writes their cookies, and the URL it listens on, to the
//     state file the runner reads;
//
//   - serves the application on a local port, with the runtime host bundle the runner built at
//     /assets/runtime-host.js (the file an application's frontend build would emit);
//
//   - waits until the runner stops it (SIGINT/SIGTERM, or its stdin closing).
//
//     IOTA_EXT_BROWSER_STATE    path of the JSON state file to write (the switch that enables this test)
//     IOTA_EXT_BROWSER_HOST_JS  path of the runtime host bundle (browser/build-host.mjs); its lazy chunks (the UI kit)
//     sit next to it and are served as /assets/runtime-host-chunk-*.js
//     IOTA_EXT_BROWSER_HOST_CSS optional path of the host stylesheet (the SDK tokens and component classes); stands in for
//     the application's compiled main.min.css when a checkout has not built it
//     IOTA_EXT_BROWSER_PORT     port to listen on, 0 or unset for a free one
const (
	browserStateEnv  = "IOTA_EXT_BROWSER_STATE"
	browserHostEnv   = "IOTA_EXT_BROWSER_HOST_JS"
	browserCSSEnv    = "IOTA_EXT_BROWSER_HOST_CSS"
	browserPortEnv   = "IOTA_EXT_BROWSER_PORT"
	hostEntryPath    = "/assets/runtime-host.js"
	hostChunkPrefix  = "/assets/runtime-host-chunk-"
	languageEndpoint = "/__e2e/language"

	permBrowserUse   = "e2e.browser.use"
	permBrowserAudit = "e2e.browser.audit"
)

// browserUser is a signed-in user of the browser suite.
type browserUser struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	CookieName string `json:"cookieName"`
	Token      string `json:"token"`
}

// browserState is what the runner learns about the running server.
type browserState struct {
	BaseURL string                 `json:"baseURL"`
	Users   map[string]browserUser `json:"users"`
	// Clients are the keys of the seeded CRM clients by first name.
	Clients map[string]string `json:"clients"`
}

func newBrowserUser(name string, a *actor) browserUser {
	return browserUser{ID: a.id, Name: name, Email: a.email, CookieName: "sid", Token: a.token}
}

func TestBrowserServer(t *testing.T) {
	statePath := os.Getenv(browserStateEnv)
	if statePath == "" {
		t.Skipf("the browser suite's server: set %s (see pkg/extruntime/e2e/browser)", browserStateEnv)
	}
	hostJS := os.Getenv(browserHostEnv)
	require.NotEmpty(t, hostJS, "%s is the runtime host bundle built by browser/build-host.mjs", browserHostEnv)
	require.FileExists(t, hostJS)

	e := newEnv(t)
	admin := e.admin

	// Fixture packages, activated as an operator would: through the management API.
	annKey := e.newClient(admin, "Ann")
	bobKey := e.newClient(admin, "Bob")
	e.deploy(admin, "e2e.clientextra", "1.0.0", loadTree(t, "clientextra"))
	e.mustRPC(admin, "command", "e2e.clientextra.setProfile",
		map[string]any{"client": map[string]string{"entity": "crm.client", "key": annKey}, "tier": "gold", "score": 30, "internalNote": "board-member"}, nil)
	e.deploy(admin, "e2e.tiercell", "1.0.0", loadTree(t, "tiercell"))
	e.deploy(admin, "e2e.browser", "1.0.0", loadTree(t, "browser"))
	e.deploy(admin, "e2e.styled", "1.0.0", loadTree(t, "styled"))

	// auditor holds every right of the suite; member lacks the audit panels; viewer only reads clients;
	// admin manages extensions (bindings) and, as the legacy pages need, reads clients.
	auditor := e.newActor("auditor", append(extensionPerms(), "User.Read", "Client.Read", permBrowserUse, permBrowserAudit, permNotesWrite)...)
	member := e.newActor("member", "Client.Read", permBrowserUse)
	viewer := e.newActor("viewer", "Client.Read")
	state := browserState{
		Users: map[string]browserUser{
			"admin": newBrowserUser("admin", admin), "auditor": newBrowserUser("auditor", auditor),
			"member": newBrowserUser("member", member), "viewer": newBrowserUser("viewer", viewer),
		},
		Clients: map[string]string{"Ann": annKey, "Bob": bobKey},
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// The runner holds our stdin; its death or an explicit close stops the server too.
	go func() {
		_, _ = io.Copy(io.Discard, os.Stdin)
		stop()
	}()

	port := os.Getenv(browserPortEnv)
	if port == "" {
		port = "0"
	}
	lis, err := (&net.ListenConfig{}).Listen(ctx, "tcp", net.JoinHostPort("127.0.0.1", port))
	require.NoError(t, err)

	handler := e.browserHandler(hostJS, os.Getenv(browserCSSEnv))
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 30 * time.Second}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(lis) }()
	t.Cleanup(func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	})

	state.BaseURL = "http://" + lis.Addr().String()
	raw, err := json.MarshalIndent(state, "", "  ")
	require.NoError(t, err)
	// The runner polls for the file: write it whole, then move it into place.
	tmp := statePath + ".tmp"
	require.NoError(t, os.WriteFile(tmp, raw, 0o600))
	require.NoError(t, os.Rename(tmp, statePath))
	t.Logf("browser suite server ready on %s", state.BaseURL)

	select {
	case <-ctx.Done():
	case err := <-serveErr:
		require.NoError(t, err)
	}
}

// browserHandler fronts the application with what a deployment adds around it: the static assets (htmx, Alpine,
// the theme script), the frontend bundle of the runtime host, and a test-only control for state the suite cannot
// change through the application's UI.
func (e *env) browserHandler(hostJS, hostCSS string) http.Handler {
	r := mux.NewRouter()
	r.HandleFunc(hostEntryPath, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, req, hostJS)
	})
	// The lazy chunks of the host bundle (the UI kit) sit next to the entry; only the bundler's own names are served.
	chunkDir := filepath.Dir(hostJS)
	r.HandleFunc(hostChunkPrefix+"{name:[A-Za-z0-9_-]+}.js", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, req, filepath.Join(chunkDir, "runtime-host-chunk-"+mux.Vars(req)["name"]+".js"))
	})
	// POST {"user": <id>, "language": "ru"}: the user's saved interface language, as the account page would set it.
	r.HandleFunc(languageEndpoint, func(w http.ResponseWriter, req *http.Request) {
		var in struct {
			User     uint   `json:"user"`
			Language string `json:"language"`
		}
		if req.Method != http.MethodPost || json.NewDecoder(req.Body).Decode(&in) != nil || in.User == 0 || in.Language == "" {
			http.Error(w, "POST {user, language}", http.StatusBadRequest)
			return
		}
		if _, err := e.pool.Exec(req.Context(), `UPDATE users SET ui_language = $1 WHERE id = $2 AND tenant_id = $3`, in.Language, in.User, e.tenant); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	// The realtime socket needs the application's hub, which this composition does not run: refuse it cleanly
	// instead of letting the controller panic on every page load.
	r.Path("/ws").HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "the realtime socket is not served in the browser suite", http.StatusNotFound)
	})
	// The compiled Tailwind stylesheet is a build product (`just css`). A checkout without it gets the host stylesheet the
	// runner built (the SDK tokens and component classes, as the real one carries them), or an empty one, so the page does
	// not fail on a missing resource.
	if f, err := coreassets.FS.Open("css/main.min.css"); err != nil {
		r.HandleFunc("/assets/css/main.min.css", func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
			if hostCSS != "" {
				http.ServeFile(w, req, hostCSS)
			}
		})
	} else {
		_ = f.Close()
	}
	// The static files the server's bootstrap serves (cmd/server installs the same file systems).
	controllers.NewStaticFilesController([]*hashfs.FS{coreassets.HashFS, internalassets.HashFS}).Register(r)
	r.PathPrefix("/").Handler(e.srv.Config.Handler)
	return r
}
