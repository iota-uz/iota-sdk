package itf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
	"golang.org/x/text/language"
)

// MiddlewareFunc is a function that can modify the request context
type MiddlewareFunc func(ctx context.Context, r *http.Request) context.Context

// HookFunc is a function that runs before each test
type HookFunc func(ctx context.Context) context.Context

type Suite struct {
	t           testing.TB
	env         *TestEnvironment
	router      *mux.Router
	modules     []application.Module
	user        user.User
	middlewares []MiddlewareFunc
	beforeEach  []HookFunc
}

func NewSuite(tb testing.TB, modules ...application.Module) *Suite {
	tb.Helper()

	s := &Suite{
		t:           tb,
		modules:     modules,
		middlewares: make([]MiddlewareFunc, 0),
		beforeEach:  make([]HookFunc, 0),
	}

	s.env = NewTestContext().WithModules(modules...).Build(tb)
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

// BeforeEach registers a hook function that runs before each test request
func (s *Suite) BeforeEach(hook HookFunc) *Suite {
	s.beforeEach = append(s.beforeEach, hook)
	return s
}

func (s *Suite) Environment() *TestEnvironment {
	return s.env
}

// Env is a shorthand for Environment()
func (s *Suite) Env() *TestEnvironment {
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

// Upload handles the common two-step file upload pattern used in IOTA SDK tests:
// 1. Upload file to /uploads endpoint with multipart form data
// 2. Extract FileID from the response HTML
// 3. Submit FileID to the target path via POST with form data
// 4. Return the final response from the target endpoint
func (s *Suite) Upload(targetPath string, fileContent []byte, fileName string) *Response {
	s.t.Helper()

	if len(fileContent) == 0 {
		s.t.Fatal("Upload: file content cannot be empty")
	}

	if fileName == "" {
		s.t.Fatal("Upload: file name cannot be empty")
	}

	if targetPath == "" {
		s.t.Fatal("Upload: target path cannot be empty")
	}

	// Step 1: Upload file to /uploads endpoint
	uploadResponse := s.POST("/uploads").
		MultipartData(NewMultipart().
			AddFile("file", fileName, fileContent).
			AddField("_name", "FileID")).
		HTMX().
		Expect(s.t)

	// Step 2: Extract FileID from response HTML
	fileIDElement := uploadResponse.HTML().Element("//input[@name='FileID']")
	if fileIDElement.node == nil {
		s.t.Fatalf("Upload: FileID input element not found in upload response. Response body: %s", uploadResponse.Body())
	}

	fileID := fileIDElement.Attr("value")
	if fileID == "" {
		s.t.Fatalf("Upload: FileID value is empty in upload response. Response body: %s", uploadResponse.Body())
	}

	// Step 3: Submit FileID to target path
	finalResponse := s.POST(targetPath).
		Form(url.Values{"FileID": []string{fileID}}).
		HTMX().
		Expect(s.t)

	return finalResponse
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
	// Use the standard middleware for i18n/localizer setup
	s.router.Use(middleware.ProvideLocalizer(s.env.App.Bundle()))

	s.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Execute BeforeEach hooks
			for _, hook := range s.beforeEach {
				ctx = hook(ctx) //nolint:fatcontext
			}

			currentUser := s.env.User
			if s.user != nil {
				currentUser = s.user
			}

			if currentUser != nil {
				ctx = composables.WithUser(ctx, currentUser)
			}
			ctx = composables.WithPool(ctx, s.env.Pool)
			ctx = composables.WithTx(ctx, s.env.Tx)
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
			//nolint:staticcheck // SA1019: Using PageContext for test fixtures is acceptable
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

// Enhanced Form Builder Methods

// FormField adds a single form field to the request
func (r *Request) FormField(key string, value interface{}) *Request {
	if r.headers.Get("Content-Type") == "" {
		r.headers.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	// Convert existing body back to url.Values if needed
	var values url.Values
	if r.body != nil {
		// Try to parse existing form data
		if parsed, err := url.ParseQuery(string(r.body)); err == nil {
			values = parsed
		} else {
			values = make(url.Values)
		}
	} else {
		values = make(url.Values)
	}

	// Convert value to string and add to form
	strValue := r.convertToString(value)
	values.Add(key, strValue)

	// Update the request body
	r.body = []byte(values.Encode())
	return r
}

// FormFields adds multiple form fields to the request from a map
func (r *Request) FormFields(fields map[string]interface{}) *Request {
	for key, value := range fields {
		r = r.FormField(key, value)
	}
	return r
}

// FormString adds a string form field to the request
func (r *Request) FormString(key string, value string) *Request {
	return r.FormField(key, value)
}

// FormFloat adds a float form field to the request
func (r *Request) FormFloat(key string, value float64) *Request {
	return r.FormField(key, value)
}

// FormInt adds an integer form field to the request
func (r *Request) FormInt(key string, value int) *Request {
	return r.FormField(key, value)
}

// FormBool adds a boolean form field to the request
func (r *Request) FormBool(key string, value bool) *Request {
	return r.FormField(key, value)
}

// convertToString converts various types to string for form encoding
func (r *Request) convertToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case int:
		return fmt.Sprintf("%d", v)
	case int32:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case float32:
		return fmt.Sprintf("%g", v)
	case float64:
		return fmt.Sprintf("%g", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		// Fallback to fmt.Sprintf for other types
		return fmt.Sprintf("%v", v)
	}
}

type MultipartFile struct {
	FieldName string
	FileName  string
	Content   []byte
}

type MultipartData struct {
	files      []MultipartFile
	formValues url.Values
}

func NewMultipart() *MultipartData {
	return &MultipartData{}
}

func (m *MultipartData) AddFile(fieldName, fileName string, content []byte) *MultipartData {
	m.files = append(m.files, MultipartFile{
		FieldName: fieldName,
		FileName:  fileName,
		Content:   content,
	})
	return m
}

func (m *MultipartData) AddField(key, value string) *MultipartData {
	if m.formValues == nil {
		m.formValues = make(url.Values)
	}
	m.formValues.Add(key, value)
	return m
}

func (m *MultipartData) AddForm(formValues url.Values) *MultipartData {
	if m.formValues == nil {
		m.formValues = make(url.Values)
	}
	for key, values := range formValues {
		for _, value := range values {
			m.formValues.Add(key, value)
		}
	}
	return m
}

// Deprecated: Use MultipartData with NewMultipart() instead for more flexibility
func (r *Request) File(fieldName, fileName string, content []byte) *Request {
	return r.MultipartData(NewMultipart().AddFile(fieldName, fileName, content))
}

func (r *Request) MultipartData(data *MultipartData) *Request {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add files
	for _, file := range data.files {
		part, err := writer.CreateFormFile(file.FieldName, file.FileName)
		if err != nil {
			r.suite.t.Fatalf("Failed to create form file: %v", err)
		}

		if _, err := part.Write(file.Content); err != nil {
			r.suite.t.Fatalf("Failed to write file content: %v", err)
		}
	}

	// Add form fields
	if data.formValues != nil {
		for key, values := range data.formValues {
			for _, value := range values {
				if err := writer.WriteField(key, value); err != nil {
					r.suite.t.Fatalf("Failed to write form field %s: %v", key, err)
				}
			}
		}
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

// HTMX request helpers for enhanced SPA testing

// HTMXTarget sets the HX-Target header for HTMX requests
func (r *Request) HTMXTarget(target string) *Request {
	return r.Header("HX-Target", target)
}

// HTMXTrigger sets the HX-Trigger-Name header for HTMX requests
func (r *Request) HTMXTrigger(triggerName string) *Request {
	return r.Header("HX-Trigger-Name", triggerName)
}

// HTMXSwap sets the HX-Swap header for HTMX requests
func (r *Request) HTMXSwap(swapStyle string) *Request {
	return r.Header("HX-Swap", swapStyle)
}

// HTMXCurrentURL sets the HX-Current-URL header for HTMX requests
func (r *Request) HTMXCurrentURL(url string) *Request {
	return r.Header("HX-Current-URL", url)
}

// HTMXPrompt sets the HX-Prompt header value for HTMX requests
func (r *Request) HTMXPrompt(response string) *Request {
	return r.Header("HX-Prompt", response)
}

// HTMXHistoryRestore marks the request as a history restore request
func (r *Request) HTMXHistoryRestore() *Request {
	return r.Header("HX-History-Restore-Request", "true")
}

// HTMXBoosted marks the request as a boosted request
func (r *Request) HTMXBoosted() *Request {
	return r.Header("HX-Boosted", "true")
}

// WithQuery adds query parameters from a map to the request URL
func (r *Request) WithQuery(params map[string]string) *Request {
	r.suite.t.Helper()

	if len(params) == 0 {
		return r
	}

	// Parse existing URL to preserve any existing query parameters
	parsedURL, err := url.Parse(r.path)
	if err != nil {
		r.suite.t.Fatalf("Failed to parse URL %s: %v", r.path, err)
		return r
	}

	// Get existing query values
	query := parsedURL.Query()

	// Add new parameters with proper URL encoding
	for key, value := range params {
		query.Set(key, value)
	}

	// Rebuild URL with new query parameters
	parsedURL.RawQuery = query.Encode()
	r.path = parsedURL.String()

	return r
}

// WithQueryValue adds a single query parameter to the request URL
func (r *Request) WithQueryValue(key, value string) *Request {
	return r.WithQuery(map[string]string{key: value})
}

func (r *Request) Expect(tb testing.TB) *Response {
	tb.Helper()

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
		t:        tb,
	}
}

// Assert provides enhanced response assertions with fluent API
func (r *Request) Assert(tb testing.TB) *ResponseAssertion {
	tb.Helper()
	response := r.Expect(tb)
	return newResponseAssertion(tb, response)
}

type Response struct {
	suite    *Suite
	recorder *httptest.ResponseRecorder
	doc      *html.Node
	t        testing.TB
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
	t     testing.TB
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
	t     testing.TB
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

// Table-driven test support

// TestCase represents a single test case for table-driven testing
type TestCase struct {
	Name    string
	Setup   func(suite *Suite) // Optional setup for this specific test case
	Request func(suite *Suite) *Request
	Assert  func(t *testing.T, response *Response)
	Skip    bool   // Skip this test case
	Only    bool   // Run only this test case (useful for debugging)
	Cleanup func() // Optional cleanup after test case
}

// RunCases executes a table of test cases
// Note: This method requires the suite to be created with *testing.T, not testing.TB
func (s *Suite) RunCases(cases []TestCase) {
	s.t.Helper()

	// Ensure we have a *testing.T for sub-tests
	runner, ok := s.t.(*testing.T)
	if !ok {
		s.t.Fatal("RunCases requires *testing.T, not testing.TB. Use *testing.T when creating the suite for table-driven tests.")
	}

	// Check if any test case has Only=true, if so, run only those
	onlyTests := make([]TestCase, 0)
	for _, tc := range cases {
		if tc.Only {
			onlyTests = append(onlyTests, tc)
		}
	}

	// If we have "Only" tests, run only those
	testCases := cases
	if len(onlyTests) > 0 {
		testCases = onlyTests
		runner.Logf("Running %d test cases marked with Only=true", len(onlyTests))
	}

	for _, tc := range testCases {
		runner.Run(tc.Name, func(t *testing.T) {
			if tc.Skip {
				t.Skip("Test case marked as skipped")
			}

			// Setup for this specific test case
			if tc.Setup != nil {
				tc.Setup(s)
			}

			// Cleanup after test case
			if tc.Cleanup != nil {
				defer tc.Cleanup()
			}

			// Execute request
			request := tc.Request(s)
			response := request.Expect(t)

			// Execute assertions
			tc.Assert(t, response)
		})
	}
}

// RunCase executes a single test case (helper for single case testing)
func (s *Suite) RunCase(tc TestCase) {
	s.t.Helper()
	s.RunCases([]TestCase{tc})
}

// Legacy HTTP method shortcuts for backward compatibility
// Note: These are deprecated in favor of the new TestCaseBuilder pattern in case_builder.go

// TestGET creates a basic GET request for legacy tests
func (s *Suite) TestGET(path string) func(suite *Suite) *Request {
	return func(suite *Suite) *Request {
		return suite.GET(path)
	}
}

// TestPOST creates a basic POST request for legacy tests
func (s *Suite) TestPOST(path string) func(suite *Suite) *Request {
	return func(suite *Suite) *Request {
		return suite.POST(path)
	}
}

// TestPUT creates a basic PUT request for legacy tests
func (s *Suite) TestPUT(path string) func(suite *Suite) *Request {
	return func(suite *Suite) *Request {
		return suite.PUT(path)
	}
}

// TestDELETE creates a basic DELETE request for legacy tests
func (s *Suite) TestDELETE(path string) func(suite *Suite) *Request {
	return func(suite *Suite) *Request {
		return suite.DELETE(path)
	}
}

// Batch test execution helpers

// BatchTestConfig configures batch test execution
type BatchTestConfig struct {
	Parallel   bool                          // Run test cases in parallel
	MaxWorkers int                           // Maximum number of parallel workers (default: number of test cases)
	BeforeEach func(t *testing.T)            // Hook to run before each test case
	AfterEach  func(t *testing.T)            // Hook to run after each test case
	OnError    func(t *testing.T, err error) // Hook to run when a test fails
}

// RunBatch executes test cases with advanced configuration
// Note: This method requires the suite to be created with *testing.T, not testing.TB
func (s *Suite) RunBatch(cases []TestCase, config *BatchTestConfig) {
	s.t.Helper()

	// Ensure we have a *testing.T for sub-tests
	runner, ok := s.t.(*testing.T)
	if !ok {
		s.t.Fatal("RunBatch requires *testing.T, not testing.TB. Use *testing.T when creating the suite for table-driven tests.")
	}

	if config == nil {
		config = &BatchTestConfig{}
	}

	// Default configuration
	if config.MaxWorkers <= 0 {
		config.MaxWorkers = len(cases)
	}

	// Check for "Only" tests
	onlyTests := make([]TestCase, 0)
	for _, tc := range cases {
		if tc.Only {
			onlyTests = append(onlyTests, tc)
		}
	}

	testCases := cases
	if len(onlyTests) > 0 {
		testCases = onlyTests
		runner.Logf("Running %d test cases marked with Only=true", len(onlyTests))
	}

	if config.Parallel {
		// Run test cases in parallel
		for _, tc := range testCases {
			runner.Run(tc.Name, func(t *testing.T) {
				t.Parallel()
				s.runSingleCase(t, tc, config)
			})
		}
	} else {
		// Run test cases sequentially
		for _, tc := range testCases {
			runner.Run(tc.Name, func(t *testing.T) {
				s.runSingleCase(t, tc, config)
			})
		}
	}
}

// runSingleCase executes a single test case with hooks
func (s *Suite) runSingleCase(t *testing.T, tc TestCase, config *BatchTestConfig) {
	t.Helper()

	if tc.Skip {
		t.Skip("Test case marked as skipped")
	}

	// Run BeforeEach hook
	if config.BeforeEach != nil {
		config.BeforeEach(t)
	}

	// Cleanup after test case
	defer func() {
		if tc.Cleanup != nil {
			tc.Cleanup()
		}
		if config.AfterEach != nil {
			config.AfterEach(t)
		}
	}()

	// Setup for this specific test case
	if tc.Setup != nil {
		tc.Setup(s)
	}

	// Execute request
	request := tc.Request(s)
	response := request.Expect(t)

	// Execute assertions with error handling
	defer func() {
		if r := recover(); r != nil && config.OnError != nil {
			if err, ok := r.(error); ok {
				config.OnError(t, err)
			}
		}
	}()

	tc.Assert(t, response)
}
