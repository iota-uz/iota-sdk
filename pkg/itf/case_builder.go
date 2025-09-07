package itf

import (
	"fmt"
	"testing"
)

// TestCaseBuilder provides a fluent API for building test cases with reduced verbosity
// and common patterns for table-driven tests.
type TestCaseBuilder struct {
	name           string
	method         string
	path           string
	queryParams    map[string]string
	formFields     map[string]interface{}
	jsonData       interface{}
	expectStatus   int
	expectElement  string
	expectRedirect string
	isHTMX         bool
	headers        map[string]string
	customAssert   func(t *testing.T, response *Response)
	setup          func(suite *Suite)
	cleanup        func()
	skip           bool
	only           bool
}

// Entry points for HTTP methods

// GET creates a new TestCaseBuilder for GET requests
func GET(path string) *TestCaseBuilder {
	return &TestCaseBuilder{
		method:      "GET",
		path:        path,
		queryParams: make(map[string]string),
		formFields:  make(map[string]interface{}),
		headers:     make(map[string]string),
	}
}

// POST creates a new TestCaseBuilder for POST requests
func POST(path string) *TestCaseBuilder {
	return &TestCaseBuilder{
		method:      "POST",
		path:        path,
		queryParams: make(map[string]string),
		formFields:  make(map[string]interface{}),
		headers:     make(map[string]string),
	}
}

// PUT creates a new TestCaseBuilder for PUT requests
func PUT(path string) *TestCaseBuilder {
	return &TestCaseBuilder{
		method:      "PUT",
		path:        path,
		queryParams: make(map[string]string),
		formFields:  make(map[string]interface{}),
		headers:     make(map[string]string),
	}
}

// DELETE creates a new TestCaseBuilder for DELETE requests
func DELETE(path string) *TestCaseBuilder {
	return &TestCaseBuilder{
		method:      "DELETE",
		path:        path,
		queryParams: make(map[string]string),
		formFields:  make(map[string]interface{}),
		headers:     make(map[string]string),
	}
}

// Configuration methods (immutable - return new instances)

// Named sets the test case name
func (tcb *TestCaseBuilder) Named(name string) *TestCaseBuilder {
	builder := tcb.copy()
	builder.name = name
	return builder
}

// WithQuery adds query parameters to the request
func (tcb *TestCaseBuilder) WithQuery(params map[string]string) *TestCaseBuilder {
	builder := tcb.copy()
	for k, v := range params {
		builder.queryParams[k] = v
	}
	return builder
}

// WithQueryParam adds a single query parameter to the request
func (tcb *TestCaseBuilder) WithQueryParam(key, value string) *TestCaseBuilder {
	builder := tcb.copy()
	builder.queryParams[key] = value
	return builder
}

// WithForm adds form fields to the request
func (tcb *TestCaseBuilder) WithForm(fields map[string]interface{}) *TestCaseBuilder {
	builder := tcb.copy()
	for k, v := range fields {
		builder.formFields[k] = v
	}
	return builder
}

// WithFormField adds a single form field to the request
func (tcb *TestCaseBuilder) WithFormField(key string, value interface{}) *TestCaseBuilder {
	builder := tcb.copy()
	builder.formFields[key] = value
	return builder
}

// WithJSON sets JSON data for the request
func (tcb *TestCaseBuilder) WithJSON(data interface{}) *TestCaseBuilder {
	builder := tcb.copy()
	builder.jsonData = data
	return builder
}

// WithHeader adds a custom header to the request
func (tcb *TestCaseBuilder) WithHeader(key, value string) *TestCaseBuilder {
	builder := tcb.copy()
	builder.headers[key] = value
	return builder
}

// HTMX marks the request as an HTMX request
func (tcb *TestCaseBuilder) HTMX() *TestCaseBuilder {
	builder := tcb.copy()
	builder.isHTMX = true
	return builder
}

// HTMXTarget sets the HX-Target header for HTMX requests
func (tcb *TestCaseBuilder) HTMXTarget(target string) *TestCaseBuilder {
	builder := tcb.copy()
	builder.isHTMX = true
	builder.headers["HX-Target"] = target
	return builder
}

// HTMXTrigger sets the HX-Trigger-Name header for HTMX requests
func (tcb *TestCaseBuilder) HTMXTrigger(triggerName string) *TestCaseBuilder {
	builder := tcb.copy()
	builder.isHTMX = true
	builder.headers["HX-Trigger-Name"] = triggerName
	return builder
}

// HTMXSwap sets the HX-Swap header for HTMX requests
func (tcb *TestCaseBuilder) HTMXSwap(swapStyle string) *TestCaseBuilder {
	builder := tcb.copy()
	builder.isHTMX = true
	builder.headers["HX-Swap"] = swapStyle
	return builder
}

// Common expectation shortcuts

// ExpectOK expects a 200 OK status
func (tcb *TestCaseBuilder) ExpectOK() *TestCaseBuilder {
	builder := tcb.copy()
	builder.expectStatus = 200
	return builder
}

// ExpectCreated expects a 201 Created status
func (tcb *TestCaseBuilder) ExpectCreated() *TestCaseBuilder {
	builder := tcb.copy()
	builder.expectStatus = 201
	return builder
}

// ExpectBadRequest expects a 400 Bad Request status
func (tcb *TestCaseBuilder) ExpectBadRequest() *TestCaseBuilder {
	builder := tcb.copy()
	builder.expectStatus = 400
	return builder
}

// ExpectNotFound expects a 404 Not Found status
func (tcb *TestCaseBuilder) ExpectNotFound() *TestCaseBuilder {
	builder := tcb.copy()
	builder.expectStatus = 404
	return builder
}

// ExpectConflict expects a 409 Conflict status
func (tcb *TestCaseBuilder) ExpectConflict() *TestCaseBuilder {
	builder := tcb.copy()
	builder.expectStatus = 409
	return builder
}

// ExpectUnauthorized expects a 401 Unauthorized status
func (tcb *TestCaseBuilder) ExpectUnauthorized() *TestCaseBuilder {
	builder := tcb.copy()
	builder.expectStatus = 401
	return builder
}

// ExpectForbidden expects a 403 Forbidden status
func (tcb *TestCaseBuilder) ExpectForbidden() *TestCaseBuilder {
	builder := tcb.copy()
	builder.expectStatus = 403
	return builder
}

// ExpectMethodNotAllowed expects a 405 Method Not Allowed status
func (tcb *TestCaseBuilder) ExpectMethodNotAllowed() *TestCaseBuilder {
	builder := tcb.copy()
	builder.expectStatus = 405
	return builder
}

// ExpectAccepted expects a 202 Accepted status
func (tcb *TestCaseBuilder) ExpectAccepted() *TestCaseBuilder {
	builder := tcb.copy()
	builder.expectStatus = 202
	return builder
}

// ExpectInternalServerError expects a 500 Internal Server Error status
func (tcb *TestCaseBuilder) ExpectInternalServerError() *TestCaseBuilder {
	builder := tcb.copy()
	builder.expectStatus = 500
	return builder
}

// ExpectStatus expects a specific status code
func (tcb *TestCaseBuilder) ExpectStatus(code int) *TestCaseBuilder {
	builder := tcb.copy()
	builder.expectStatus = code
	return builder
}

// ExpectOKWithForm expects 200 OK and presence of a form element
func (tcb *TestCaseBuilder) ExpectOKWithForm() *TestCaseBuilder {
	builder := tcb.copy()
	builder.expectStatus = 200
	builder.expectElement = "//form"
	return builder
}

// ExpectElement expects a specific element to exist (XPath)
func (tcb *TestCaseBuilder) ExpectElement(xpath string) *TestCaseBuilder {
	builder := tcb.copy()
	builder.expectElement = xpath
	return builder
}

// ExpectRedirect expects a redirect to a specific location
func (tcb *TestCaseBuilder) ExpectRedirect(location string) *TestCaseBuilder {
	builder := tcb.copy()
	builder.expectRedirect = location
	return builder
}

// Custom assertion

// Assert sets a custom assertion function
func (tcb *TestCaseBuilder) Assert(assertFunc func(t *testing.T, response *Response)) *TestCaseBuilder {
	builder := tcb.copy()
	builder.customAssert = assertFunc
	return builder
}

// Test case lifecycle

// Setup sets a setup function for this test case
func (tcb *TestCaseBuilder) Setup(setupFunc func(suite *Suite)) *TestCaseBuilder {
	builder := tcb.copy()
	builder.setup = setupFunc
	return builder
}

// Cleanup sets a cleanup function for this test case
func (tcb *TestCaseBuilder) Cleanup(cleanupFunc func()) *TestCaseBuilder {
	builder := tcb.copy()
	builder.cleanup = cleanupFunc
	return builder
}

// Skip marks this test case to be skipped
func (tcb *TestCaseBuilder) Skip() *TestCaseBuilder {
	builder := tcb.copy()
	builder.skip = true
	return builder
}

// Only marks this test case to run exclusively (useful for debugging)
func (tcb *TestCaseBuilder) Only() *TestCaseBuilder {
	builder := tcb.copy()
	builder.only = true
	return builder
}

// Build methods

// TestCase converts the builder to a TestCase struct for use in table-driven tests
func (tcb *TestCaseBuilder) TestCase() TestCase {
	// Use method and path as default name if none provided
	name := tcb.name
	if name == "" {
		if len(tcb.queryParams) > 0 || len(tcb.formFields) > 0 {
			name = fmt.Sprintf("%s %s with params", tcb.method, tcb.path)
		} else {
			name = fmt.Sprintf("%s %s", tcb.method, tcb.path)
		}
	}

	return TestCase{
		Name:    name,
		Setup:   tcb.setup,
		Request: tcb.buildRequest(),
		Assert:  tcb.buildAssert(),
		Skip:    tcb.skip,
		Only:    tcb.only,
		Cleanup: tcb.cleanup,
	}
}

// buildRequest creates the request function for the TestCase
func (tcb *TestCaseBuilder) buildRequest() func(suite *Suite) *Request {
	return func(suite *Suite) *Request {
		var request *Request

		// Create request based on method
		switch tcb.method {
		case "GET":
			request = suite.GET(tcb.path)
		case "POST":
			request = suite.POST(tcb.path)
		case "PUT":
			request = suite.PUT(tcb.path)
		case "DELETE":
			request = suite.DELETE(tcb.path)
		default:
			panic(fmt.Sprintf("Unsupported HTTP method: %s", tcb.method))
		}

		// Add query parameters
		if len(tcb.queryParams) > 0 {
			request = request.WithQuery(tcb.queryParams)
		}

		// Add form fields
		if len(tcb.formFields) > 0 {
			request = request.FormFields(tcb.formFields)
		}

		// Add JSON data
		if tcb.jsonData != nil {
			request = request.JSON(tcb.jsonData)
		}

		// Add custom headers
		for key, value := range tcb.headers {
			request = request.Header(key, value)
		}

		// Mark as HTMX request if needed
		if tcb.isHTMX {
			request = request.HTMX()
		}

		return request
	}
}

// buildAssert creates the assertion function for the TestCase
func (tcb *TestCaseBuilder) buildAssert() func(t *testing.T, response *Response) {
	return func(t *testing.T, response *Response) {
		t.Helper()

		// Check expected status code
		if tcb.expectStatus != 0 {
			response.Status(tcb.expectStatus)
		}

		// Check expected element
		if tcb.expectElement != "" {
			response.HTML().Element(tcb.expectElement).Exists()
		}

		// Check expected redirect
		if tcb.expectRedirect != "" {
			response.RedirectTo(tcb.expectRedirect)
		}

		// Run custom assertions
		if tcb.customAssert != nil {
			tcb.customAssert(t, response)
		}
	}
}

// copy creates a deep copy of the TestCaseBuilder for immutability
func (tcb *TestCaseBuilder) copy() *TestCaseBuilder {
	copied := &TestCaseBuilder{
		name:           tcb.name,
		method:         tcb.method,
		path:           tcb.path,
		expectStatus:   tcb.expectStatus,
		expectElement:  tcb.expectElement,
		expectRedirect: tcb.expectRedirect,
		isHTMX:         tcb.isHTMX,
		jsonData:       tcb.jsonData,
		customAssert:   tcb.customAssert,
		setup:          tcb.setup,
		cleanup:        tcb.cleanup,
		skip:           tcb.skip,
		only:           tcb.only,
		queryParams:    make(map[string]string),
		formFields:     make(map[string]interface{}),
		headers:        make(map[string]string),
	}

	// Deep copy maps
	for k, v := range tcb.queryParams {
		copied.queryParams[k] = v
	}
	for k, v := range tcb.formFields {
		copied.formFields[k] = v
	}
	for k, v := range tcb.headers {
		copied.headers[k] = v
	}

	return copied
}

// Batch building helpers

// Cases converts multiple TestCaseBuilder instances to TestCase slice
func Cases(builders ...*TestCaseBuilder) []TestCase {
	cases := make([]TestCase, len(builders))
	for i, builder := range builders {
		cases[i] = builder.TestCase()
	}
	return cases
}

// Common patterns for SHY ELD application

// FilterTest creates a test case for filtering with query parameters
func FilterTest(path, filterName, filterValue string) *TestCaseBuilder {
	return GET(path).
		Named(fmt.Sprintf("Filter by %s - %s", filterName, filterValue)).
		WithQueryParam(filterName, filterValue).
		ExpectOK().ExpectElement("//table")
}

// FormSubmissionTest creates a test case for form submission
func FormSubmissionTest(path string, formData map[string]interface{}) *TestCaseBuilder {
	return POST(path).
		Named(fmt.Sprintf("Submit form to %s", path)).
		WithForm(formData).
		HTMX().
		ExpectOK()
}

// PaginationTest creates a test case for pagination
func PaginationTest(path string, page int) *TestCaseBuilder {
	return GET(path).
		Named(fmt.Sprintf("Page %d", page)).
		WithQueryParam("page", fmt.Sprintf("%d", page)).
		ExpectOK().ExpectElement("//table")
}

// SearchTest creates a test case for search functionality
func SearchTest(path, searchTerm string) *TestCaseBuilder {
	return GET(path).
		Named(fmt.Sprintf("Search for '%s'", searchTerm)).
		WithQueryParam("search", searchTerm).
		ExpectOK().ExpectElement("//table")
}

// HTMXUpdateTest creates a test case for HTMX partial updates
func HTMXUpdateTest(path string, formData map[string]interface{}, targetElement string) *TestCaseBuilder {
	return POST(path).
		Named(fmt.Sprintf("HTMX update %s", targetElement)).
		WithForm(formData).
		HTMX().
		HTMXTarget(targetElement).
		ExpectOK()
}

// Example usage functions for documentation

/*
Example usage:

// Basic GET request
testCase := itf.GET("/transactions").
    Named("List all transactions").
    ExpectOK().ExpectElement("//table").
    TestCase()

// POST with form data and HTMX
testCase := itf.POST("/transactions").
    Named("Create new transaction").
    WithForm(map[string]interface{}{
        "Amount": 100.00,
        "Type": "DEDUCTION",
    }).
    HTMX().
    ExpectRedirect("/transactions").
    TestCase()

// Complex test with custom assertion
testCase := itf.GET("/transactions").
    Named("Filter transactions by type").
    WithQueryParam("Type", "DEDUCTION").
    ExpectOK().
    Assert(func(t *testing.T, response *itf.Response) {
        response.HTML().Element("//tr").Exists()
        response.Contains("DEDUCTION")
    }).
    TestCase()

// Batch test cases
testCases := itf.Cases(
    itf.GET("/transactions").Named("List all").ExpectOK().ExpectElement("//table"),
    itf.POST("/transactions").Named("Create").WithForm(data).ExpectRedirect("/transactions"),
    itf.GET("/transactions/1").Named("Show transaction").ExpectOK(),
)

suite.RunCases(testCases)
*/
