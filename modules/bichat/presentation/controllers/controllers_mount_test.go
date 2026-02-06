package controllers

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	bichatperm "github.com/iota-uz/iota-sdk/modules/bichat/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

type flusherRecorder struct {
	*httptest.ResponseRecorder
}

func (f flusherRecorder) Flush() {}

type stubChatService struct {
	getSession        func(ctx context.Context, sessionID uuid.UUID) (domain.Session, error)
	sendMessageStream func(ctx context.Context, req services.SendMessageRequest, onChunk func(services.StreamChunk)) error
}

func (s stubChatService) CreateSession(ctx context.Context, tenantID uuid.UUID, userID int64, title string) (domain.Session, error) {
	return nil, errors.New("not implemented")
}
func (s stubChatService) GetSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error) {
	if s.getSession == nil {
		return nil, errors.New("not implemented")
	}
	return s.getSession(ctx, sessionID)
}
func (s stubChatService) ListUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) ([]domain.Session, error) {
	return nil, errors.New("not implemented")
}
func (s stubChatService) UpdateSessionTitle(ctx context.Context, sessionID uuid.UUID, title string) (domain.Session, error) {
	return nil, errors.New("not implemented")
}
func (s stubChatService) ArchiveSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error) {
	return nil, errors.New("not implemented")
}
func (s stubChatService) UnarchiveSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error) {
	return nil, errors.New("not implemented")
}
func (s stubChatService) PinSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error) {
	return nil, errors.New("not implemented")
}
func (s stubChatService) UnpinSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error) {
	return nil, errors.New("not implemented")
}
func (s stubChatService) DeleteSession(ctx context.Context, sessionID uuid.UUID) error {
	return errors.New("not implemented")
}
func (s stubChatService) ClearSessionHistory(ctx context.Context, sessionID uuid.UUID) (services.ClearSessionHistoryResponse, error) {
	return services.ClearSessionHistoryResponse{}, errors.New("not implemented")
}
func (s stubChatService) CompactSessionHistory(ctx context.Context, sessionID uuid.UUID) (services.CompactSessionHistoryResponse, error) {
	return services.CompactSessionHistoryResponse{}, errors.New("not implemented")
}
func (s stubChatService) SendMessage(ctx context.Context, req services.SendMessageRequest) (*services.SendMessageResponse, error) {
	return nil, errors.New("not implemented")
}
func (s stubChatService) SendMessageStream(ctx context.Context, req services.SendMessageRequest, onChunk func(services.StreamChunk)) error {
	if s.sendMessageStream != nil {
		return s.sendMessageStream(ctx, req, onChunk)
	}
	return errors.New("not implemented")
}
func (s stubChatService) GetSessionMessages(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]types.Message, error) {
	return nil, errors.New("not implemented")
}
func (s stubChatService) ResumeWithAnswer(ctx context.Context, req services.ResumeRequest) (*services.SendMessageResponse, error) {
	return nil, errors.New("not implemented")
}
func (s stubChatService) CancelPendingQuestion(ctx context.Context, sessionID uuid.UUID) (domain.Session, error) {
	return nil, errors.New("not implemented")
}
func (s stubChatService) GenerateSessionTitle(ctx context.Context, sessionID uuid.UUID) error {
	return errors.New("not implemented")
}

func TestControllers_BasePathRouting(t *testing.T) {
	t.Parallel()

	app := application.New(&application.ApplicationOptions{
		Bundle:             application.LoadBundle(),
		SupportedLanguages: []string{"en"},
	})
	app.RegisterServices(&coreservices.TenantService{}, &coreservices.UploadService{})

	basePath := "/admin/ali/chat"

	r := mux.NewRouter()
	NewStreamController(app, nil, WithBasePath(basePath)).Register(r)
	NewChatController(app, nil, nil, nil, nil, nil, WithBasePath(basePath)).Register(r)

	var m mux.RouteMatch

	ok := r.Match(httptest.NewRequest(http.MethodPost, basePath+"/stream", nil), &m)
	if !ok {
		t.Fatalf("expected stream route to match under %q", basePath)
	}

	ok = r.Match(httptest.NewRequest(http.MethodGet, basePath+"/sessions", nil), &m)
	if !ok {
		t.Fatalf("expected sessions route to match under %q", basePath)
	}

	ok = r.Match(httptest.NewRequest(http.MethodPost, "/bi-chat/stream", nil), &m)
	if ok {
		t.Fatalf("expected stream route not to match under default base path")
	}

	ok = r.Match(httptest.NewRequest(http.MethodGet, "/bi-chat/sessions", nil), &m)
	if ok {
		t.Fatalf("expected sessions route not to match under default base path")
	}
}

func TestStreamController_RequireAccessPermission(t *testing.T) {
	t.Parallel()

	u := user.New(
		"Test",
		"User",
		internet.MustParseEmail("test@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	req := httptest.NewRequest(http.MethodPost, "/bi-chat/stream", bytes.NewBufferString(`{}`))
	req = req.WithContext(composables.WithUser(req.Context(), u))

	w := flusherRecorder{ResponseRecorder: httptest.NewRecorder()}

	c := NewStreamController(nil, nil, WithRequireAccessPermission(bichatperm.BiChatAccess))
	c.StreamMessage(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestChatController_RequireAccessPermission(t *testing.T) {
	t.Parallel()

	u := user.New(
		"Test",
		"User",
		internet.MustParseEmail("test@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	req := httptest.NewRequest(http.MethodGet, "/bi-chat/sessions/ignored", nil)
	req = req.WithContext(composables.WithUser(req.Context(), u))

	w := httptest.NewRecorder()

	c := NewChatController(nil, nil, nil, nil, nil, nil, WithRequireAccessPermission(bichatperm.BiChatAccess))
	c.GetSession(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestChatController_OwnershipVsReadAllPermission(t *testing.T) {
	t.Parallel()

	sessionID := uuid.New()

	svc := stubChatService{
		getSession: func(ctx context.Context, sessionID uuid.UUID) (domain.Session, error) {
			return domain.NewSession(
				domain.WithID(sessionID),
				domain.WithUserID(2),
				domain.WithTitle("x"),
			), nil
		},
	}

	u := user.New(
		"Test",
		"User",
		internet.MustParseEmail("test@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	c := NewChatController(nil, svc, nil, nil, nil, nil, WithReadAllPermission(bichatperm.BiChatExport))

	req := httptest.NewRequest(http.MethodGet, "/any", nil)
	req = mux.SetURLVars(req, map[string]string{"id": sessionID.String()})
	req = req.WithContext(composables.WithUser(req.Context(), u))

	w := httptest.NewRecorder()
	c.GetSession(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}

	u2 := u.AddPermission(bichatperm.BiChatExport)
	req2 := req.WithContext(composables.WithUser(req.Context(), u2))
	w2 := httptest.NewRecorder()
	c.GetSession(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w2.Code)
	}
}

func TestStreamController_DebugMode_ForbiddenWithoutExportPermission(t *testing.T) {
	t.Parallel()

	sessionID := uuid.New()
	body := `{"sessionId":"` + sessionID.String() + `","content":"hello","debugMode":true}`

	u := user.New(
		"Test",
		"User",
		internet.MustParseEmail("test@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	req := httptest.NewRequest(http.MethodPost, "/bi-chat/stream", bytes.NewBufferString(body))
	req = req.WithContext(composables.WithUser(req.Context(), u))

	w := flusherRecorder{ResponseRecorder: httptest.NewRecorder()}

	c := NewStreamController(nil, stubChatService{})
	c.StreamMessage(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestStreamController_DebugMode_AllowedWithExportPermission(t *testing.T) {
	t.Parallel()

	sessionID := uuid.New()
	body := `{"sessionId":"` + sessionID.String() + `","content":"hello","debugMode":true}`

	u := user.New(
		"Test",
		"User",
		internet.MustParseEmail("test@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	).AddPermission(bichatperm.BiChatExport)

	svc := stubChatService{
		getSession: func(ctx context.Context, id uuid.UUID) (domain.Session, error) {
			return domain.NewSession(
				domain.WithID(id),
				domain.WithUserID(1),
				domain.WithTitle("x"),
			), nil
		},
		sendMessageStream: func(ctx context.Context, req services.SendMessageRequest, onChunk func(services.StreamChunk)) error {
			onChunk(services.StreamChunk{Type: services.ChunkTypeDone})
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/bi-chat/stream", bytes.NewBufferString(body))
	req = req.WithContext(composables.WithUser(req.Context(), u))

	w := flusherRecorder{ResponseRecorder: httptest.NewRecorder()}

	c := NewStreamController(nil, svc)
	c.StreamMessage(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}
