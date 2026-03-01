package subscription_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/subscription"
	subtesting "github.com/iota-uz/iota-sdk/pkg/subscription/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequireFeature_Denied(t *testing.T) {
	t.Parallel()

	mockSvc := subtesting.NewMockEngine()
	mockSvc.SetFeature("shyona_access", false)

	mw := subscription.RequireFeature(mockSvc, "shyona_access")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req = req.WithContext(composables.WithTenantID(req.Context(), uuid.New()))
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	assert.Equal(t, http.StatusForbidden, res.Code)
}

func TestRequireFeature_Allowed(t *testing.T) {
	t.Parallel()

	mockSvc := subtesting.NewMockEngine()
	mockSvc.SetFeature("shyona_access", true)

	mw := subscription.RequireFeature(mockSvc, "shyona_access")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req = req.WithContext(composables.WithTenantID(req.Context(), uuid.New()))
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	assert.Equal(t, http.StatusOK, res.Code)
}

func TestRequireFeature_HTMXTrigger(t *testing.T) {
	t.Parallel()

	mockSvc := subtesting.NewMockEngine()
	mockSvc.SetFeature("export_pdf", false)

	mw := subscription.RequireFeature(mockSvc, "export_pdf")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req = req.WithContext(composables.WithTenantID(req.Context(), uuid.New()))
	req.Header.Set("Hx-Request", "true")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	require.Equal(t, http.StatusForbidden, res.Code)
	assert.NotEmpty(t, res.Header().Get("Hx-Trigger"))
}

func TestEnforceLimit_Denied(t *testing.T) {
	t.Parallel()

	mockSvc := subtesting.NewMockEngine()
	mockSvc.SetLimit("drivers", 1)
	mockSvc.SetCurrentCount("drivers", 1)

	mw := subscription.EnforceLimit(mockSvc, "drivers")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/drivers", nil)
	req = req.WithContext(composables.WithTenantID(req.Context(), uuid.New()))
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	assert.Equal(t, http.StatusTooManyRequests, res.Code)
}

func TestEnforceLimit_Allowed(t *testing.T) {
	t.Parallel()

	mockSvc := subtesting.NewMockEngine()
	mockSvc.SetLimit("drivers", 5)
	mockSvc.SetCurrentCount("drivers", 1)

	mw := subscription.EnforceLimit(mockSvc, "drivers")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/drivers", nil)
	req = req.WithContext(composables.WithTenantID(req.Context(), uuid.New()))
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	assert.Equal(t, http.StatusOK, res.Code)
}

func TestRequirePlan_Denied(t *testing.T) {
	t.Parallel()

	mockSvc := subtesting.NewMockEngine()
	mockSvc.SetPlan("FREE")

	mw := subscription.RequirePlan(mockSvc, "PRO", "ENTERPRISE")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req = req.WithContext(composables.WithTenantID(req.Context(), uuid.New()))
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	assert.Equal(t, http.StatusForbidden, res.Code)
}

func TestRequirePlan_Allowed(t *testing.T) {
	t.Parallel()

	mockSvc := subtesting.NewMockEngine()
	mockSvc.SetPlan("PRO")

	mw := subscription.RequirePlan(mockSvc, "PRO", "ENTERPRISE")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req = req.WithContext(composables.WithTenantID(req.Context(), uuid.New()))
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	assert.Equal(t, http.StatusOK, res.Code)
}
