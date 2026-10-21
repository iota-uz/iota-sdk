//go:build dev

package agentsignin

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	queryrepo "github.com/iota-uz/iota-sdk/pkg/repo"
)

type controller struct{ options Options }

func NewController(o Options) (application.Controller, error) {
	if !o.Enabled {
		return nil, nil //nolint:nilnil // Disabled opt-in deliberately registers no controller.
	}
	if err := o.Validate(); err != nil {
		return nil, err
	}
	if _, err := privateDirectory(o); err != nil {
		return nil, err
	}
	return &controller{options: o}, nil
}

func (c *controller) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("core.agent-sign-in", 0,
		application.Route(http.MethodGet, Path, application.Public()),
		application.Route(http.MethodPost, Path, application.Public()))
}

func (c *controller) Register(r *mux.Router) {
	sub := r.PathPrefix(Path).Subrouter()
	sub.Use(middleware.CSRF(nil))
	sub.HandleFunc("", c.get).Methods(http.MethodGet)
	sub.HandleFunc("", di.H(c.post)).Methods(http.MethodPost)
}

func (c *controller) allowed(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	u, parseErr := url.Parse(c.options.Origin)
	if err != nil || !loopback(host) || parseErr != nil || r.Host != u.Host {
		return false
	}
	if r.Header.Get("Origin") != "" && r.Header.Get("Origin") != c.options.Origin {
		return false
	}
	return r.Header.Get("Sec-Fetch-Site") != "cross-site"
}

func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

func (c *controller) get(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	// Form navigation needs its same-origin Origin header. The token is only
	// in the fragment and is removed before submission; cross-origin referrers
	// remain suppressed.
	w.Header().Set("Referrer-Policy", "same-origin")
	if !c.allowed(r) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := handoff().Render(r.Context(), w); err != nil {
		http.Error(w, "Sign-in unavailable", http.StatusInternalServerError)
	}
}

func (c *controller) post(r *http.Request, w http.ResponseWriter, repo user.Repository, auth *services.AuthService, browser *services.BrowserSessionService, appCfg *appconfig.Config) {
	noStore(w)
	if !c.allowed(r) || r.Header.Get("Origin") != c.options.Origin || appCfg.Environment != c.options.Environment {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid sign-in", http.StatusBadRequest)
		return
	}
	t, err := consume(c.options, r.PostForm.Get("Token"))
	if err != nil {
		http.Error(w, "Invalid or expired sign-in", http.StatusUnauthorized)
		return
	}
	ctx, cancel := context.WithTimeout(composables.WithTenantID(r.Context(), c.options.TenantID), 10*time.Second)
	defer cancel()
	var cookie *http.Cookie
	err = composables.InTx(ctx, func(ctx context.Context) error {
		u, err := resolveUser(ctx, repo, t.User)
		if err != nil {
			return err
		}
		if u.TenantID() != c.options.TenantID {
			return errors.New("tenant mismatch")
		}
		sess, err := auth.CreateAgentSession(ctx, u)
		if err != nil {
			return err
		}
		cookie, err = browser.AddFromRequest(ctx, r, sess)
		return err
	})
	if err != nil {
		http.Error(w, "Sign-in unavailable for this user", http.StatusUnauthorized)
		return
	}
	http.SetCookie(w, cookie)
	http.Redirect(w, r, t.Next, http.StatusSeeOther)
}

func resolveUser(ctx context.Context, repo user.Repository, selector string) (user.User, error) {
	if id, err := strconv.ParseUint(selector, 10, 32); err == nil && id > 0 {
		return repo.GetByID(ctx, uint(id))
	}
	users, err := repo.GetPaginated(ctx, &user.FindParams{Limit: 2, Filters: []user.Filter{{Column: user.EmailField, Filter: queryrepo.Eq(strings.TrimSpace(selector))}}})
	if err != nil {
		return nil, err
	}
	if len(users) != 1 {
		return nil, errors.New("email must identify exactly one user; use an ID")
	}
	return users[0], nil
}
