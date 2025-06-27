package controllertest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/antchfx/htmlquery"
	"github.com/gorilla/mux"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/testutils/builder"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
	"golang.org/x/text/language"
)

// MiddlewareFunc is a function that can modify the request context
type MiddlewareFunc func(ctx context.Context, r *http.Request) context.Context

type Suite struct {
	t           *testing.T
	env         *builder.TestEnvironment
	router      *mux.Router
	modules     []application.Module
	user        user.User
	middlewares []MiddlewareFunc
}

func New(t *testing.T, modules ...application.Module) *Suite {
	t.Helper()

	s := &Suite{
		t:           t,
		modules:     modules,
		middlewares: make([]MiddlewareFunc, 0),
	}

	s.env = builder.New().WithModules(modules...).Build(t)
	s.router = mux.NewRouter()
	s.setupMiddleware()

	return s
}

func (s *Suite) AsUser(u user.User) *Suite {
	s.user = u
	// Reuse existing environment but update the user context
	s.env.User = u
	s.env.Ctx = composables.WithUser(s.env.Ctx, u)
	return s
}

func (s *Suite) Register(controller interface{ Register(*mux.Router) }) *Suite {
	controller.Register(s.router)
	return s
}

// WithMiddleware registers a custom middleware function that can modify the request context
func (s *Suite) WithMiddleware(middleware MiddlewareFunc) *Suite {
	s.middlewares = append(s.middlewares, middleware)
	return s
}

func (s *Suite) Environment() *builder.TestEnvironment {
	return s.env
}

func (s *Suite) GET(path string) *Request {
	return s.newRequest(http.MethodGet, path)
}

func (s *Suite) POST(path string) *Request {
	return s.newRequest(http.MethodPost, path)
}

func (s *Suite) PUT(path string) *Request {
	return s.newRequest(http.MethodPut, path)
}

func (s *Suite) DELETE(path string) *Request {
	return s.newRequest(http.MethodDelete, path)
}

func (s *Suite) newRequest(method, path string) *Request {
	return &Request{
		suite:   s,
		method:  method,
		path:    path,
		headers: make(http.Header),
	}
}

func (s *Suite) setupMiddleware() {
	s.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			currentUser := s.env.User
			if s.user != nil {
				currentUser = s.user
			}

			ctx = composables.WithUser(ctx, currentUser)
			ctx = composables.WithPool(ctx, s.env.Pool)
			ctx = composables.WithSession(ctx, &session.Session{})
			ctx = composables.WithTenantID(ctx, s.env.Tenant.ID)
			ctx = context.WithValue(ctx, constants.AppKey, s.env.App)
			ctx = context.WithValue(ctx, constants.HeadKey, templ.NopComponent)
			ctx = context.WithValue(ctx, constants.LogoKey, templ.NopComponent)

			logger := logrus.New()
			fieldsLogger := logger.WithFields(logrus.Fields{
				"test": true,
				"path": r.URL.Path,
			})
			ctx = context.WithValue(ctx, constants.LoggerKey, fieldsLogger)

			params := &composables.Params{
				IP:            "127.0.0.1",
				UserAgent:     "test-agent",
				Authenticated: currentUser != nil,
				Request:       r,
				Writer:        w,
			}
			ctx = composables.WithParams(ctx, params)

			localizer := i18n.NewLocalizer(s.env.App.Bundle(), "en")
			parsedURL, _ := url.Parse(r.URL.Path)
			ctx = composables.WithPageCtx(ctx, &types.PageContext{
				Locale:    language.English,
				URL:       parsedURL,
				Localizer: localizer,
			})

			// Execute custom middleware functions
			for _, mw := range s.middlewares {
				ctx = mw(ctx, r) //nolint:fatcontext
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
}

type Request struct {
	suite   *Suite
	method  string
	path    string
	headers http.Header
	body    []byte
}

func (r *Request) JSON(v interface{}) *Request {
	data, err := json.Marshal(v)
	if err != nil {
		r.suite.t.Fatalf("Failed to marshal JSON: %v", err)
	}
	r.body = data
	r.headers.Set("Content-Type", "application/json")
	return r
}

func (r *Request) Form(values url.Values) *Request {
	r.body = []byte(values.Encode())
	r.headers.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func (r *Request) File(fieldName, fileName string, content []byte) *Request {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		r.suite.t.Fatalf("Failed to create form file: %v", err)
	}

	if _, err := part.Write(content); err != nil {
		r.suite.t.Fatalf("Failed to write file content: %v", err)
	}

	if err := writer.Close(); err != nil {
		r.suite.t.Fatalf("Failed to close multipart writer: %v", err)
	}

	r.body = body.Bytes()
	r.headers.Set("Content-Type", writer.FormDataContentType())
	return r
}

func (r *Request) Header(key, value string) *Request {
	r.headers.Set(key, value)
	return r
}

func (r *Request) Cookie(name, value string) *Request {
	r.headers.Add("Cookie", name+"="+value)
	return r
}

func (r *Request) HTMX() *Request {
	return r.Header("Hx-Request", "true")
}

func (r *Request) Expect(t *testing.T) *Response {
	t.Helper()

	var bodyReader io.Reader
	if r.body != nil {
		bodyReader = bytes.NewReader(r.body)
	}

	req := httptest.NewRequest(r.method, r.path, bodyReader)
	for k, v := range r.headers {
		req.Header[k] = v
	}

	recorder := httptest.NewRecorder()
	r.suite.router.ServeHTTP(recorder, req)

	return &Response{
		suite:    r.suite,
		recorder: recorder,
		t:        t,
	}
}

type Response struct {
	suite    *Suite
	recorder *httptest.ResponseRecorder
	doc      *html.Node
	t        *testing.T
}

func (r *Response) Status(code int) *Response {
	r.t.Helper()
	assert.Equal(r.t, code, r.recorder.Code, "Unexpected status code. Body: %s", r.Body())
	return r
}

func (r *Response) RedirectTo(location string) *Response {
	r.t.Helper()
	assert.Equal(r.t, location, r.recorder.Header().Get("Location"))
	return r
}

func (r *Response) Contains(text string) *Response {
	r.t.Helper()
	assert.Contains(r.t, r.Body(), text)
	return r
}

func (r *Response) NotContains(text string) *Response {
	r.t.Helper()
	assert.NotContains(r.t, r.Body(), text)
	return r
}

func (r *Response) Body() string {
	return r.recorder.Body.String()
}

func (r *Response) Header(key string) string {
	return r.recorder.Header().Get(key)
}

func (r *Response) Cookies() []*http.Cookie {
	return r.recorder.Result().Cookies()
}

func (r *Response) Raw() *http.Response {
	return r.recorder.Result()
}

func (r *Response) HTML() *HTML {
	r.t.Helper()
	if r.doc == nil {
		doc, err := htmlquery.Parse(strings.NewReader(r.Body()))
		require.NoError(r.t, err, "Failed to parse HTML")
		r.doc = doc
	}
	return &HTML{
		suite: r.suite,
		doc:   r.doc,
		t:     r.t,
	}
}

type HTML struct {
	suite *Suite
	doc   *html.Node
	t     *testing.T
}

func (h *HTML) Element(xpath string) *Element {
	node := htmlquery.FindOne(h.doc, xpath)
	return &Element{
		suite: h.suite,
		node:  node,
		xpath: xpath,
		t:     h.t,
	}
}

func (h *HTML) Elements(xpath string) []*html.Node {
	return htmlquery.Find(h.doc, xpath)
}

func (h *HTML) HasErrorFor(fieldID string) bool {
	xpath := "//small[@data-testid='field-error' and @data-field-id='" + fieldID + "']"
	return htmlquery.FindOne(h.doc, xpath) != nil
}

type Element struct {
	suite *Suite
	node  *html.Node
	xpath string
	t     *testing.T
}

func (e *Element) Exists() *Element {
	e.t.Helper()
	assert.NotNil(e.t, e.node, "Element not found: %s", e.xpath)
	return e
}

func (e *Element) NotExists() *Element {
	e.t.Helper()
	assert.Nil(e.t, e.node, "Element should not exist: %s", e.xpath)
	return e
}

func (e *Element) Text() string {
	if e.node == nil {
		return ""
	}
	return htmlquery.InnerText(e.node)
}

func (e *Element) Attr(name string) string {
	if e.node == nil {
		return ""
	}
	return htmlquery.SelectAttr(e.node, name)
}
