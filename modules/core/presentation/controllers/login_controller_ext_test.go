package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/go-i18n/v2/i18n"
	coreuser "github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	pkgtwofactor "github.com/iota-uz/iota-sdk/pkg/twofactor"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"
)

type stubMethodProvider struct {
	id     string
	method *LoginMethod
}

func (s stubMethodProvider) ID() string {
	return s.id
}

func (s stubMethodProvider) RegisterRoutes(r *mux.Router, c *LoginController) {
	// no-op
}

func (s stubMethodProvider) BuildMethod(ctx context.Context, r *http.Request) (*LoginMethod, error) {
	return s.method, nil
}

type errorPolicy struct{}

func (errorPolicy) Requires(ctx context.Context, attempt pkgtwofactor.AuthAttempt) (bool, error) {
	return false, errors.New("boom")
}

func TestBuildLoginMethods_Order(t *testing.T) {
	includeGoogle := false
	controller := &LoginController{
		options: &LoginControllerOptions{
			IncludeGoogleMethod: &includeGoogle,
			MethodProviders: []LoginMethodProvider{
				stubMethodProvider{id: "eimzo", method: &LoginMethod{ID: "eimzo", Label: "E-IMZO"}},
				stubMethodProvider{id: "otp", method: &LoginMethod{ID: "otp", Label: "OTP"}},
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req = req.WithContext(withLocalizer(t, req.Context(), `{"Login":{"Login":"Log in"}}`))

	methods, err := controller.buildLoginMethods(httptest.NewRecorder(), req)
	if err != nil {
		t.Fatalf("build methods: %v", err)
	}

	if len(methods) != 3 {
		t.Fatalf("expected 3 methods, got %d", len(methods))
	}
	if methods[0].ID != "password" || methods[1].ID != "eimzo" || methods[2].ID != "otp" {
		t.Fatalf("unexpected method order: %#v", methods)
	}
}

func TestGet_UsesCustomRenderer(t *testing.T) {
	includePassword := false
	includeGoogle := false

	controller := &LoginController{
		options: &LoginControllerOptions{
			IncludePasswordMethod: &includePassword,
			IncludeGoogleMethod:   &includeGoogle,
			Renderer: func(ctx context.Context, vm LoginPageViewModel) templ.Component {
				return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
					_, err := io.WriteString(w, "custom-renderer")
					return err
				})
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req = req.WithContext(withLocalizer(t, req.Context(), `{"Login":{"Meta":{"Title":"Login"}}}`))
	w := httptest.NewRecorder()

	controller.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if body := w.Body.String(); body != "custom-renderer" {
		t.Fatalf("unexpected renderer body: %q", body)
	}
}

func TestFinalizeAuthenticatedUser_AccessCheckRedirects(t *testing.T) {
	controller := &LoginController{
		options: &LoginControllerOptions{
			LoginAccessCheck: func(ctx context.Context, u coreuser.User) error {
				return errors.New("blocked")
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()

	controller.FinalizeAuthenticatedUser(w, req, mustTestUser(t), pkgtwofactor.AuthMethodExternal, "/dashboard")

	if w.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d", w.Code)
	}
	if location := w.Header().Get("Location"); location != "/login?next=%2Fdashboard" {
		t.Fatalf("unexpected redirect location: %s", location)
	}
}

func TestFinalizeAuthenticatedSession_PolicyErrorRedirects(t *testing.T) {
	controller := &LoginController{
		twoFactorPolicy: errorPolicy{},
	}

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	ctx := withLocalizer(t, req.Context(), `{"Errors":{"Internal":"Internal error"}}`)
	ctx = context.WithValue(ctx, constants.LoggerKey, logrus.New().WithField("test", true))
	req = req.WithContext(ctx)

	sess := session.New("token", 1, uuid.New(), "127.0.0.1", "test-agent")
	w := httptest.NewRecorder()

	controller.finalizeAuthenticatedSession(
		w,
		req,
		mustTestUser(t),
		sess,
		pkgtwofactor.AuthMethodExternal,
		"/dashboard",
		"/login?next=%2Fdashboard",
	)

	if w.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d", w.Code)
	}
	if location := w.Header().Get("Location"); location != "/login?next=%2Fdashboard" {
		t.Fatalf("unexpected redirect location: %s", location)
	}
}

func mustTestUser(t *testing.T) coreuser.User {
	t.Helper()
	email, err := internet.NewEmail("user@example.com")
	if err != nil {
		t.Fatalf("create email: %v", err)
	}
	return coreuser.New(
		"John",
		"Doe",
		email,
		coreuser.UILanguageEN,
		coreuser.WithID(1),
		coreuser.WithTenantID(uuid.New()),
	)
}

func withLocalizer(t *testing.T, ctx context.Context, messagesJSON string) context.Context {
	t.Helper()
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	bundle.MustParseMessageFileBytes([]byte(messagesJSON), "en.json")
	return intl.WithLocalizer(ctx, i18n.NewLocalizer(bundle, "en"))
}
