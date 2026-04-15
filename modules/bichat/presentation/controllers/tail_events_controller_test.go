package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/persistence"
	coreuser "github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeStreamCommands is a minimal StreamCommands stub for controller unit tests.
type fakeStreamCommands struct {
	tailRunEventsFunc func(ctx context.Context, sessionID, runID uuid.UUID, from string, onEvent func(bichatservices.RunEventDelivery)) error
}

func (f *fakeStreamCommands) SendMessageStream(_ context.Context, _ bichatservices.SendMessageRequest, _ func(bichatservices.StreamChunk)) error {
	return nil
}
func (f *fakeStreamCommands) StopGeneration(_ context.Context, _ uuid.UUID) error { return nil }
func (f *fakeStreamCommands) GetStreamStatus(_ context.Context, _ uuid.UUID) (*bichatservices.StreamStatus, error) {
	return &bichatservices.StreamStatus{}, nil
}
func (f *fakeStreamCommands) ResumeStream(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ func(bichatservices.StreamChunk)) error {
	return nil
}
func (f *fakeStreamCommands) TailRunEvents(ctx context.Context, sessionID, runID uuid.UUID, from string, onEvent func(bichatservices.RunEventDelivery)) error {
	if f.tailRunEventsFunc != nil {
		return f.tailRunEventsFunc(ctx, sessionID, runID, from, onEvent)
	}
	return bichatservices.ErrRunEventLogUnavailable
}
func (f *fakeStreamCommands) TailActiveRuns(_ context.Context, _ func(bichatservices.ActiveRunDelivery)) error {
	return bichatservices.ErrRunEventLogUnavailable
}

// fakeSessionQueries satisfies bichatservices.SessionQueries with minimal stubs.
type fakeSessionQueries struct{}

func (f *fakeSessionQueries) GetSession(_ context.Context, _ uuid.UUID) (domain.Session, error) {
	return nil, persistence.ErrSessionNotFound
}
func (f *fakeSessionQueries) ListUserSessions(_ context.Context, _ int64, _ domain.ListOptions) ([]domain.Session, error) {
	return nil, nil
}
func (f *fakeSessionQueries) CountUserSessions(_ context.Context, _ int64, _ domain.ListOptions) (int, error) {
	return 0, nil
}
func (f *fakeSessionQueries) ListAccessibleSessions(_ context.Context, _ int64, _ domain.ListOptions) ([]domain.SessionSummary, error) {
	return nil, nil
}
func (f *fakeSessionQueries) CountAccessibleSessions(_ context.Context, _ int64, _ domain.ListOptions) (int, error) {
	return 0, nil
}
func (f *fakeSessionQueries) ListAllSessions(_ context.Context, _ int64, _ domain.ListOptions, _ *int64) ([]domain.SessionSummary, error) {
	return nil, nil
}
func (f *fakeSessionQueries) CountAllSessions(_ context.Context, _ domain.ListOptions, _ *int64) (int, error) {
	return 0, nil
}
func (f *fakeSessionQueries) ResolveSessionAccess(_ context.Context, _ uuid.UUID, _ int64, _ bool) (domain.SessionAccess, error) {
	access, _ := domain.NewSessionAccess(domain.SessionMemberRoleOwner, domain.SessionAccessSourceOwner)
	return access, nil
}
func (f *fakeSessionQueries) ListSessionMembers(_ context.Context, _ uuid.UUID) ([]domain.SessionMember, error) {
	return nil, nil
}
func (f *fakeSessionQueries) GetTenantUser(_ context.Context, _ int64) (domain.SessionUser, error) {
	return domain.SessionUser{}, nil
}
func (f *fakeSessionQueries) ListTenantUsers(_ context.Context) ([]domain.SessionUser, error) {
	return nil, nil
}

// testUser builds a minimal coreuser.User for injecting into request context.
func testUser(t *testing.T) coreuser.User {
	t.Helper()
	email, err := internet.NewEmail("test@example.com")
	require.NoError(t, err)
	return coreuser.New("Test", "User", email, coreuser.UILanguageEN)
}

// TestTailEvents_LastEventIDForwardedToService verifies that the
// Last-Event-ID header value is passed verbatim as the `from` parameter
// to StreamCommands.TailRunEvents.
func TestTailEvents_LastEventIDForwardedToService(t *testing.T) {
	t.Parallel()

	sessionID := uuid.New()
	runID := uuid.New()
	const lastEventID = "1712345678000-0"

	var capturedFrom string
	fake := &fakeStreamCommands{
		tailRunEventsFunc: func(_ context.Context, _, _ uuid.UUID, from string, onEvent func(bichatservices.RunEventDelivery)) error {
			capturedFrom = from
			// Deliver one content event.
			payload, err := json.Marshal(map[string]string{"type": "content", "content": "hello"})
			require.NoError(t, err)
			onEvent(bichatservices.RunEventDelivery{
				StreamID: "1712345678001-0",
				Type:     "content",
				Payload:  payload,
			})
			// Deliver terminal done.
			donePayload, err := json.Marshal(map[string]string{"type": "done"})
			require.NoError(t, err)
			onEvent(bichatservices.RunEventDelivery{
				StreamID: "1712345678002-0",
				Type:     "done",
				Payload:  donePayload,
			})
			return nil
		},
	}

	controller := NewStreamController(fake, &fakeSessionQueries{}, nil)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf(
		"/stream/events?sessionId=%s&runId=%s", sessionID, runID,
	), nil)
	req.Header.Set("Last-Event-ID", lastEventID)
	req = req.WithContext(composables.WithUser(req.Context(), testUser(t)))

	rw := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/stream/events", controller.TailEvents).Methods("GET")
	router.ServeHTTP(rw, req)

	require.Equal(t, http.StatusOK, rw.Code)
	assert.Equal(t, lastEventID, capturedFrom, "Last-Event-ID must be forwarded to TailRunEvents")

	body := rw.Body.String()
	assert.Contains(t, body, "id: 1712345678001-0", "SSE id line must be written")
	assert.Contains(t, body, "event: content", "SSE event: content line must be written")
}

// TestTailEvents_MissingParamsReturns400 verifies 400 on missing query params.
func TestTailEvents_MissingParamsReturns400(t *testing.T) {
	t.Parallel()

	controller := NewStreamController(&fakeStreamCommands{}, &fakeSessionQueries{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/stream/events", nil)
	req = req.WithContext(composables.WithUser(req.Context(), testUser(t)))
	rw := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/stream/events", controller.TailEvents).Methods("GET")
	router.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusBadRequest, rw.Code)
}

// TestTailEvents_UnavailableEmitsErrorEvent verifies that when the event log
// is unavailable the controller emits a synthetic SSE error event (not an HTTP error).
func TestTailEvents_UnavailableEmitsErrorEvent(t *testing.T) {
	t.Parallel()

	sessionID := uuid.New()
	runID := uuid.New()

	// Default fakeStreamCommands returns ErrRunEventLogUnavailable.
	controller := NewStreamController(&fakeStreamCommands{}, &fakeSessionQueries{}, nil)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf(
		"/stream/events?sessionId=%s&runId=%s", sessionID, runID,
	), nil)
	req = req.WithContext(composables.WithUser(req.Context(), testUser(t)))

	rw := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/stream/events", controller.TailEvents).Methods("GET")
	router.ServeHTTP(rw, req)

	require.Equal(t, http.StatusOK, rw.Code)
	body := rw.Body.String()
	assert.Contains(t, body, "event: error", "must emit SSE error event when log unavailable")
	assert.Contains(t, body, "unavailable", "error payload must mention unavailable")
}

