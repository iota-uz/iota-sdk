package controllertest

import (
	"bytes"
	"context"
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

// Suite provides a fluent API for controller testing
type Suite struct {
	env     *builder.TestEnvironment
	router  *mux.Router
	modules []application.Module
}

// New creates a new controller test suite
func New() *Suite {
	return &Suite{}
}

// WithModule adds a module for the test
func (s *Suite) WithModules(module ...application.Module) *Suite {
	s.modules = append(s.modules, module...)
	return s
}

// WithUser sets the user for the test
func (s *Suite) WithUser(t *testing.T, u user.User) *Suite {
	t.Helper()
	s.env = builder.New().
		WithModules(s.modules...).
		WithUser(u).
		Build(t)
	return s
}

// Build finalizes the test suite setup
func (s *Suite) Build(t *testing.T) *Suite {
	t.Helper()
	if s.env == nil {
		s.env = builder.New().
			WithModules(s.modules...).
			Build(t)
	}

	s.router = mux.NewRouter()
	s.setupMiddleware()

	return s
}

// RegisterController registers a controller with the router
func (s *Suite) RegisterController(controller interface{ Register(*mux.Router) }) *Suite {
	controller.Register(s.router)
	return s
}

// Request creates a new request builder
func (s *Suite) Request(method, path string) *RequestBuilder {
	return &RequestBuilder{
		suite:   s,
		method:  method,
		path:    path,
		headers: make(http.Header),
	}
}

// GET creates a GET request builder
func (s *Suite) GET(path string) *RequestBuilder {
	return s.Request(http.MethodGet, path)
}

// POST creates a POST request builder
func (s *Suite) POST(path string) *RequestBuilder {
	return s.Request(http.MethodPost, path)
}

// PUT creates a PUT request builder
func (s *Suite) PUT(path string) *RequestBuilder {
	return s.Request(http.MethodPut, path)
}

// DELETE creates a DELETE request builder
func (s *Suite) DELETE(path string) *RequestBuilder {
	return s.Request(http.MethodDelete, path)
}

// Environment returns the test environment
func (s *Suite) Environment() *builder.TestEnvironment {
	return s.env
}

func (s *Suite) setupMiddleware() {
	s.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Add all necessary context values
			ctx = composables.WithUser(ctx, s.env.User)
			ctx = composables.WithPool(ctx, s.env.Pool)
			ctx = composables.WithTx(ctx, s.env.Tx)
			ctx = composables.WithSession(ctx, &session.Session{})
			ctx = composables.WithTenantID(ctx, s.env.Tenant.ID)
			ctx = context.WithValue(ctx, constants.AppKey, s.env.App)
			ctx = context.WithValue(ctx, constants.HeadKey, templ.NopComponent)
			ctx = context.WithValue(ctx, constants.LogoKey, templ.NopComponent)

			// Add logger
			logger := logrus.New()
			fieldsLogger := logger.WithFields(logrus.Fields{
				"test": true,
				"path": r.URL.Path,
			})
			ctx = context.WithValue(ctx, constants.LoggerKey, fieldsLogger)

			// Add params
			params := &composables.Params{
				IP:            "127.0.0.1",
				UserAgent:     "test-agent",
				Authenticated: s.env.User != nil,
				Request:       r,
				Writer:        w,
			}
			ctx = composables.WithParams(ctx, params)

			// Add localizer and page context
			localizer := i18n.NewLocalizer(s.env.App.Bundle(), "en")
			parsedURL, _ := url.Parse(r.URL.Path)
			ctx = composables.WithPageCtx(ctx, &types.PageContext{
				Locale:    language.English,
				URL:       parsedURL,
				Localizer: localizer,
			})

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
}

// RequestBuilder builds HTTP requests
type RequestBuilder struct {
	suite         *Suite
	method        string
	path          string
	body          []byte
	headers       http.Header
	multipartForm *MultipartFormBuilder
}

// WithJSON sets JSON body
func (rb *RequestBuilder) WithJSON(v interface{}) *RequestBuilder {
	// Implementation would serialize v to JSON
	return rb
}

// WithForm sets form data
func (rb *RequestBuilder) WithForm(values url.Values) *RequestBuilder {
	rb.body = []byte(values.Encode())
	rb.headers.Set("Content-Type", "application/x-www-form-urlencoded")
	return rb
}

// WithMultipartForm creates a multipart form builder
func (rb *RequestBuilder) WithMultipartForm() *MultipartFormBuilder {
	rb.multipartForm = &MultipartFormBuilder{
		requestBuilder: rb,
		fields:         make(map[string]string),
		files:          make([]multipartFile, 0),
	}
	return rb.multipartForm
}

// WithHeader adds a header
func (rb *RequestBuilder) WithHeader(key, value string) *RequestBuilder {
	rb.headers.Set(key, value)
	return rb
}

// HTMX marks the request as HTMX
func (rb *RequestBuilder) HTMX() *RequestBuilder {
	return rb.WithHeader("Hx-Request", "true")
}

// Expect executes the request and returns response assertions
func (rb *RequestBuilder) Expect() *ResponseAssertion {
	var bodyReader io.Reader

	if rb.multipartForm != nil {
		// Build multipart form
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Add fields
		for key, value := range rb.multipartForm.fields {
			_ = writer.WriteField(key, value)
		}

		// Add files
		for _, file := range rb.multipartForm.files {
			part, _ := writer.CreateFormFile(file.fieldName, file.fileName)
			_, _ = io.Copy(part, bytes.NewReader(file.content))
		}

		_ = writer.Close()

		bodyReader = body
		rb.headers.Set("Content-Type", writer.FormDataContentType())
	} else if rb.body != nil {
		bodyReader = bytes.NewReader(rb.body)
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}

	req := httptest.NewRequest(rb.method, rb.path, bodyReader)
	for k, v := range rb.headers {
		req.Header[k] = v
	}

	rr := httptest.NewRecorder()
	rb.suite.router.ServeHTTP(rr, req)

	return &ResponseAssertion{
		suite:    rb.suite,
		recorder: rr,
	}
}

// ResponseAssertion provides response assertions
type ResponseAssertion struct {
	suite    *Suite
	recorder *httptest.ResponseRecorder
	doc      *html.Node
}

// Status asserts the response status code
func (ra *ResponseAssertion) Status(t *testing.T, code int) *ResponseAssertion {
	t.Helper()
	assert.Equal(t, code, ra.recorder.Code)
	return ra
}

// RedirectTo asserts redirect location
func (ra *ResponseAssertion) RedirectTo(t *testing.T, location string) *ResponseAssertion {
	t.Helper()
	assert.Equal(t, location, ra.recorder.Header().Get("Location"))
	return ra
}

// Body returns the response body
func (ra *ResponseAssertion) Body() string {
	return ra.recorder.Body.String()
}

// HTML parses and returns the HTML document
func (ra *ResponseAssertion) HTML(t *testing.T) *HTMLAssertion {
	t.Helper()
	if ra.doc == nil {
		doc, err := htmlquery.Parse(strings.NewReader(ra.Body()))
		require.NoError(t, err)
		ra.doc = doc
	}
	return &HTMLAssertion{
		suite: ra.suite,
		doc:   ra.doc,
	}
}

// Contains asserts the body contains text
func (ra *ResponseAssertion) Contains(t *testing.T, text string) *ResponseAssertion {
	t.Helper()
	assert.Contains(t, ra.Body(), text)
	return ra
}

// NotContains asserts the body doesn't contain text
func (ra *ResponseAssertion) NotContains(t *testing.T, text string) *ResponseAssertion {
	t.Helper()
	assert.NotContains(t, ra.Body(), text)
	return ra
}

// HTMLAssertion provides HTML-specific assertions
type HTMLAssertion struct {
	suite *Suite
	doc   *html.Node
}

// Element finds an element by XPath
func (ha *HTMLAssertion) Element(xpath string) *ElementAssertion {
	node := htmlquery.FindOne(ha.doc, xpath)
	return &ElementAssertion{
		suite: ha.suite,
		node:  node,
		xpath: xpath,
	}
}

// Elements finds multiple elements by XPath
func (ha *HTMLAssertion) Elements(xpath string) []*html.Node {
	return htmlquery.Find(ha.doc, xpath)
}

// HasErrorFor checks if there's an error for a specific field
func (ha *HTMLAssertion) HasErrorFor(fieldID string) bool {
	xpath := "//small[@data-testid='field-error' and @data-field-id='" + fieldID + "']"
	return htmlquery.FindOne(ha.doc, xpath) != nil
}

// ElementAssertion provides element-specific assertions
type ElementAssertion struct {
	suite *Suite
	node  *html.Node
	xpath string
}

// Exists asserts the element exists
func (ea *ElementAssertion) Exists(t *testing.T) *ElementAssertion {
	t.Helper()
	assert.NotNil(t, ea.node, "Element not found: %s", ea.xpath)
	return ea
}

// NotExists asserts the element doesn't exist
func (ea *ElementAssertion) NotExists(t *testing.T) *ElementAssertion {
	t.Helper()
	assert.Nil(t, ea.node, "Element should not exist: %s", ea.xpath)
	return ea
}

// Text returns the element's text content
func (ea *ElementAssertion) Text() string {
	if ea.node == nil {
		return ""
	}
	return htmlquery.InnerText(ea.node)
}

// Attr returns an attribute value
func (ea *ElementAssertion) Attr(name string) string {
	if ea.node == nil {
		return ""
	}
	return htmlquery.SelectAttr(ea.node, name)
}

// multipartFile represents a file in a multipart form
type multipartFile struct {
	fieldName string
	fileName  string
	content   []byte
}

// MultipartFormBuilder builds multipart form requests
type MultipartFormBuilder struct {
	requestBuilder *RequestBuilder
	fields         map[string]string
	files          []multipartFile
}

// AddField adds a field to the multipart form
func (mfb *MultipartFormBuilder) AddField(key, value string) *MultipartFormBuilder {
	mfb.fields[key] = value
	return mfb
}

// AddFile adds a file to the multipart form
func (mfb *MultipartFormBuilder) AddFile(fieldName, fileName string, content []byte) *MultipartFormBuilder {
	mfb.files = append(mfb.files, multipartFile{
		fieldName: fieldName,
		fileName:  fileName,
		content:   content,
	})
	return mfb
}

// AddFileFromReader adds a file from an io.Reader to the multipart form
func (mfb *MultipartFormBuilder) AddFileFromReader(fieldName, fileName string, reader io.Reader) *MultipartFormBuilder {
	content, _ := io.ReadAll(reader)
	return mfb.AddFile(fieldName, fileName, content)
}

// WithHeader adds a header to the request
func (mfb *MultipartFormBuilder) WithHeader(key, value string) *MultipartFormBuilder {
	mfb.requestBuilder.WithHeader(key, value)
	return mfb
}

// HTMX marks the request as HTMX
func (mfb *MultipartFormBuilder) HTMX() *MultipartFormBuilder {
	mfb.requestBuilder.HTMX()
	return mfb
}

// Expect executes the request and returns response assertions
func (mfb *MultipartFormBuilder) Expect() *ResponseAssertion {
	return mfb.requestBuilder.Expect()
}
