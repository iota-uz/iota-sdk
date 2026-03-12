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
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	pkgtwofactor "github.com/iota-uz/iota-sdk/pkg/twofactor"
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

func TestFinalizeAuthenticatedUser_AccessCheckBlocked(t *testing.T) {
	// Access check runs before session creation in FinalizeAuthentication, so authService
	// and sessionService can be nil — the function returns early when the check fails.
	authFlowService := services.NewAuthFlowService(nil, nil)
	controller := &LoginController{
		authFlowService: authFlowService,
		options: &LoginControllerOptions{
			LoginAccessCheck: func(ctx context.Context, u coreuser.User) error {
				return errors.New("blocked")
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()

	controller.FinalizeAuthenticatedUser(w, req, mustTestUser(t), pkgtwofactor.AuthMethodExternal, "/dashboard")

	assert.Equal(t, http.StatusFound, w.Code)
	// Redirect URL is constructed from nextURL by the controller, not passed in directly —
	// this verifies the URL-construction logic is non-circular.
	assert.Equal(t, "/login?next=%2Fdashboard", w.Header().Get("Location"))
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
