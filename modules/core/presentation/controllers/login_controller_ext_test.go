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
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	pkgtwofactor "github.com/iota-uz/iota-sdk/pkg/twofactor"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

type stubMethodProvider struct {
	id     string
	method *LoginMethod
}

func (s stubMethodProvider) ID() string {
	return s.id
}

func (s stubMethodProvider) RegisterRoutes(r *mux.Router, c LoginFlowHandler) {
	// no-op
}

func (s stubMethodProvider) BuildMethod(ctx context.Context, r *http.Request) (*LoginMethod, error) {
	return s.method, nil
}

type errorPolicy struct{}

func (errorPolicy) Requires(ctx context.Context, attempt pkgtwofactor.AuthAttempt) (bool, error) {
	return false, errors.New("boom")
}

func TestBuildLoginMethodsOrder(t *testing.T) {
	includeGoogle := false
	includePassword := false

	tests := []struct {
		name          string
		options       *LoginControllerOptions
		localizerJSON string
		wantIDs       []string
	}{
		{
			name: "default password and providers",
			options: &LoginControllerOptions{
				IncludeGoogleMethod: &includeGoogle,
				MethodProviders: []LoginMethodProvider{
					stubMethodProvider{id: "eimzo", method: &LoginMethod{ID: "eimzo", Label: "E-IMZO"}},
					stubMethodProvider{id: "otp", method: &LoginMethod{ID: "otp", Label: "OTP"}},
				},
			},
			localizerJSON: `{"Login":{"Login":"Log in"}}`,
			wantIDs:       []string{"password", "eimzo", "otp"},
		},
		{
			name: "external method ID is trimmed",
			options: &LoginControllerOptions{
				IncludePasswordMethod: &includePassword,
				IncludeGoogleMethod:   &includeGoogle,
				MethodProviders: []LoginMethodProvider{
					stubMethodProvider{id: "provider-1", method: &LoginMethod{ID: "  external-id  ", Label: "External"}},
				},
			},
			localizerJSON: `{"Login":{}}`,
			wantIDs:       []string{"external-id"},
		},
		{
			name: "no methods configured",
			options: &LoginControllerOptions{
				IncludePasswordMethod: boolPtr(false),
				IncludeGoogleMethod:   boolPtr(false),
				MethodProviders:       []LoginMethodProvider{},
			},
			localizerJSON: `{"Login":{}}`,
			wantIDs:       []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			controller := &LoginController{options: tc.options}
			req := httptest.NewRequest(http.MethodGet, "/login", nil)
			req = req.WithContext(withLocalizer(t, req.Context(), tc.localizerJSON))

			methods, err := controller.buildLoginMethods(httptest.NewRecorder(), req)
			require.NoError(t, err)

			gotIDs := make([]string, 0, len(methods))
			for _, method := range methods {
				gotIDs = append(gotIDs, method.ID)
			}

			assert.Equal(t, tc.wantIDs, gotIDs)
		})
	}
}

func TestGetRenderModes(t *testing.T) {
	includePassword := false
	includeGoogle := false

	tests := []struct {
		name         string
		makeOptions  func() *LoginControllerOptions
		expectStatus int
		expectedBody string
	}{
		{
			name: "custom renderer handles empty methods",
			makeOptions: func() *LoginControllerOptions {
				return &LoginControllerOptions{
					IncludePasswordMethod: &includePassword,
					IncludeGoogleMethod:   &includeGoogle,
					Renderer: func(ctx context.Context, vm LoginPageViewModel) templ.Component {
						return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
							_, err := w.Write([]byte("custom-renderer"))
							return err
						})
					},
				}
			},
			expectStatus: http.StatusOK,
			expectedBody: "custom-renderer",
		},
		{
			name: "no login methods configured returns server error",
			makeOptions: func() *LoginControllerOptions {
				return &LoginControllerOptions{
					IncludePasswordMethod: &includePassword,
					IncludeGoogleMethod:   &includeGoogle,
				}
			},
			expectStatus: http.StatusInternalServerError,
			expectedBody: "no login methods configured\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			controller := &LoginController{options: tc.makeOptions()}
			req := httptest.NewRequest(http.MethodGet, "/login", nil)
			req = req.WithContext(withLocalizer(t, req.Context(), `{"Login":{"Meta":{"Title":"Login"},"Login":"Log in","LoginWithGoogle":"Log in with Google"}}`))
			w := httptest.NewRecorder()

			controller.Get(w, req)

			assert.Equal(t, tc.expectStatus, w.Code)
			assert.Equal(t, tc.expectedBody, w.Body.String())
		})
	}
}

func TestFinalizeAuthenticatedSessionAndUserRedirects(t *testing.T) {
	tests := []struct {
		name         string
		setup        func() *LoginController
		setupRequest func(t *testing.T, req *http.Request) *http.Request
		invoke       func(controller *LoginController, w http.ResponseWriter, req *http.Request, u coreuser.User)
		location     string
	}{
		{
			name: "access check blocks user",
			setup: func() *LoginController {
				return &LoginController{
					authFlowService: services.NewAuthFlowService(nil, nil),
					options: &LoginControllerOptions{
						LoginAccessCheck: func(ctx context.Context, u coreuser.User) error {
							return errors.New("blocked")
						},
					},
				}
			},
			invoke: func(controller *LoginController, w http.ResponseWriter, req *http.Request, u coreuser.User) {
				controller.FinalizeAuthenticatedUser(
					w,
					req,
					u,
					pkgtwofactor.AuthMethodExternal,
					"/dashboard",
				)
			},
			location: "/login?next=%2Fdashboard",
		},
		{
			name: "policy errors redirect to login",
			setup: func() *LoginController {
				authFlowService := services.NewAuthFlowService(nil, nil)
				authFlowService.SetTwoFactorPolicy(errorPolicy{})
				return &LoginController{authFlowService: authFlowService}
			},
			setupRequest: func(t *testing.T, req *http.Request) *http.Request {
				t.Helper()
				ctx := withLocalizer(t, req.Context(), `{"Errors":{"Internal":"Internal error"}}`)
				ctx = context.WithValue(ctx, constants.LoggerKey, logrus.New().WithField("test", true))
				return req.WithContext(ctx)
			},
			invoke: func(controller *LoginController, w http.ResponseWriter, req *http.Request, u coreuser.User) {
				controller.FinalizeAuthentication(
					w,
					req,
					&services.AuthenticationResult{
						User:            u,
						Session:         session.New("token", 1, uuid.New(), "127.0.0.1", "test-agent"),
						Method:          pkgtwofactor.AuthMethodExternal,
						AuthenticatorID: string(pkgtwofactor.AuthMethodExternal),
					},
					"/dashboard",
				)
			},
			location: "/login?next=%2Fdashboard",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			controller := tc.setup()

			req := httptest.NewRequest(http.MethodGet, "/login", nil)
			if tc.setupRequest != nil {
				req = tc.setupRequest(t, req)
			}

			w := httptest.NewRecorder()
			tc.invoke(controller, w, req, mustTestUser(t))

			assert.Equal(t, http.StatusFound, w.Code)
			assert.Equal(t, tc.location, w.Header().Get("Location"))
		})
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func mustTestUser(t *testing.T) coreuser.User {
	t.Helper()
	email, err := internet.NewEmail("user@example.com")
	require.NoError(t, err)
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
