package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/a-h/templ"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestRedirectNotAuthenticatedPreservesTheCompleteReturnURL(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/analytics/profitability?range-start=2026-01-01&range-end=2026-12-31&compare=previous_period", nil)
	request = request.WithContext(composables.WithParams(request.Context(), &composables.Params{}))
	handler := RedirectNotAuthenticated()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unauthenticated request reached the protected handler")
	}))

	handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusFound, recorder.Code)
	location, err := url.Parse(recorder.Header().Get("Location"))
	require.NoError(t, err)
	require.Equal(t, request.URL.RequestURI(), location.Query().Get("next"))
}

func TestRenderRouteForbiddenSetsHTMLContentType(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/forbidden", nil)
	request = request.WithContext(forbiddenPageTestContext(t))

	renderRouteForbidden(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Equal(t, "text/html; charset=utf-8", recorder.Header().Get("Content-Type"))
}

func TestRenderRouteForbiddenFallsBackWhenPageContextMissing(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/forbidden", nil)

	renderRouteForbidden(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Equal(t, "text/html; charset=utf-8", recorder.Header().Get("Content-Type"))
	require.Contains(t, recorder.Body.String(), "403 Forbidden")
}

func forbiddenPageTestContext(t *testing.T) context.Context {
	t.Helper()

	bundle := i18n.NewBundle(language.English)
	bundle.MustAddMessages(language.English,
		&i18n.Message{ID: "ErrorPages.Forbidden.Message", Other: "Forbidden"},
		&i18n.Message{ID: "ErrorPages.Forbidden._Description", Other: "Access denied."},
		&i18n.Message{ID: "ErrorPages.Forbidden.Home", Other: "Home"},
		&i18n.Message{ID: "ErrorPages.Forbidden.GoBack", Other: "Go back"},
	)
	ctx := intl.WithLocale(t.Context(), language.English)
	ctx = intl.WithLocalizer(ctx, i18n.NewLocalizer(bundle, language.English.String()))
	return context.WithValue(ctx, constants.HeadKey, templ.ComponentFunc(func(context.Context, io.Writer) error {
		return nil
	}))
}
