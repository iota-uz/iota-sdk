package applet

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

// TestDefaultRouter_ParseRoute tests DefaultRouter with various paths
func TestDefaultRouter_ParseRoute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		requestPath  string
		basePath     string
		expectedPath string
		hasQuery     bool
		queryKey     string
		queryValue   string
	}{
		{
			name:         "root path",
			requestPath:  "/bichat",
			basePath:     "/bichat",
			expectedPath: "/",
			hasQuery:     false,
		},
		{
			name:         "simple path after basePath",
			requestPath:  "/bichat/sessions",
			basePath:     "/bichat",
			expectedPath: "/sessions",
			hasQuery:     false,
		},
		{
			name:         "nested path",
			requestPath:  "/bichat/sessions/123",
			basePath:     "/bichat",
			expectedPath: "/sessions/123",
			hasQuery:     false,
		},
		{
			name:         "path with query parameters",
			requestPath:  "/bichat/sessions?page=2&limit=10",
			basePath:     "/bichat",
			expectedPath: "/sessions",
			hasQuery:     true,
			queryKey:     "page",
			queryValue:   "2",
		},
		{
			name:         "empty basePath",
			requestPath:  "/sessions/123",
			basePath:     "",
			expectedPath: "/sessions/123",
			hasQuery:     false,
		},
		{
			name:         "basePath with trailing slash",
			requestPath:  "/bichat/sessions",
			basePath:     "/bichat/",
			expectedPath: "sessions",
			hasQuery:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewDefaultRouter()
			r := httptest.NewRequest(http.MethodGet, tt.requestPath, nil)

			routeCtx := router.ParseRoute(r, tt.basePath)

			assert.Equal(t, tt.expectedPath, routeCtx.Path)
			assert.NotNil(t, routeCtx.Params)
			assert.Empty(t, routeCtx.Params) // DefaultRouter doesn't extract params
			assert.NotNil(t, routeCtx.Query)

			if tt.hasQuery {
				assert.Equal(t, tt.queryValue, routeCtx.Query[tt.queryKey])
			}
		})
	}
}

// TestDefaultRouter_ParseRoute_QueryParams tests query parameter extraction
func TestDefaultRouter_ParseRoute_QueryParams(t *testing.T) {
	t.Parallel()

	router := NewDefaultRouter()
	r := httptest.NewRequest(http.MethodGet, "/bichat/sessions?tab=history&filter=active", nil)

	routeCtx := router.ParseRoute(r, "/bichat")

	assert.Equal(t, "/sessions", routeCtx.Path)
	assert.Equal(t, "history", routeCtx.Query["tab"])
	assert.Equal(t, "active", routeCtx.Query["filter"])
}

// TestDefaultRouter_ParseRoute_MultipleQueryValues tests handling of multiple query values
func TestDefaultRouter_ParseRoute_MultipleQueryValues(t *testing.T) {
	t.Parallel()

	router := NewDefaultRouter()
	r := httptest.NewRequest(http.MethodGet, "/bichat/sessions?tags=urgent&tags=important", nil)

	routeCtx := router.ParseRoute(r, "/bichat")

	// DefaultRouter takes only the first value
	assert.Equal(t, "urgent", routeCtx.Query["tags"])
}

// TestDefaultRouter_ParseRoute_EmptyQueryValue tests empty query parameter values
func TestDefaultRouter_ParseRoute_EmptyQueryValue(t *testing.T) {
	t.Parallel()

	router := NewDefaultRouter()
	r := httptest.NewRequest(http.MethodGet, "/bichat/sessions?empty=&normal=value", nil)

	routeCtx := router.ParseRoute(r, "/bichat")

	assert.Equal(t, "", routeCtx.Query["empty"])
	assert.Equal(t, "value", routeCtx.Query["normal"])
}

// TestMuxRouter_ParseRoute_WithParams tests MuxRouter with route parameters
func TestMuxRouter_ParseRoute_WithParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		pattern       string
		requestPath   string
		basePath      string
		expectedPath  string
		expectedParam string
		paramKey      string
	}{
		{
			name:          "single parameter",
			pattern:       "/bichat/sessions/{id}",
			requestPath:   "/bichat/sessions/123",
			basePath:      "/bichat",
			expectedPath:  "/sessions/123",
			expectedParam: "123",
			paramKey:      "id",
		},
		{
			name:          "multiple parameters",
			pattern:       "/bichat/sessions/{sessionID}/messages/{messageID}",
			requestPath:   "/bichat/sessions/abc/messages/xyz",
			basePath:      "/bichat",
			expectedPath:  "/sessions/abc/messages/xyz",
			expectedParam: "abc",
			paramKey:      "sessionID",
		},
		{
			name:          "UUID parameter",
			pattern:       "/bichat/sessions/{id}",
			requestPath:   "/bichat/sessions/550e8400-e29b-41d4-a716-446655440000",
			basePath:      "/bichat",
			expectedPath:  "/sessions/550e8400-e29b-41d4-a716-446655440000",
			expectedParam: "550e8400-e29b-41d4-a716-446655440000",
			paramKey:      "id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up gorilla/mux router
			muxRouter := mux.NewRouter()
			var routeCtx RouteContext

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				appletRouter := NewMuxRouter()
				routeCtx = appletRouter.ParseRoute(r, tt.basePath)
			})

			muxRouter.HandleFunc(tt.pattern, handler)

			r := httptest.NewRequest(http.MethodGet, tt.requestPath, nil)
			w := httptest.NewRecorder()

			muxRouter.ServeHTTP(w, r)

			assert.Equal(t, tt.expectedPath, routeCtx.Path)
			assert.Equal(t, tt.expectedParam, routeCtx.Params[tt.paramKey])
		})
	}
}

// TestMuxRouter_ParseRoute_WithQuery tests MuxRouter with query parameters
func TestMuxRouter_ParseRoute_WithQuery(t *testing.T) {
	t.Parallel()

	// Set up gorilla/mux router
	muxRouter := mux.NewRouter()
	var routeCtx RouteContext

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appletRouter := NewMuxRouter()
		routeCtx = appletRouter.ParseRoute(r, "/bichat")
	})

	muxRouter.HandleFunc("/bichat/sessions/{id}", handler)

	r := httptest.NewRequest(http.MethodGet, "/bichat/sessions/123?tab=history&filter=active", nil)
	w := httptest.NewRecorder()

	muxRouter.ServeHTTP(w, r)

	// Verify route params
	assert.Equal(t, "123", routeCtx.Params["id"])

	// Verify query params
	assert.Equal(t, "history", routeCtx.Query["tab"])
	assert.Equal(t, "active", routeCtx.Query["filter"])
}

// TestMuxRouter_ParseRoute_WithBasePath tests MuxRouter with different base paths
func TestMuxRouter_ParseRoute_WithBasePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		pattern      string
		requestPath  string
		basePath     string
		expectedPath string
	}{
		{
			name:         "bichat module",
			pattern:      "/bichat/sessions/{id}",
			requestPath:  "/bichat/sessions/123",
			basePath:     "/bichat",
			expectedPath: "/sessions/123",
		},
		{
			name:         "finance module",
			pattern:      "/finance/invoices/{id}",
			requestPath:  "/finance/invoices/456",
			basePath:     "/finance",
			expectedPath: "/invoices/456",
		},
		{
			name:         "nested module path",
			pattern:      "/admin/tenants/{id}/settings",
			requestPath:  "/admin/tenants/789/settings",
			basePath:     "/admin",
			expectedPath: "/tenants/789/settings",
		},
		{
			name:         "empty basePath",
			pattern:      "/sessions/{id}",
			requestPath:  "/sessions/123",
			basePath:     "",
			expectedPath: "/sessions/123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up gorilla/mux router
			muxRouter := mux.NewRouter()
			var routeCtx RouteContext

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				appletRouter := NewMuxRouter()
				routeCtx = appletRouter.ParseRoute(r, tt.basePath)
			})

			muxRouter.HandleFunc(tt.pattern, handler)

			r := httptest.NewRequest(http.MethodGet, tt.requestPath, nil)
			w := httptest.NewRecorder()

			muxRouter.ServeHTTP(w, r)

			assert.Equal(t, tt.expectedPath, routeCtx.Path)
		})
	}
}

// TestMuxRouter_ParseRoute_NoParams tests MuxRouter when no params are extracted
func TestMuxRouter_ParseRoute_NoParams(t *testing.T) {
	t.Parallel()

	// Set up gorilla/mux router
	muxRouter := mux.NewRouter()
	var routeCtx RouteContext

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appletRouter := NewMuxRouter()
		routeCtx = appletRouter.ParseRoute(r, "/bichat")
	})

	muxRouter.HandleFunc("/bichat/sessions", handler)

	r := httptest.NewRequest(http.MethodGet, "/bichat/sessions", nil)
	w := httptest.NewRecorder()

	muxRouter.ServeHTTP(w, r)

	assert.Equal(t, "/sessions", routeCtx.Path)
	assert.NotNil(t, routeCtx.Params)
	assert.Empty(t, routeCtx.Params) // No params in this route
}

// TestMuxRouter_ParseRoute_RootPath tests MuxRouter with root path
func TestMuxRouter_ParseRoute_RootPath(t *testing.T) {
	t.Parallel()

	// Set up gorilla/mux router
	muxRouter := mux.NewRouter()
	var routeCtx RouteContext

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appletRouter := NewMuxRouter()
		routeCtx = appletRouter.ParseRoute(r, "/bichat")
	})

	muxRouter.HandleFunc("/bichat", handler)

	r := httptest.NewRequest(http.MethodGet, "/bichat", nil)
	w := httptest.NewRecorder()

	muxRouter.ServeHTTP(w, r)

	assert.Equal(t, "/", routeCtx.Path)
}

// TestMuxRouter_ParseRoute_ComplexParams tests MuxRouter with complex parameter patterns
func TestMuxRouter_ParseRoute_ComplexParams(t *testing.T) {
	t.Parallel()

	// Set up gorilla/mux router
	muxRouter := mux.NewRouter()
	var routeCtx RouteContext

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appletRouter := NewMuxRouter()
		routeCtx = appletRouter.ParseRoute(r, "/bichat")
	})

	// Complex pattern with multiple params
	muxRouter.HandleFunc("/bichat/orgs/{orgID}/sessions/{sessionID}/messages/{messageID}", handler)

	r := httptest.NewRequest(http.MethodGet, "/bichat/orgs/org1/sessions/sess2/messages/msg3", nil)
	w := httptest.NewRecorder()

	muxRouter.ServeHTTP(w, r)

	assert.Equal(t, "/orgs/org1/sessions/sess2/messages/msg3", routeCtx.Path)
	assert.Equal(t, "org1", routeCtx.Params["orgID"])
	assert.Equal(t, "sess2", routeCtx.Params["sessionID"])
	assert.Equal(t, "msg3", routeCtx.Params["messageID"])
}

// TestMuxRouter_ParseRoute_SpecialCharacters tests MuxRouter with special characters in params
func TestMuxRouter_ParseRoute_SpecialCharacters(t *testing.T) {
	t.Parallel()

	// Set up gorilla/mux router
	muxRouter := mux.NewRouter()
	var routeCtx RouteContext

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appletRouter := NewMuxRouter()
		routeCtx = appletRouter.ParseRoute(r, "/bichat")
	})

	muxRouter.HandleFunc("/bichat/sessions/{id}", handler)

	r := httptest.NewRequest(http.MethodGet, "/bichat/sessions/abc-123_xyz", nil)
	w := httptest.NewRecorder()

	muxRouter.ServeHTTP(w, r)

	assert.Equal(t, "abc-123_xyz", routeCtx.Params["id"])
}

// TestMuxRouter_ParseRoute_URLEncodedParams tests MuxRouter with URL-encoded parameters
func TestMuxRouter_ParseRoute_URLEncodedParams(t *testing.T) {
	t.Parallel()

	// Set up gorilla/mux router
	muxRouter := mux.NewRouter()
	var routeCtx RouteContext

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appletRouter := NewMuxRouter()
		routeCtx = appletRouter.ParseRoute(r, "/bichat")
	})

	muxRouter.HandleFunc("/bichat/sessions/{id}", handler)

	// URL with encoded query parameter
	r := httptest.NewRequest(http.MethodGet, "/bichat/sessions/123?search=hello%20world", nil)
	w := httptest.NewRecorder()

	muxRouter.ServeHTTP(w, r)

	assert.Equal(t, "123", routeCtx.Params["id"])
	assert.Equal(t, "hello world", routeCtx.Query["search"]) // Should be decoded
}

// TestRouterInterface tests that both routers implement AppletRouter
func TestRouterInterface(t *testing.T) {
	t.Parallel()

	var _ AppletRouter = (*DefaultRouter)(nil)
	var _ AppletRouter = (*MuxRouter)(nil)
}

// TestNewDefaultRouter tests DefaultRouter constructor
func TestNewDefaultRouter(t *testing.T) {
	t.Parallel()

	router := NewDefaultRouter()
	assert.NotNil(t, router)
	assert.Implements(t, (*AppletRouter)(nil), router)
}

// TestNewMuxRouter tests MuxRouter constructor
func TestNewMuxRouter(t *testing.T) {
	t.Parallel()

	router := NewMuxRouter()
	assert.NotNil(t, router)
	assert.Implements(t, (*AppletRouter)(nil), router)
}
