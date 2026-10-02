package serrorhttp_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	serrors "github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/serrors/serrorhttp"
	"github.com/stretchr/testify/require"
)

func TestWriteProblemSafeDependencyErrors(t *testing.T) {
	for _, code := range []serrors.Code{serrors.Internal, serrors.Unavailable, serrors.Timeout} {
		t.Run(code.String(), func(t *testing.T) {
			err := serrors.New(code, "secret SQL /private/path").WithCause(errors.New("password=secret")).WithMeta(map[string]serrors.Value{"token": serrors.Text("secret")})
			w := httptest.NewRecorder()
			require.NoError(t, serrorhttp.Write(w, err, nil, serrorhttp.Profile{}))
			require.Equal(t, serrorhttp.Status(err), w.Code)
			require.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))
			var body serrorhttp.Problem
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			require.Equal(t, code.String(), body.Code)
			require.Equal(t, w.Code, body.Status)
			require.NotEmpty(t, body.Detail)
			require.NotContains(t, w.Body.String(), "secret")
			require.NotContains(t, w.Body.String(), "/private")
		})
	}
}

func TestWriteProfileRetainsRouteContract(t *testing.T) {
	w := httptest.NewRecorder()
	profile := serrorhttp.Profile{Status: func(error) int { return http.StatusUnprocessableEntity }, Headers: http.Header{"Retry-After": {"30"}}, ContentType: "application/json", Encode: func(w http.ResponseWriter, p serrors.Projection, status int) error {
		return json.NewEncoder(w).Encode(map[string]any{"error": p.Message, "status": status})
	}}
	require.NoError(t, serrorhttp.Write(w, serrors.NewInvalid("private").WithPublic(serrors.Message{ID: "missing.translation"}), nil, profile))
	require.Equal(t, 422, w.Code)
	require.Equal(t, "30", w.Header().Get("Retry-After"))
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	require.JSONEq(t, `{"error":"Check the supplied information.","status":422}`, w.Body.String())
}

func TestWriteFormRetainsRouteOwnedFields(t *testing.T) {
	for _, hx := range []bool{false, true} {
		t.Run(fmt.Sprint(hx), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/form", strings.NewReader("Email=kept%40example.com"))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if hx {
				r.Header.Set("HX-Request", "true")
			}
			w := httptest.NewRecorder()
			err := serrors.NewInvalid("private").WithFields(serrors.FieldViolation{Field: "Email", Message: serrors.Message{ID: "missing"}})
			form := serrorhttp.Form{Target: "#account", Swap: "outerHTML", Render: func(w http.ResponseWriter, r *http.Request, p serrors.Projection) error {
				_, err := fmt.Fprintf(w, `<form id="account"><input name="Email" value="%s"><span>%s</span></form>`, html.EscapeString(r.FormValue("Email")), html.EscapeString(p.Fields[0].Message))
				return err
			}}
			require.NoError(t, serrorhttp.WriteForm(w, r, err, nil, form))
			expected := 400
			if hx {
				expected = 200
			}
			require.Equal(t, expected, w.Code)
			require.Equal(t, "#account", w.Header().Get("HX-Retarget"))
			require.Equal(t, "outerHTML", w.Header().Get("HX-Reswap"))
			require.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
			require.Contains(t, w.Body.String(), `name="Email" value="kept@example.com"`)
			require.Contains(t, w.Body.String(), "Check the supplied information.")
			require.NotContains(t, w.Body.String(), "private")
		})
	}
}

func TestWriteFormRequiresRendererBeforeWriting(t *testing.T) {
	w := httptest.NewRecorder()
	err := serrorhttp.WriteForm(w, httptest.NewRequest("POST", "/", nil), serrors.NewInvalid("private"), nil, serrorhttp.Form{})
	require.True(t, serrors.HasCode(err, serrors.Internal))
	require.Empty(t, w.Body.String())
	require.Empty(t, w.Header())
}

func TestWriteTextPreservesPlainTextRouteContract(t *testing.T) {
	w := httptest.NewRecorder()
	w.Header().Set("X-Route", "existing")
	serrorhttp.WriteText(w, serrors.NewInternal("SELECT secret").WithCause(errors.New("secret cause")), http.StatusBadGateway, nil)
	require.Equal(t, http.StatusBadGateway, w.Code)
	require.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	require.Equal(t, "existing", w.Header().Get("X-Route"))
	require.Equal(t, "An unexpected error occurred.\n", w.Body.String())
}

func TestWriteTextClassifiesSemanticErrorsAndRetainsUnknownRouteStatus(t *testing.T) {
	for _, tc := range []struct {
		err               error
		routeStatus, want int
	}{{serrors.NewNotFound("secret"), 500, 404}, {serrors.NewInvalid("secret"), 500, 400}, {errors.New("private decoding details"), 400, 400}} {
		w := httptest.NewRecorder()
		serrorhttp.WriteText(w, tc.err, tc.routeStatus, nil)
		require.Equal(t, tc.want, w.Code)
		require.NotContains(t, w.Body.String(), "secret")
		require.NotContains(t, w.Body.String(), "private")
	}
}
