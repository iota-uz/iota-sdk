package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	middleware2 "github.com/iota-uz/iota-sdk/modules/superadmin/middleware"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/sirupsen/logrus"
)

func TestRequireSuperAdmin(t *testing.T) {
	t.Parallel()

	// Create a test handler that should only be reached if middleware passes
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	tests := []struct {
		name          string
		setupUser     func() user.User
		expectStatus  int
		expectBody    string
		expectAllowed bool
		description   string
	}{
		{
			name: "SuperadminUserAllowed",
			setupUser: func() user.User {
				email, _ := internet.NewEmail("superadmin@example.com")
				return user.New(
					"Super",
					"Admin",
					email,
					user.UILanguageEN,
					user.WithID(1),
					user.WithType(user.TypeSuperAdmin),
					user.WithTenantID(uuid.New()),
				)
			},
			expectStatus:  http.StatusOK,
			expectBody:    "success",
			expectAllowed: true,
			description:   "Superadmin user should be allowed to access the route",
		},
		{
			name: "RegularUserBlocked",
			setupUser: func() user.User {
				email, _ := internet.NewEmail("user@example.com")
				return user.New(
					"Regular",
					"User",
					email,
					user.UILanguageEN,
					user.WithID(2),
					user.WithType(user.TypeUser),
					user.WithTenantID(uuid.New()),
				)
			},
			expectStatus:  http.StatusForbidden,
			expectBody:    "Forbidden\n",
			expectAllowed: false,
			description:   "Regular user should be blocked from accessing the route",
		},
		{
			name: "SystemUserBlocked",
			setupUser: func() user.User {
				email, _ := internet.NewEmail("system@example.com")
				return user.New(
					"System",
					"User",
					email,
					user.UILanguageEN,
					user.WithID(3),
					user.WithType(user.TypeSystem),
					user.WithTenantID(uuid.New()),
				)
			},
			expectStatus:  http.StatusForbidden,
			expectBody:    "Forbidden\n",
			expectAllowed: false,
			description:   "System user should be blocked from accessing the route",
		},
		{
			name: "NoUserBlocked",
			setupUser: func() user.User {
				return nil // No user in context
			},
			expectStatus:  http.StatusForbidden,
			expectBody:    "Forbidden\n",
			expectAllowed: false,
			description:   "Request with no user should be blocked",
		},
	}

	for _, tc := range tests {
		tc := tc // Capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Create router with middleware
			router := mux.NewRouter()
			router.Use(middleware2.RequireSuperAdmin())
			router.HandleFunc("/test", testHandler)

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)

			// Setup context with user (if any)
			ctx := req.Context()

			// Add logger to context
			logger := logrus.New()
			logger.SetLevel(logrus.DebugLevel)
			ctx = context.WithValue(ctx, constants.LoggerKey, logrus.NewEntry(logger))

			// Add user to context if provided
			usr := tc.setupUser()
			if usr != nil {
				ctx = composables.WithUser(ctx, usr)
			}

			req = req.WithContext(ctx)

			// Create response recorder
			rr := httptest.NewRecorder()

			// Serve request
			router.ServeHTTP(rr, req)

			// Assert response
			assert.Equal(t, tc.expectStatus, rr.Code, tc.description)
			assert.Equal(t, tc.expectBody, rr.Body.String(), "Response body should match expected")

			if tc.expectAllowed {
				assert.Equal(t, http.StatusOK, rr.Code, "Allowed request should return 200")
			} else {
				assert.Equal(t, http.StatusForbidden, rr.Code, "Blocked request should return 403")
			}
		})
	}
}

func TestRequireSuperAdmin_ThreadSafety(t *testing.T) {
	t.Parallel()

	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create router with middleware
	router := mux.NewRouter()
	router.Use(middleware2.RequireSuperAdmin())
	router.HandleFunc("/test", testHandler)

	// Create different users
	email1, _ := internet.NewEmail("superadmin1@example.com")
	superadmin := user.New(
		"Super",
		"Admin",
		email1,
		user.UILanguageEN,
		user.WithID(1),
		user.WithType(user.TypeSuperAdmin),
		user.WithTenantID(uuid.New()),
	)

	email2, _ := internet.NewEmail("user@example.com")
	regularUser := user.New(
		"Regular",
		"User",
		email2,
		user.UILanguageEN,
		user.WithID(2),
		user.WithType(user.TypeUser),
		user.WithTenantID(uuid.New()),
	)

	// Run concurrent requests with different users
	const numRequests = 50
	done := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(idx int) {
			// Alternate between superadmin and regular user
			var usr user.User
			var expectedStatus int
			if idx%2 == 0 {
				usr = superadmin
				expectedStatus = http.StatusOK
			} else {
				usr = regularUser
				expectedStatus = http.StatusForbidden
			}

			// Create request with user context
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			ctx := req.Context()

			// Add logger
			logger := logrus.New()
			ctx = context.WithValue(ctx, constants.LoggerKey, logrus.NewEntry(logger))

			// Add user
			ctx = composables.WithUser(ctx, usr)
			req = req.WithContext(ctx)

			// Execute request
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			// Verify response
			require.Equal(t, expectedStatus, rr.Code, "Concurrent request %d should return correct status", idx)
			done <- true
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		<-done
	}
}

func TestRequireSuperAdmin_ErrorLogging(t *testing.T) {
	t.Parallel()

	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create router with middleware
	router := mux.NewRouter()
	router.Use(middleware2.RequireSuperAdmin())
	router.HandleFunc("/test", testHandler)

	tests := []struct {
		name      string
		setupUser func() user.User
		hasLogger bool
	}{
		{
			name: "WithLogger",
			setupUser: func() user.User {
				email, _ := internet.NewEmail("user@example.com")
				return user.New(
					"Regular",
					"User",
					email,
					user.UILanguageEN,
					user.WithType(user.TypeUser),
				)
			},
			hasLogger: true,
		},
		{
			name: "WithoutLogger",
			setupUser: func() user.User {
				email, _ := internet.NewEmail("user@example.com")
				return user.New(
					"Regular",
					"User",
					email,
					user.UILanguageEN,
					user.WithType(user.TypeUser),
				)
			},
			hasLogger: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			ctx := req.Context()

			// Conditionally add logger
			if tc.hasLogger {
				logger := logrus.New()
				ctx = context.WithValue(ctx, constants.LoggerKey, logrus.NewEntry(logger))
			}

			// Add user
			usr := tc.setupUser()
			if usr != nil {
				ctx = composables.WithUser(ctx, usr)
			}

			req = req.WithContext(ctx)
			rr := httptest.NewRecorder()

			// Should not panic regardless of logger presence
			require.NotPanics(t, func() {
				router.ServeHTTP(rr, req)
			})

			// Should still return 403
			assert.Equal(t, http.StatusForbidden, rr.Code)
		})
	}
}
