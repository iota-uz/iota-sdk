package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	bichatpersistence "github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/persistence"
	bichatperm "github.com/iota-uz/iota-sdk/modules/bichat/permissions"
	modservices "github.com/iota-uz/iota-sdk/modules/bichat/services"
	coreuser "github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	coreupload "github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	bichatagents "github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatctx "github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context/renderers"
	bichatdomain "github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichathooks "github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
	pkgservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/storage"
	bichattypes "github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type flusherRecorder struct {
	*httptest.ResponseRecorder
}

func (f flusherRecorder) Flush() {}

type deterministicModel struct {
	content string
}

func (m *deterministicModel) Generate(ctx context.Context, req bichatagents.Request, opts ...bichatagents.GenerateOption) (*bichatagents.Response, error) {
	return &bichatagents.Response{
		Message: bichattypes.AssistantMessage(m.content),
		Usage:   bichattypes.TokenUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		// "stop" is treated as completion without tools.
		FinishReason:       "stop",
		ProviderResponseID: "resp_test",
	}, nil
}

func (m *deterministicModel) Stream(ctx context.Context, req bichatagents.Request, opts ...bichatagents.GenerateOption) (bichattypes.Generator[bichatagents.Chunk], error) {
	return bichattypes.NewGenerator(ctx, func(ctx context.Context, yield func(bichatagents.Chunk) bool) error {
		if m.content != "" {
			if !yield(bichatagents.Chunk{Delta: m.content}) {
				return nil
			}
		}
		yield(bichatagents.Chunk{
			Done:               true,
			FinishReason:       "stop",
			ProviderResponseID: "resp_test",
			Usage:              &bichattypes.TokenUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		})
		return nil
	}), nil
}

func (m *deterministicModel) Info() bichatagents.ModelInfo {
	return bichatagents.ModelInfo{
		Name:     "deterministic-test-model",
		Provider: "test",
		Capabilities: []bichatagents.Capability{
			bichatagents.CapabilityStreaming,
			bichatagents.CapabilityTools,
		},
	}
}

func (m *deterministicModel) HasCapability(capability bichatagents.Capability) bool {
	for _, c := range m.Info().Capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

func (m *deterministicModel) Pricing() bichatagents.ModelPricing { return bichatagents.ModelPricing{} }

type controllerDeps struct {
	chatRepo          bichatdomain.ChatRepository
	agentService      pkgservices.AgentService
	chatService       pkgservices.ChatService
	attachmentService pkgservices.AttachmentService
}

func newControllerDeps(t *testing.T) controllerDeps {
	t.Helper()

	chatRepo := bichatpersistence.NewPostgresChatRepository()

	agent := bichatagents.NewBaseAgent(
		bichatagents.WithName("test_agent"),
		bichatagents.WithDescription("Deterministic agent for controller integration tests"),
		bichatagents.WithSystemPrompt("You are a deterministic test agent."),
	)
	model := &deterministicModel{content: "ok"}

	agentService := modservices.NewAgentService(modservices.AgentServiceConfig{
		Agent:        agent,
		Model:        model,
		Policy:       bichatctx.DefaultPolicy(),
		Renderer:     renderers.NewAnthropicRenderer(),
		Checkpointer: bichatagents.NewInMemoryCheckpointer(),
		EventBus:     bichathooks.NewEventBus(),
		ChatRepo:     chatRepo,
	})

	chatService := modservices.NewChatService(chatRepo, agentService, model, nil, nil)
	attachmentService := modservices.NewAttachmentService(storage.NewNoOpFileStorage())

	return controllerDeps{
		chatRepo:          chatRepo,
		agentService:      agentService,
		chatService:       chatService,
		attachmentService: attachmentService,
	}
}

func newRouterWithContext(t *testing.T, env *itf.TestEnvironment, u coreuser.User) *mux.Router {
	t.Helper()

	r := mux.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			ctx = composables.WithPool(ctx, env.Pool)
			ctx = composables.WithTx(ctx, env.Tx)
			ctx = composables.WithTenantID(ctx, env.Tenant.ID)
			ctx = composables.WithUser(ctx, u)
			ctx = context.WithValue(ctx, constants.AppKey, env.App)

			params := &composables.Params{
				IP:            "127.0.0.1",
				UserAgent:     "test-agent",
				Authenticated: true,
				Request:       req,
				Writer:        w,
			}
			ctx = composables.WithParams(ctx, params)

			logger := logrus.New().WithFields(logrus.Fields{"test": true, "path": req.URL.Path})
			ctx = context.WithValue(ctx, constants.LoggerKey, logger)

			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	return r
}

func mustCreateSession(t *testing.T, ctx context.Context, deps controllerDeps, tenantID uuid.UUID, u coreuser.User, title string) bichatdomain.Session {
	t.Helper()

	s, err := deps.chatService.CreateSession(ctx, tenantID, int64(u.ID()), title)
	require.NoError(t, err)
	require.NotNil(t, s)
	return s
}

func mustCreateControllerUpload(t *testing.T, ctx context.Context, fileName, mimeType string, size int) int64 {
	t.Helper()

	mime := mimetype.Lookup(mimeType)
	if mime == nil {
		mime = mimetype.Lookup("application/octet-stream")
	}

	uploadRepo := corepersistence.NewUploadRepository()
	seed := uuid.NewString()[:8]
	entity := coreupload.New(
		"bichat-controller-hash-"+seed,
		"uploads/bichat-controller-"+seed,
		fileName,
		"bichat-controller-"+seed,
		size,
		mime,
	)

	created, err := uploadRepo.Create(ctx, entity)
	require.NoError(t, err)
	return int64(created.ID())
}

func TestControllers_BasePathRouting_Integration(t *testing.T) {
	t.Parallel()

	env := setupControllerTest(t)
	u := createCoreUser(t, env, "bichat-controllers-route@example.com").AddPermission(bichatperm.BiChatAccess)
	deps := newControllerDeps(t)

	basePath := "/admin/ali/chat"

	r := newRouterWithContext(t, env, u)
	NewChatController(env.App, deps.chatService, deps.chatRepo, deps.agentService, nil, nil,
		WithBasePath(basePath),
		WithRequireAccessPermission(bichatperm.BiChatAccess),
	).Register(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, basePath+"/sessions", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/bi-chat/sessions", nil)
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusNotFound, w2.Code, w2.Body.String())
}

func TestStreamController_RequireAccessPermission_Integration(t *testing.T) {
	t.Parallel()

	env := setupControllerTest(t)
	u := createCoreUser(t, env, "bichat-controllers-access@example.com") // no BiChatAccess
	deps := newControllerDeps(t)

	r := newRouterWithContext(t, env, u)
	NewStreamController(env.App, deps.chatService, deps.attachmentService, WithRequireAccessPermission(bichatperm.BiChatAccess)).Register(r)

	w := flusherRecorder{ResponseRecorder: httptest.NewRecorder()}
	req := httptest.NewRequest(http.MethodPost, "/bi-chat/stream", bytes.NewBufferString(`{}`))
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}

func TestChatController_OwnershipVsReadAllPermission_Integration(t *testing.T) {
	t.Parallel()

	env := setupControllerTest(t)
	u1 := createCoreUser(t, env, "bichat-controllers-owner1@example.com").AddPermission(bichatperm.BiChatAccess)
	u2 := createCoreUser(t, env, "bichat-controllers-owner2@example.com").AddPermission(bichatperm.BiChatAccess)

	deps := newControllerDeps(t)
	session := mustCreateSession(t, env.Ctx, deps, env.Tenant.ID, u2, "owned by u2")

	r := newRouterWithContext(t, env, u1)
	NewChatController(env.App, deps.chatService, deps.chatRepo, deps.agentService, nil, nil,
		WithRequireAccessPermission(bichatperm.BiChatAccess),
		WithReadAllPermission(bichatperm.BiChatExport),
	).Register(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/bi-chat/sessions/"+session.ID().String(), nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	u1ReadAll := u1.AddPermission(bichatperm.BiChatExport)
	r2 := newRouterWithContext(t, env, u1ReadAll)
	NewChatController(env.App, deps.chatService, deps.chatRepo, deps.agentService, nil, nil,
		WithRequireAccessPermission(bichatperm.BiChatAccess),
		WithReadAllPermission(bichatperm.BiChatExport),
	).Register(r2)

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/bi-chat/sessions/"+session.ID().String(), nil)
	r2.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code, w2.Body.String())
	require.Contains(t, w2.Body.String(), session.ID().String())
}

func TestStreamController_DebugMode_AllowedWithoutExtraPermission_Integration(t *testing.T) {
	t.Parallel()

	env := setupControllerTest(t)
	u := createCoreUser(t, env, "bichat-controllers-debug-noexport@example.com").
		AddPermission(bichatperm.BiChatAccess)

	deps := newControllerDeps(t)
	session := mustCreateSession(t, env.Ctx, deps, env.Tenant.ID, u, "s")

	r := newRouterWithContext(t, env, u)
	NewStreamController(env.App, deps.chatService,
		deps.attachmentService,
		WithRequireAccessPermission(bichatperm.BiChatAccess),
	).Register(r)

	body := map[string]any{
		"sessionId": session.ID().String(),
		"content":   "hello",
		"debugMode": true,
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	w := flusherRecorder{ResponseRecorder: httptest.NewRecorder()}
	req := httptest.NewRequest(http.MethodPost, "/bi-chat/stream", bytes.NewReader(data))
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
	require.Contains(t, w.Body.String(), `"type":"done"`)
}

func TestStreamController_DebugMode_AllowedWithExportPermission_Integration(t *testing.T) {
	t.Parallel()

	env := setupControllerTest(t)
	u := createCoreUser(t, env, "bichat-controllers-debug-export@example.com").
		AddPermission(bichatperm.BiChatAccess).
		AddPermission(bichatperm.BiChatExport)

	deps := newControllerDeps(t)
	session := mustCreateSession(t, env.Ctx, deps, env.Tenant.ID, u, "s")

	r := newRouterWithContext(t, env, u)
	NewStreamController(env.App, deps.chatService,
		deps.attachmentService,
		WithRequireAccessPermission(bichatperm.BiChatAccess),
	).Register(r)

	body := map[string]any{
		"sessionId": session.ID().String(),
		"content":   "hello",
		"debugMode": true,
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	w := flusherRecorder{ResponseRecorder: httptest.NewRecorder()}
	req := httptest.NewRequest(http.MethodPost, "/bi-chat/stream", bytes.NewReader(data))
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
	require.Contains(t, w.Body.String(), `"type":"done"`)
}

func TestStreamController_ReplaceFromMessageID_TruncatesHistory_Integration(t *testing.T) {
	t.Parallel()

	env := setupControllerTest(t)
	u := createCoreUser(t, env, "bichat-controllers-replace@example.com").
		AddPermission(bichatperm.BiChatAccess)

	deps := newControllerDeps(t)
	session := mustCreateSession(t, env.Ctx, deps, env.Tenant.ID, u, "s")

	r := newRouterWithContext(t, env, u)
	NewStreamController(env.App, deps.chatService,
		deps.attachmentService,
		WithRequireAccessPermission(bichatperm.BiChatAccess),
	).Register(r)

	first := map[string]any{
		"sessionId": session.ID().String(),
		"content":   "first prompt",
	}
	firstData, err := json.Marshal(first)
	require.NoError(t, err)

	w1 := flusherRecorder{ResponseRecorder: httptest.NewRecorder()}
	req1 := httptest.NewRequest(http.MethodPost, "/bi-chat/stream", bytes.NewReader(firstData))
	r.ServeHTTP(w1, req1)
	require.Equal(t, http.StatusOK, w1.Code, w1.Body.String())

	msgs, err := deps.chatRepo.GetSessionMessages(env.Ctx, session.ID(), bichatdomain.ListOptions{Limit: 100, Offset: 0})
	require.NoError(t, err)
	require.NotEmpty(t, msgs)

	var replaceFrom uuid.UUID
	for _, m := range msgs {
		if m.Role() == bichattypes.RoleUser {
			replaceFrom = m.ID()
			break
		}
	}
	require.NotEqual(t, uuid.Nil, replaceFrom)

	second := map[string]any{
		"sessionId":            session.ID().String(),
		"content":              "updated prompt",
		"replaceFromMessageId": replaceFrom.String(),
	}
	secondData, err := json.Marshal(second)
	require.NoError(t, err)

	w2 := flusherRecorder{ResponseRecorder: httptest.NewRecorder()}
	req2 := httptest.NewRequest(http.MethodPost, "/bi-chat/stream", bytes.NewReader(secondData))
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code, w2.Body.String())

	msgs2, err := deps.chatRepo.GetSessionMessages(env.Ctx, session.ID(), bichatdomain.ListOptions{Limit: 100, Offset: 0})
	require.NoError(t, err)
	require.Len(t, msgs2, 2)

	var userContent string
	for _, m := range msgs2 {
		if m.Role() == bichattypes.RoleUser {
			userContent = m.Content()
		}
	}
	require.Equal(t, "updated prompt", userContent)
}

func TestStreamController_AttachmentUpload_PersistsOnUserMessage_Integration(t *testing.T) {
	t.Parallel()

	env := setupControllerTest(t)
	u := createCoreUser(t, env, "bichat-controllers-attachments@example.com").
		AddPermission(bichatperm.BiChatAccess)

	deps := newControllerDeps(t)
	session := mustCreateSession(t, env.Ctx, deps, env.Tenant.ID, u, "attachments")

	r := newRouterWithContext(t, env, u)
	NewStreamController(env.App, deps.chatService,
		deps.attachmentService,
		WithRequireAccessPermission(bichatperm.BiChatAccess),
	).Register(r)

	uploadID := mustCreateControllerUpload(t, env.Ctx, "notes.txt", "text/plain", len("hello,artifact-reader"))
	body := map[string]any{
		"sessionId": session.ID().String(),
		"content":   "Analyze this file",
		"attachments": []map[string]any{
			{
				"uploadId": uploadID,
			},
		},
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	w := flusherRecorder{ResponseRecorder: httptest.NewRecorder()}
	req := httptest.NewRequest(http.MethodPost, "/bi-chat/stream", bytes.NewReader(data))
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	msgs, err := deps.chatRepo.GetSessionMessages(env.Ctx, session.ID(), bichatdomain.ListOptions{Limit: 20, Offset: 0})
	require.NoError(t, err)
	require.NotEmpty(t, msgs)

	var userMsg bichattypes.Message
	for _, msg := range msgs {
		if msg.Role() == bichattypes.RoleUser {
			userMsg = msg
			break
		}
	}
	require.NotNil(t, userMsg)
	require.Len(t, userMsg.Attachments(), 1)
	assert.Equal(t, "notes.txt", userMsg.Attachments()[0].FileName)
	assert.Equal(t, "text/plain", userMsg.Attachments()[0].MimeType)
	assert.NotEmpty(t, userMsg.Attachments()[0].FilePath)
}

func TestStreamController_Stop_Returns200_Integration(t *testing.T) {
	t.Parallel()

	env := setupControllerTest(t)
	u := createCoreUser(t, env, "bichat-controllers-stop@example.com").
		AddPermission(bichatperm.BiChatAccess)

	deps := newControllerDeps(t)
	session := mustCreateSession(t, env.Ctx, deps, env.Tenant.ID, u, "stop test")

	r := newRouterWithContext(t, env, u)
	NewStreamController(env.App, deps.chatService,
		deps.attachmentService,
		WithRequireAccessPermission(bichatperm.BiChatAccess),
	).Register(r)

	body := map[string]any{"sessionId": session.ID().String()}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/bi-chat/stream/stop", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestStreamController_Stop_Returns400_WhenSessionIDMissing(t *testing.T) {
	t.Parallel()

	env := setupControllerTest(t)
	u := createCoreUser(t, env, "bichat-controllers-stop-bad@example.com").
		AddPermission(bichatperm.BiChatAccess)

	deps := newControllerDeps(t)
	r := newRouterWithContext(t, env, u)
	NewStreamController(env.App, deps.chatService,
		deps.attachmentService,
		WithRequireAccessPermission(bichatperm.BiChatAccess),
	).Register(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/bi-chat/stream/stop", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "sessionId")
}
