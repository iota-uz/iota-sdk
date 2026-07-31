package share

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

type failingListStore struct{ Store }

func (s failingListStore) ListViews(context.Context, Access, string) ([]SavedView, error) {
	return nil, errors.New("database password leaked")
}

func TestHTTPHandlerSavedViewAndScheduleFlow(t *testing.T) {
	t.Parallel()
	access := Access{TenantID: uuid.New(), UserID: 17, ManageTeam: true, ScheduleMail: true}
	handler, err := NewHTTPHandler(NewMemoryStore(), func(*http.Request) (Access, error) { return access, nil }, "/lens/share", WithScheduledDelivery())
	require.NoError(t, err)
	router := mux.NewRouter()
	handler.Register(router)

	viewResponse := performJSON(t, router, http.MethodPost, handler.ViewsEndpoint(), SavedView{
		DashboardID: "claims", Name: "My slice", Scope: ViewScopePersonal, StateURL: "/claims?year=2026&_hidden=paid",
	})
	require.Equal(t, http.StatusOK, viewResponse.Code, viewResponse.Body.String())
	var view SavedView
	require.NoError(t, json.Unmarshal(viewResponse.Body.Bytes(), &view))
	require.NotEqual(t, uuid.Nil, view.ID)

	list := httptest.NewRecorder()
	router.ServeHTTP(list, httptest.NewRequest(http.MethodGet, handler.ViewsEndpoint()+"?dashboard=claims", nil))
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	require.Contains(t, list.Body.String(), "My slice")

	scheduleResponse := performJSON(t, router, http.MethodPost, handler.SchedulesEndpoint(), ExportSchedule{
		DashboardID: "claims", ViewID: view.ID, Name: "Monday claims", Cron: "0 8 * * 1", Timezone: "Asia/Tashkent",
		Recipients: []string{"analyst@example.com"}, Enabled: true,
	})
	require.Equal(t, http.StatusOK, scheduleResponse.Code, scheduleResponse.Body.String())
	var schedule ExportSchedule
	require.NoError(t, json.Unmarshal(scheduleResponse.Body.Bytes(), &schedule))
	require.False(t, schedule.NextRunAt.IsZero())

	deleted := httptest.NewRecorder()
	router.ServeHTTP(deleted, httptest.NewRequest(http.MethodDelete, handler.ViewsEndpoint()+"/"+view.ID.String(), nil))
	require.Equal(t, http.StatusNoContent, deleted.Code, deleted.Body.String())
}

func TestHTTPHandlerReturnsDeterministicMatchingRoleDefault(t *testing.T) {
	t.Parallel()
	roleFour := uint(4)
	roleSeven := uint(7)
	access := Access{
		TenantID: uuid.New(), UserID: 17, ManageTeam: true,
		RoleIDs: []uint{roleSeven, roleFour},
		Roles:   []RoleRef{{ID: roleSeven, Name: "Executive"}, {ID: roleFour, Name: "Analyst"}},
	}
	handler, err := NewHTTPHandler(NewMemoryStore(), func(*http.Request) (Access, error) { return access, nil }, "/lens/defaults")
	require.NoError(t, err)
	router := mux.NewRouter()
	handler.Register(router)
	for _, view := range []SavedView{
		{DashboardID: "claims", Name: "Executive", Scope: ViewScopeTeam, StateURL: "/claims?segment=executive", DefaultRoleID: &roleSeven},
		{DashboardID: "claims", Name: "Analyst", Scope: ViewScopeTeam, StateURL: "/claims?segment=analyst", DefaultRoleID: &roleFour},
	} {
		response := performJSON(t, router, http.MethodPost, handler.ViewsEndpoint(), view)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, handler.ViewsEndpoint()+"?dashboard=claims", nil))
	require.Equal(t, http.StatusOK, response.Code)
	var body struct {
		DefaultView SavedView `json:"defaultView"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Equal(t, roleFour, *body.DefaultView.DefaultRoleID)
	require.Equal(t, "/claims?segment=analyst", body.DefaultView.StateURL)
}

func TestHTTPHandlerKeepsScheduleCapabilityOffUntilDeliveryIsRegistered(t *testing.T) {
	t.Parallel()
	access := Access{TenantID: uuid.New(), UserID: 17, ScheduleMail: true}
	handler, err := NewHTTPHandler(NewMemoryStore(), func(*http.Request) (Access, error) { return access, nil }, "/lens/safe-default")
	require.NoError(t, err)
	router := mux.NewRouter()
	handler.Register(router)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, handler.ViewsEndpoint()+"?dashboard=claims", nil))
	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), `"scheduleMail":false`)

	schedule := performJSON(t, router, http.MethodPost, handler.SchedulesEndpoint(), ExportSchedule{})
	require.Equal(t, http.StatusForbidden, schedule.Code)
}

func TestHTTPHandlerRejectsUnauthenticatedAndUnknownFields(t *testing.T) {
	t.Parallel()
	handler, err := NewHTTPHandler(NewMemoryStore(), func(*http.Request) (Access, error) { return Access{}, nil }, "")
	require.NoError(t, err)
	router := mux.NewRouter()
	handler.Register(router)

	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, handler.ViewsEndpoint()+"?dashboard=claims", nil))
	require.Equal(t, http.StatusUnauthorized, unauthorized.Code)

	authenticated, err := NewHTTPHandler(NewMemoryStore(), func(*http.Request) (Access, error) {
		return Access{TenantID: uuid.New(), UserID: 1}, nil
	}, "/lens/auth")
	require.NoError(t, err)
	router = mux.NewRouter()
	authenticated.Register(router)
	bad := httptest.NewRecorder()
	router.ServeHTTP(bad, httptest.NewRequest(http.MethodPost, authenticated.ViewsEndpoint(), bytes.NewBufferString(`{"unknown":true}`)))
	require.Equal(t, http.StatusBadRequest, bad.Code)
}

func TestHTTPHandlerSeparatesAuthenticationAndInternalFailures(t *testing.T) {
	t.Parallel()
	internalAccess, err := NewHTTPHandler(NewMemoryStore(), func(*http.Request) (Access, error) {
		return Access{}, errors.New("role query exposed secret")
	}, "/lens/internal")
	require.NoError(t, err)
	router := mux.NewRouter()
	internalAccess.Register(router)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, internalAccess.ViewsEndpoint()+"?dashboard=claims", nil))
	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.NotContains(t, response.Body.String(), "secret")

	unauthenticated, err := NewHTTPHandler(NewMemoryStore(), func(*http.Request) (Access, error) {
		return Access{}, ErrUnauthenticated
	}, "/lens/unauthenticated")
	require.NoError(t, err)
	router = mux.NewRouter()
	unauthenticated.Register(router)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, unauthenticated.ViewsEndpoint()+"?dashboard=claims", nil))
	require.Equal(t, http.StatusUnauthorized, response.Code)

	access := Access{TenantID: uuid.New(), UserID: 1}
	internalStore, err := NewHTTPHandler(failingListStore{Store: NewMemoryStore()}, func(*http.Request) (Access, error) {
		return access, nil
	}, "/lens/store")
	require.NoError(t, err)
	router = mux.NewRouter()
	internalStore.Register(router)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, internalStore.ViewsEndpoint()+"?dashboard=claims", nil))
	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.NotContains(t, response.Body.String(), "password")

	invalid := performJSON(t, router, http.MethodPost, internalStore.ViewsEndpoint(), SavedView{})
	// The failing store delegates PutView to MemoryStore, so validation remains
	// a client error while unexpected persistence failures stay opaque.
	require.Equal(t, http.StatusBadRequest, invalid.Code)
	require.Contains(t, invalid.Body.String(), "dashboardId")
}

func performJSON(t *testing.T, handler http.Handler, method, path string, value any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(value)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, path, bytes.NewReader(payload)))
	return recorder
}
