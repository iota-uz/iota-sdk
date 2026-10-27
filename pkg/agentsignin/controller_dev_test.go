//go:build dev

package agentsignin_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/agentsession"
	"github.com/iota-uz/iota-sdk/pkg/agentsignin"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/twofactor"
	"github.com/stretchr/testify/require"
)

// Falsely green if the fixture gains permissions or a fabricated cookie replaces the HTTP-issued cookie.
func TestMountedHandoffCreatesRealBlockedUserSession(t *testing.T) {
	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{PermissionSchema: defaults.PermissionSchema()})).AsAdmin().Build()
	env := suite.Env()
	repo, err := composition.Resolve[user.Repository](env.Container)
	require.NoError(t, err)
	email, err := internet.NewEmail("agent@example.com")
	require.NoError(t, err)
	u, err := repo.Create(env.Ctx, user.New("Agent", "Test", email, "en", user.WithTenantID(env.TenantID()), user.WithTwoFactorMethod(twofactor.MethodEmail), user.WithTwoFactorEnabledAt(time.Now())))
	require.NoError(t, err)
	u = u.Block("Local test", u.ID(), env.TenantID())
	require.NoError(t, repo.Update(env.Ctx, u))
	o := agentsignin.Options{Enabled: true, Environment: "development", Origin: "http://localhost:3200", TenantID: env.TenantID(), Directory: filepath.Join(t.TempDir(), "tickets")}
	c, err := agentsignin.NewController(o)
	require.NoError(t, err)
	r := mux.NewRouter()
	r.Use(itf.TestMiddleware(env, env.User), middleware.CSRF(nil))
	c.Register(r)
	link, err := agentsignin.Issue(o, strconv.Itoa(int(u.ID())), "/protected")
	require.NoError(t, err)
	parsed, err := url.Parse(link)
	require.NoError(t, err)
	token := strings.TrimPrefix(parsed.Fragment, "token=")
	request := func(method, origin string) *httptest.ResponseRecorder {
		body := url.Values{"Token": {token}}.Encode()
		req := httptest.NewRequest(method, o.Origin+agentsignin.Path, strings.NewReader(body))
		req.RemoteAddr = "127.0.0.1:1234"
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	require.Equal(t, http.StatusForbidden, request(http.MethodPost, "http://evil.example").Code)
	require.Equal(t, http.StatusOK, request(http.MethodGet, o.Origin).Code)
	w := request(http.MethodPost, o.Origin)
	require.Equal(t, http.StatusSeeOther, w.Code, w.Body.String())
	require.Equal(t, "/protected", w.Header().Get("Location"))
	require.NotEmpty(t, w.Result().Cookies())
	browser, err := composition.Resolve[*services.BrowserSessionService](env.Container)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodGet, o.Origin+"/protected", nil)
	for _, cookie := range w.Result().Cookies() {
		req.AddCookie(cookie)
	}
	active, err := browser.Active(httptest.NewRecorder(), req.WithContext(env.Ctx))
	require.NoError(t, err)
	require.Equal(t, u.ID(), active.User.ID())
	require.Empty(t, active.User.Roles())
	require.Empty(t, active.User.Permissions())
	require.True(t, active.User.IsBlocked())
	require.True(t, active.User.Has2FAEnabled())
	require.True(t, active.Session.IsActive())
	require.True(t, agentsession.Is(active.Session, "development"))
	auth, err := composition.Resolve[*services.AuthService](env.Container)
	require.NoError(t, err)
	_, err = auth.Authorize(env.Ctx, active.Session.Token())
	require.NoError(t, err)
	normalSession, err := auth.CreateSession(env.Ctx, u)
	require.NoError(t, err)
	normalCookie, err := browser.Add(env.Ctx, "", normalSession)
	require.NoError(t, err)
	normalRequest := httptest.NewRequest(http.MethodGet, o.Origin+"/protected", nil)
	normalRequest.AddCookie(normalCookie)
	_, err = browser.Active(httptest.NewRecorder(), normalRequest.WithContext(env.Ctx))
	require.Error(t, err)
	require.Equal(t, http.StatusUnauthorized, request(http.MethodPost, o.Origin).Code)
}

func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}
