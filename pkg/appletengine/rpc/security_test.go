package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/applets"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

type stubUser struct {
	id          uint
	permissions map[string]bool
}

func (u *stubUser) ID() uint                       { return u.id }
func (u *stubUser) DisplayName() string            { return "stub" }
func (u *stubUser) HasPermission(name string) bool { return u.permissions[name] }
func (u *stubUser) PermissionNames() []string      { return nil }

type stubHost struct {
	user     applets.AppletUser
	tenantID uuid.UUID
}

func (h *stubHost) ExtractUser(context.Context) (applets.AppletUser, error) {
	if h.user == nil {
		return nil, applets.ErrPermissionDenied
	}
	return h.user, nil
}

func (h *stubHost) ExtractTenantID(context.Context) (uuid.UUID, error) {
	if h.tenantID == uuid.Nil {
		return uuid.Nil, applets.ErrPermissionDenied
	}
	return h.tenantID, nil
}

func (h *stubHost) ExtractPool(context.Context) (*pgxpool.Pool, error) {
	return nil, applets.ErrInternal
}

func (h *stubHost) ExtractPageLocale(context.Context) language.Tag { return language.English }

type auditRecorder struct {
	events []AuditEvent
}

func (r *auditRecorder) RecordRPCMutation(event AuditEvent) {
	r.events = append(r.events, event)
}

type metricsRecorder struct {
	durations []map[string]string
	counters  []map[string]string
}

func (r *metricsRecorder) RecordDuration(_ string, _ time.Duration, labels map[string]string) {
	r.durations = append(r.durations, labels)
}

func (r *metricsRecorder) IncrementCounter(_ string, labels map[string]string) {
	r.counters = append(r.counters, labels)
}

func TestRegistry_PublicMethodWithoutPolicyFails(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	err := registry.RegisterPublic("bichat", "bichat.anon", applets.RPCMethod{Handler: dummyMethod().Handler}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no access policy")
}

func TestRegistry_TypedContractWithoutKindFails(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	err := registry.RegisterPublicContract("bichat", "bichat.anon", dummyMethod(), nil, Invalidates("other"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "explicit kind")
}

func TestRegistry_PublicOptInAllowsAnonymous(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	require.NoError(t, registry.RegisterPublicContract("bichat", "bichat.status", dummyMethod(), nil, Query(false, 0), Public()))
	method, ok := registry.Get("bichat.status")
	require.True(t, ok)
	assert.True(t, method.Contract.public)
}

func TestRegistry_RejectsCrossOwnerNamespace(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	require.NoError(t, registry.RegisterPublic("bichat", "bichat.ping", dummyMethod(), nil))

	err := registry.RegisterPublic("files", "bichat.upload", dummyMethod(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `namespace "bichat" is owned by applet "bichat"`)

	require.NoError(t, registry.RegisterPublic("bichat", "bichat.other", dummyMethod(), nil))
}

func TestDispatcher_IgnoresClientIdentityHeaders(t *testing.T) {
	t.Parallel()

	handlerCtx := &handlerBox{}
	registry := NewRegistry()
	require.NoError(t, registry.RegisterPublic("bichat", "bichat.echo", applets.RPCMethod{
		RequirePermissions: []string{"test.access"},
		Handler: func(c context.Context, _ json.RawMessage) (any, error) {
			handlerCtx.ctx = c
			return map[string]any{"ok": true}, nil
		},
	}, nil))
	dispatcher := NewDispatcher(registry, &stubHost{
		user:     &stubUser{id: 7, permissions: map[string]bool{"test.access": true}},
		tenantID: uuid.MustParse("00000000-0000-0000-0000-000000000042"),
	}, logrus.New())

	req := httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(`{"id":"1","method":"bichat.echo","params":{}}`))
	req.Header.Set("X-Iota-Tenant-Id", "spoofed-tenant")
	req.Header.Set("X-Iota-User-Id", "spoofed-user")
	rec := httptest.NewRecorder()
	dispatcher.HandlePublicHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	tenantID, hasTenant := TenantIDFromContext(handlerCtx.ctx)
	userID, hasUser := UserIDFromContext(handlerCtx.ctx)
	assert.True(t, hasTenant)
	assert.Equal(t, "00000000-0000-0000-0000-000000000042", tenantID, "tenant must come from the host, not the client header")
	assert.True(t, hasUser)
	assert.Equal(t, "7", userID, "user must come from the host, not the client header")
}

func TestDispatcher_RequestIDPassthroughAndGeneration(t *testing.T) {
	t.Parallel()

	var seen []string
	registry := NewRegistry()
	require.NoError(t, registry.RegisterPublic("bichat", "bichat.echo", applets.RPCMethod{
		RequirePermissions: []string{"test.access"},
		Handler: func(c context.Context, _ json.RawMessage) (any, error) {
			id, _ := RequestIDFromContext(c)
			seen = append(seen, id)
			return map[string]any{"ok": true}, nil
		},
	}, nil))
	dispatcher := NewDispatcher(registry, authorizedHost(), logrus.New())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(`{"id":"1","method":"bichat.echo","params":{}}`))
	req.Header.Set("X-Iota-Request-Id", "given-id")
	dispatcher.HandlePublicHTTP(rec, req)

	dispatcher.HandlePublicHTTP(rec, httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(`{"id":"2","method":"bichat.echo","params":{}}`)))

	require.Len(t, seen, 2)
	assert.Equal(t, "given-id", seen[0])
	assert.NotEmpty(t, seen[1])
	assert.NotEqual(t, "given-id", seen[1])
}

func TestDispatcher_LoginRedirectBecomesTypedUnauthorized(t *testing.T) {
	t.Parallel()

	redirect := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/login", http.StatusFound)
		})
	}
	registry := NewRegistry()
	require.NoError(t, registry.RegisterPublic("bichat", "bichat.echo", applets.RPCMethod{
		RequirePermissions: []string{"test.access"},
		Handler: func(_ context.Context, _ json.RawMessage) (any, error) {
			return map[string]any{"ok": true}, nil
		},
	}, []mux.MiddlewareFunc{redirect}))
	dispatcher := NewDispatcher(registry, nil, logrus.New())

	resp := doRPCRequest(t, dispatcher.HandlePublicHTTP, `{"id":"1","method":"bichat.echo","params":{}}`)
	decoded := decodeObject(t, resp.Body.Bytes())
	errorObj := decoded["error"].(map[string]any)
	assert.Equal(t, "unauthorized", errorObj["code"], "a login redirect must surface as typed unauthorized, not opaque -32603")
}

func TestDispatcher_AuditSinkReceivesMutationsOnly(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	require.NoError(t, registry.RegisterPublicContract("bichat", "bichat.query", dummyMethod(), nil, Query(false, 0)))
	require.NoError(t, registry.RegisterPublicContract("bichat", "bichat.save", dummyMethod(), nil, Mutation()))
	audit := &auditRecorder{}
	dispatcher := NewDispatcher(registry, &stubHost{
		user:     &stubUser{id: 7, permissions: map[string]bool{"test.access": true}},
		tenantID: uuid.MustParse("00000000-0000-0000-0000-000000000042"),
	}, logrus.New())
	dispatcher.SetAuditSink(audit)

	doRPCRequest(t, dispatcher.HandlePublicHTTP, `{"id":"1","method":"bichat.query","params":{}}`)
	doRPCRequest(t, dispatcher.HandlePublicHTTP, `{"id":"2","method":"bichat.save","params":{}}`)

	require.Len(t, audit.events, 1)
	assert.Equal(t, "bichat.save", audit.events[0].Method)
	assert.Equal(t, "bichat", audit.events[0].Applet)
	assert.True(t, audit.events[0].Authorized)
	assert.NotEmpty(t, audit.events[0].RequestID)
	assert.Equal(t, uint(7), audit.events[0].UserID)
	assert.Equal(t, "00000000-0000-0000-0000-000000000042", audit.events[0].TenantID)
}

func TestDispatcher_PermissionDeniedIsTypedForbidden(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	require.NoError(t, registry.RegisterPublicContract("bichat", "bichat.admin.purge", applets.RPCMethod{
		RequirePermissions: []string{"bichat.admin"},
		Handler: func(_ context.Context, _ json.RawMessage) (any, error) {
			return map[string]any{"purged": true}, nil
		},
	}, nil, Mutation()))
	audit := &auditRecorder{}
	dispatcher := NewDispatcher(registry, &stubHost{
		user:     &stubUser{id: 9, permissions: map[string]bool{"bichat.read": true}},
		tenantID: uuid.MustParse("00000000-0000-0000-0000-000000000043"),
	}, logrus.New())
	dispatcher.SetAuditSink(audit)

	resp := doRPCRequest(t, dispatcher.HandlePublicHTTP, `{"id":"1","method":"bichat.admin.purge","params":{}}`)
	decoded := decodeObject(t, resp.Body.Bytes())
	errorObj := decoded["error"].(map[string]any)
	assert.Equal(t, "forbidden", errorObj["code"])
	assert.Contains(t, errorObj["message"], "permission denied")

	require.Len(t, audit.events, 1)
	assert.False(t, audit.events[0].Authorized, "a rejected mutation must be audited as unauthorized")
	assert.Equal(t, "forbidden", audit.events[0].ErrCode)
	assert.Equal(t, uint(9), audit.events[0].UserID)
}

func TestDispatcher_MetricsCarryMethodKindAndCode(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	require.NoError(t, registry.RegisterPublicContract("bichat", "bichat.query", dummyMethod(), nil, Query(false, 0)))
	require.NoError(t, registry.RegisterPublicContract("bichat", "bichat.failing", applets.RPCMethod{
		RequirePermissions: []string{"test.access"},
		Handler: func(_ context.Context, _ json.RawMessage) (any, error) {
			return nil, applets.ErrNotFound
		},
	}, nil, Mutation()))
	metrics := &metricsRecorder{}
	dispatcher := NewDispatcher(registry, authorizedHost(), logrus.New())
	dispatcher.SetMetricsRecorder(metrics)

	doRPCRequest(t, dispatcher.HandlePublicHTTP, `{"id":"1","method":"bichat.query","params":{}}`)
	doRPCRequest(t, dispatcher.HandlePublicHTTP, `{"id":"2","method":"bichat.failing","params":{}}`)

	require.Len(t, metrics.counters, 2)
	assert.Equal(t, "query", metrics.counters[0]["kind"])
	assert.Empty(t, metrics.counters[0]["code"])
	assert.Equal(t, "mutation", metrics.counters[1]["kind"])
	assert.Equal(t, "not_found", metrics.counters[1]["code"])
}

func TestDispatcher_BunForwardingUsesTrustedIdentity(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	require.NoError(t, registry.RegisterPublicWithTarget("bichat", "bichat.ping", MethodTargetBun, applets.RPCMethod{
		RequirePermissions: []string{"test.access"},
		Handler: func(_ context.Context, _ json.RawMessage) (any, error) {
			return map[string]any{"ok": true}, nil
		},
	}, nil))
	dispatcher := NewDispatcher(registry, &stubHost{
		user:     &stubUser{id: 7, permissions: map[string]bool{"test.access": true}},
		tenantID: uuid.MustParse("00000000-0000-0000-0000-000000000042"),
	}, logrus.New())
	bunCaller := &bunCallerStub{}
	dispatcher.SetBunPublicCaller(bunCaller)

	req := httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(`{"id":"1","method":"bichat.ping","params":{}}`))
	req.Header.Set("X-Iota-Tenant-Id", "spoofed")
	req.Header.Set("X-Iota-User-Id", "spoofed")
	req.Header.Set("X-Keep", "yes")
	rec := httptest.NewRecorder()
	dispatcher.HandlePublicHTTP(rec, req)

	require.True(t, bunCaller.called)
	assert.Equal(t, "00000000-0000-0000-0000-000000000042", bunCaller.lastHeaders.Get("X-Iota-Tenant-Id"), "the forwarded tenant must come from the host, not the client header")
	assert.Equal(t, "7", bunCaller.lastHeaders.Get("X-Iota-User-Id"))
	assert.Equal(t, "yes", bunCaller.lastHeaders.Get("X-Keep"))
}

type handlerBox struct {
	ctx context.Context
}

func authorizedHost() applets.HostServices {
	return &stubHost{user: &stubUser{id: 1, permissions: map[string]bool{"test.access": true}}}
}
