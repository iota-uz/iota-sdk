package serve

import (
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQueryProtocolErrorKeepsSchemaWithoutEchoingPrivateInput(t *testing.T) {
	handlers, _, _ := newTestHandlers(t, 0)
	w := httptest.NewRecorder()
	handlers.Query(w, httptest.NewRequest(http.MethodPost, "/dash/lens/query", strings.NewReader(`{"snapshotId":"snapshot","password=secret":true}`)))
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	require.JSONEq(t, `{"error":"bad_request","message":"invalid JSON body: unknown field"}`, w.Body.String())
	require.NotContains(t, w.Body.String(), "secret")
}
