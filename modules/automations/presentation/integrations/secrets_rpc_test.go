package integrations

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/automations/permissions"
	"github.com/iota-uz/iota-sdk/modules/automations/services"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/automation/secrets"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/listext"
)

const secretValue = "sk-live-never-echoed"

// listOnly is a Service that lists no connectors; the secrets methods never call the Service.
type listOnly struct {
	Service
	listed int
}

func (s *listOnly) List(context.Context, services.Actor, services.ListOptions) ([]services.ConnectorEntry, error) {
	s.listed++
	return nil, nil
}

// memSecrets is an in-memory secrets.Manager that keeps the values it was given apart from the records.
type memSecrets struct {
	mu      sync.Mutex
	records map[uuid.UUID]secrets.Secret
	values  map[uuid.UUID]string
	fail    error
}

func newMemSecrets() *memSecrets {
	return &memSecrets{records: map[uuid.UUID]secrets.Secret{}, values: map[uuid.UUID]string{}}
}

func (m *memSecrets) List(_ context.Context, tenant uuid.UUID) ([]secrets.Secret, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail != nil {
		return nil, m.fail
	}
	out := []secrets.Secret{}
	for _, s := range m.records {
		if s.TenantID == tenant {
			out = append(out, s)
		}
	}
	return out, nil
}

func (m *memSecrets) Create(_ context.Context, tenant uuid.UUID, in secrets.CreateInput) (secrets.Secret, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail != nil {
		return secrets.Secret{}, m.fail
	}
	for _, s := range m.records {
		if s.TenantID == tenant && s.Name == in.Name {
			return secrets.Secret{}, secrets.ErrNameTaken
		}
	}
	now := time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)
	s := secrets.Secret{ID: uuid.New(), TenantID: tenant, Name: in.Name, Description: in.Description, Version: 1, ValueVersion: 1, CreatedBy: in.CreatedBy, CreatedAt: now, UpdatedAt: now, RotatedAt: now}
	m.records[s.ID], m.values[s.ID] = s, in.Value
	return s, nil
}

func (m *memSecrets) get(tenant, id uuid.UUID, version int) (secrets.Secret, error) {
	s, ok := m.records[id]
	if !ok || s.TenantID != tenant {
		return secrets.Secret{}, secrets.ErrNotFound
	}
	if s.Version != version {
		return secrets.Secret{}, secrets.ErrVersionConflict
	}
	return s, nil
}

func (m *memSecrets) Update(_ context.Context, tenant uuid.UUID, in secrets.UpdateInput) (secrets.Secret, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, err := m.get(tenant, in.ID, in.Version)
	if err != nil {
		return secrets.Secret{}, err
	}
	if in.Name != nil {
		s.Name = *in.Name
	}
	if in.Description != nil {
		s.Description = *in.Description
	}
	s.Version++
	m.records[s.ID] = s
	return s, nil
}

func (m *memSecrets) Rotate(_ context.Context, tenant uuid.UUID, in secrets.RotateInput) (secrets.Secret, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, err := m.get(tenant, in.ID, in.Version)
	if err != nil {
		return secrets.Secret{}, err
	}
	s.Version++
	s.ValueVersion++
	s.RotatedAt = s.RotatedAt.Add(time.Hour)
	m.records[s.ID], m.values[s.ID] = s, in.Value
	return s, nil
}

func (m *memSecrets) Delete(_ context.Context, tenant, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.records[id]
	if !ok || s.TenantID != tenant {
		return secrets.ErrNotFound
	}
	delete(m.records, id)
	delete(m.values, id)
	return nil
}

func newSecretsFixture(t *testing.T, mgr secrets.Manager) (*fixture, *listOnly) {
	t.Helper()
	svc := &listOnly{}
	ctrl, err := NewController(Config{Service: svc, Configured: true, Secrets: mgr})
	require.NoError(t, err)
	return &fixture{t: t, ctrl: ctrl, tenant: uuid.New(), other: uuid.New()}, svc
}

func TestSecretsRPCNeverReturnsValues(t *testing.T) {
	mgr := newMemSecrets()
	f, _ := newSecretsFixture(t, mgr)
	admin := testUser(f.tenant, permissions.SecretsManage)

	_, body := f.call(admin, MethodSecretsCreate, map[string]any{"name": "API_KEY", "description": "CRM", "value": secretValue})
	created := result[SecretSaveResponse](t, body)
	require.True(t, created.OK, body)
	assert.NotContains(t, body, secretValue)
	assert.Equal(t, "API_KEY", created.Secret.Name)
	assert.Equal(t, created.Secret.CreatedAt, created.Secret.RotatedAt)
	assert.Nil(t, created.Secret.LastUsedAt)
	id := uuid.MustParse(created.Secret.ID)
	assert.Equal(t, secretValue, mgr.values[id], "the value reaches the manager")
	assert.Equal(t, int64(7), mgr.records[id].CreatedBy)

	_, body = f.call(admin, MethodSecretsList, map[string]any{})
	list := result[SecretListResponse](t, body)
	assert.NotContains(t, body, secretValue)
	require.Len(t, list.Secrets, 1)

	_, body = f.call(admin, MethodSecretsRotate, map[string]any{"id": created.Secret.ID, "version": 1, "value": "rotated-" + secretValue})
	rotated := result[SecretSaveResponse](t, body)
	require.True(t, rotated.OK, body)
	assert.NotContains(t, body, secretValue)
	assert.Equal(t, 2, rotated.Secret.Version)
	assert.NotEqual(t, rotated.Secret.CreatedAt, rotated.Secret.RotatedAt)

	desc := "CRM key"
	_, body = f.call(admin, MethodSecretsUpdate, map[string]any{"id": created.Secret.ID, "version": 2, "name": "CRM_KEY", "description": desc})
	updated := result[SecretSaveResponse](t, body)
	require.True(t, updated.OK, body)
	assert.Equal(t, "CRM_KEY", updated.Secret.Name)
	assert.Equal(t, desc, updated.Secret.Description)
	assert.Equal(t, "rotated-"+secretValue, mgr.values[id], "an update keeps the value")

	_, body = f.call(admin, MethodSecretsDelete, map[string]any{"id": created.Secret.ID})
	assert.True(t, result[OKResponse](t, body).OK)
	_, body = f.call(admin, MethodSecretsDelete, map[string]any{"id": created.Secret.ID})
	assert.Contains(t, body, `"code":"not_found"`)
}

func TestSecretsRPCRefusals(t *testing.T) {
	mgr := newMemSecrets()
	f, _ := newSecretsFixture(t, mgr)
	admin := testUser(f.tenant, permissions.SecretsManage)

	_, body := f.call(admin, MethodSecretsCreate, map[string]any{"name": "api_key", "value": ""})
	invalid := result[SecretSaveResponse](t, body)
	assert.Equal(t, ErrInvalidInput, invalid.Error)
	assert.Equal(t, map[string]string{"name": "secret_name", "value": "required"}, invalid.Fields)
	assert.Empty(t, mgr.records, "a refused request reaches no store")

	_, body = f.call(admin, MethodSecretsCreate, map[string]any{"name": "API_KEY", "value": secretValue})
	created := result[SecretSaveResponse](t, body)
	require.True(t, created.OK, body)
	_, body = f.call(admin, MethodSecretsCreate, map[string]any{"name": "API_KEY", "value": secretValue})
	taken := result[SecretSaveResponse](t, body)
	assert.Equal(t, ErrNameTaken, taken.Error)
	assert.Equal(t, map[string]string{"name": "taken"}, taken.Fields)
	assert.NotContains(t, body, secretValue)

	_, body = f.call(admin, MethodSecretsRotate, map[string]any{"id": created.Secret.ID, "version": 9, "value": "x"})
	assert.Equal(t, ErrVersionConflict, result[SecretSaveResponse](t, body).Error)
	_, body = f.call(admin, MethodSecretsRotate, map[string]any{"id": created.Secret.ID, "version": 1, "value": ""})
	assert.Equal(t, map[string]string{"value": "required"}, result[SecretSaveResponse](t, body).Fields)

	// Another tenant's administrator, a malformed id: not found.
	stranger := testUser(f.other, permissions.SecretsManage)
	_, body = f.call(stranger, MethodSecretsUpdate, map[string]any{"id": created.Secret.ID, "version": 1, "description": "x"})
	assert.Contains(t, body, `"code":"not_found"`)
	_, body = f.call(admin, MethodSecretsDelete, map[string]any{"id": "nope"})
	assert.Contains(t, body, `"code":"not_found"`)

	// The store is down: a save says so, the list is an RPC error; neither carries a value.
	mgr.fail = fmt.Errorf("vault: %w", secrets.ErrUnavailable)
	_, body = f.call(admin, MethodSecretsCreate, map[string]any{"name": "OTHER", "value": secretValue})
	assert.Equal(t, ErrUnavailable, result[SecretSaveResponse](t, body).Error)
	assert.NotContains(t, body, secretValue)
	_, body = f.call(admin, MethodSecretsList, map[string]any{})
	assert.Contains(t, body, `"code":"unavailable"`)

	// An ErrInvalid of the manager names its field.
	assert.Equal(t, map[string]string{"description": "invalid"}, invalidSecretField(fmt.Errorf("%w: description is longer than 500 characters", secrets.ErrInvalid)))
	assert.Nil(t, invalidSecretField(secrets.ErrInvalid))
}

func TestSecretsRPCPermissions(t *testing.T) {
	f, svc := newSecretsFixture(t, newMemSecrets())
	connections := testUser(f.tenant, permissions.ConnectionsManage)
	secretsAdmin := testUser(f.tenant, permissions.SecretsManage)
	both := testUser(f.tenant, permissions.ConnectionsManage, permissions.SecretsManage)

	// Connections.Manage does not manage secrets: the page hides them and every method refuses.
	_, body := f.call(connections, MethodList, map[string]any{})
	assert.False(t, result[ListResponse](t, body).CanManageSecrets)
	for _, m := range []string{MethodSecretsList, MethodSecretsCreate, MethodSecretsUpdate, MethodSecretsRotate, MethodSecretsDelete} {
		code, body := f.call(connections, m, map[string]any{"id": uuid.NewString(), "name": "API_KEY", "value": secretValue})
		assert.Equal(t, http.StatusForbidden, code, m)
		assert.Contains(t, body, `"code":"forbidden"`, m)
		assert.NotContains(t, body, secretValue, m)
	}

	// Secrets.Manage alone opens the page with the secrets and no connectors (the service is not asked).
	listed := svc.listed
	_, body = f.call(secretsAdmin, MethodList, map[string]any{})
	only := result[ListResponse](t, body)
	assert.True(t, only.CanManageSecrets)
	assert.False(t, only.CanManage)
	assert.Empty(t, only.Connectors)
	assert.Equal(t, listed, svc.listed)
	for _, m := range []string{MethodCreate, MethodUpdate, MethodDelete, MethodTest} {
		code, _ := f.call(secretsAdmin, m, map[string]any{"id": uuid.NewString(), "kind": "telegram_bot"})
		assert.Equal(t, http.StatusForbidden, code, "Secrets.Manage does not manage connections: "+m)
	}

	_, body = f.call(both, MethodList, map[string]any{})
	full := result[ListResponse](t, body)
	assert.True(t, full.CanManage)
	assert.True(t, full.CanManageSecrets)

	// The page admits either permission; the OAuth2 callback still requires Connections.Manage.
	page := requirePage(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	serve := func(h http.Handler, u user.User) int {
		req := httptest.NewRequest(http.MethodGet, "/settings/integrations", nil)
		req = req.WithContext(composables.WithUser(req.Context(), u))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	assert.Equal(t, http.StatusNoContent, serve(page, secretsAdmin))
	assert.Equal(t, http.StatusNoContent, serve(page, connections))
	subject := func(u user.User) spi.Subject {
		sub, err := listext.SubjectFromContext(composables.WithUser(context.Background(), u))
		require.NoError(t, err)
		return sub
	}
	pagePerms := []string{permissions.ConnectionsManage.Name(), permissions.SecretsManage.Name()}
	assert.False(t, holdsAny(subject(testUser(f.tenant, permissions.AutomationsAuthor)), pagePerms...))
	assert.False(t, holdsAny(subject(secretsAdmin), permissions.ConnectionsManage.Name()), "the OAuth2 callback is the connections administrator's")
}

func TestSecretsRPCWithoutManager(t *testing.T) {
	f, _ := newSecretsFixture(t, nil)
	admin := testUser(f.tenant, permissions.ConnectionsManage, permissions.SecretsManage)
	_, body := f.call(admin, MethodList, map[string]any{})
	assert.False(t, result[ListResponse](t, body).CanManageSecrets, "no manager: no secrets section")
	_, body = f.call(admin, MethodSecretsList, map[string]any{})
	assert.Contains(t, body, `"code":"unavailable"`)
	_, body = f.call(admin, MethodSecretsCreate, map[string]any{"name": "API_KEY", "value": secretValue})
	assert.Contains(t, body, `"code":"unavailable"`)
	assert.NotContains(t, body, secretValue)
}
