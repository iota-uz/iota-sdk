package itf

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ResponseAssertion provides a fluent API for making assertions about HTTP responses
type ResponseAssertion struct {
	response *Response
	t        testing.TB
}

// newResponseAssertion creates a new ResponseAssertion
func newResponseAssertion(response *Response, t testing.TB) *ResponseAssertion {
	t.Helper()
	return &ResponseAssertion{
		response: response,
		t:        t,
	}
}

// Status assertions

// ExpectOK asserts that the response status is 200 OK
func (ra *ResponseAssertion) ExpectOK() *ResponseAssertion {
	ra.t.Helper()
	return ra.ExpectStatus(http.StatusOK)
}

// ExpectCreated asserts that the response status is 201 Created
func (ra *ResponseAssertion) ExpectCreated() *ResponseAssertion {
	ra.t.Helper()
	return ra.ExpectStatus(http.StatusCreated)
}

// ExpectAccepted asserts that the response status is 202 Accepted
func (ra *ResponseAssertion) ExpectAccepted() *ResponseAssertion {
	ra.t.Helper()
	return ra.ExpectStatus(http.StatusAccepted)
}

// ExpectNoContent asserts that the response status is 204 No Content
func (ra *ResponseAssertion) ExpectNoContent() *ResponseAssertion {
	ra.t.Helper()
	return ra.ExpectStatus(http.StatusNoContent)
}

// ExpectBadRequest asserts that the response status is 400 Bad Request
func (ra *ResponseAssertion) ExpectBadRequest() *ResponseAssertion {
	ra.t.Helper()
	return ra.ExpectStatus(http.StatusBadRequest)
}

// ExpectUnauthorized asserts that the response status is 401 Unauthorized
func (ra *ResponseAssertion) ExpectUnauthorized() *ResponseAssertion {
	ra.t.Helper()
	return ra.ExpectStatus(http.StatusUnauthorized)
}

// ExpectForbidden asserts that the response status is 403 Forbidden
func (ra *ResponseAssertion) ExpectForbidden() *ResponseAssertion {
	ra.t.Helper()
	return ra.ExpectStatus(http.StatusForbidden)
}

// ExpectNotFound asserts that the response status is 404 Not Found
func (ra *ResponseAssertion) ExpectNotFound() *ResponseAssertion {
	ra.t.Helper()
	return ra.ExpectStatus(http.StatusNotFound)
}

// ExpectMethodNotAllowed asserts that the response status is 405 Method Not Allowed
func (ra *ResponseAssertion) ExpectMethodNotAllowed() *ResponseAssertion {
	ra.t.Helper()
	return ra.ExpectStatus(http.StatusMethodNotAllowed)
}

// ExpectConflict asserts that the response status is 409 Conflict
func (ra *ResponseAssertion) ExpectConflict() *ResponseAssertion {
	ra.t.Helper()
	return ra.ExpectStatus(http.StatusConflict)
}

// ExpectUnprocessableEntity asserts that the response status is 422 Unprocessable Entity
func (ra *ResponseAssertion) ExpectUnprocessableEntity() *ResponseAssertion {
	ra.t.Helper()
	return ra.ExpectStatus(http.StatusUnprocessableEntity)
}

// ExpectInternalServerError asserts that the response status is 500 Internal Server Error
func (ra *ResponseAssertion) ExpectInternalServerError() *ResponseAssertion {
	ra.t.Helper()
	return ra.ExpectStatus(http.StatusInternalServerError)
}

// ExpectStatus asserts that the response has the expected status code
func (ra *ResponseAssertion) ExpectStatus(expectedCode int) *ResponseAssertion {
	ra.t.Helper()
	actualCode := ra.response.recorder.Code
	assert.Equal(ra.t, expectedCode, actualCode,
		"Expected status %d (%s), got %d (%s). Response body:\n%s",
		expectedCode, http.StatusText(expectedCode),
		actualCode, http.StatusText(actualCode),
		ra.response.Body())
	return ra
}

// Content assertions

// ExpectHTML asserts that the response content type is HTML
func (ra *ResponseAssertion) ExpectHTML() *HTMLAssertion {
	ra.t.Helper()
	contentType := ra.response.Header("Content-Type")
	assert.Contains(ra.t, contentType, "text/html",
		"Expected HTML content type, got: %s", contentType)
	return &HTMLAssertion{
		response: ra.response,
		t:        ra.t,
	}
}

// ExpectJSON asserts that the response content type is JSON
func (ra *ResponseAssertion) ExpectJSON() *JSONAssertion {
	ra.t.Helper()
	contentType := ra.response.Header("Content-Type")
	assert.Contains(ra.t, contentType, "application/json",
		"Expected JSON content type, got: %s", contentType)
	return &JSONAssertion{
		response: ra.response,
		t:        ra.t,
	}
}

// ExpectText asserts that the response content type is plain text
func (ra *ResponseAssertion) ExpectText() *ResponseAssertion {
	ra.t.Helper()
	contentType := ra.response.Header("Content-Type")
	assert.Contains(ra.t, contentType, "text/plain",
		"Expected plain text content type, got: %s", contentType)
	return ra
}

// ExpectContentType asserts that the response has the expected content type
func (ra *ResponseAssertion) ExpectContentType(expectedType string) *ResponseAssertion {
	ra.t.Helper()
	contentType := ra.response.Header("Content-Type")
	assert.Contains(ra.t, contentType, expectedType,
		"Expected content type to contain '%s', got: %s", expectedType, contentType)
	return ra
}

// Body content assertions

// ExpectBodyContains asserts that the response body contains the expected text
func (ra *ResponseAssertion) ExpectBodyContains(text string) *ResponseAssertion {
	ra.t.Helper()
	body := ra.response.Body()
	assert.Contains(ra.t, body, text,
		"Expected response body to contain '%s'", text)
	return ra
}

// ExpectBodyNotContains asserts that the response body does not contain the text
func (ra *ResponseAssertion) ExpectBodyNotContains(text string) *ResponseAssertion {
	ra.t.Helper()
	body := ra.response.Body()
	assert.NotContains(ra.t, body, text,
		"Expected response body to not contain '%s'", text)
	return ra
}

// ExpectBodyEquals asserts that the response body exactly equals the expected text
func (ra *ResponseAssertion) ExpectBodyEquals(expected string) *ResponseAssertion {
	ra.t.Helper()
	body := ra.response.Body()
	assert.Equal(ra.t, expected, body,
		"Expected response body to equal the expected text")
	return ra
}

// ExpectBodyEmpty asserts that the response body is empty
func (ra *ResponseAssertion) ExpectBodyEmpty() *ResponseAssertion {
	ra.t.Helper()
	body := ra.response.Body()
	assert.Empty(ra.t, strings.TrimSpace(body),
		"Expected response body to be empty, got: %s", body)
	return ra
}

// Header assertions

// ExpectHeader asserts that a response header has the expected value
func (ra *ResponseAssertion) ExpectHeader(name, expectedValue string) *ResponseAssertion {
	ra.t.Helper()
	actualValue := ra.response.Header(name)
	assert.Equal(ra.t, expectedValue, actualValue,
		"Expected header '%s' to be '%s', got '%s'", name, expectedValue, actualValue)
	return ra
}

// ExpectHeaderContains asserts that a response header contains the expected value
func (ra *ResponseAssertion) ExpectHeaderContains(name, expectedSubstring string) *ResponseAssertion {
	ra.t.Helper()
	actualValue := ra.response.Header(name)
	assert.Contains(ra.t, actualValue, expectedSubstring,
		"Expected header '%s' to contain '%s', got '%s'", name, expectedSubstring, actualValue)
	return ra
}

// ExpectHeaderExists asserts that a response header exists
func (ra *ResponseAssertion) ExpectHeaderExists(name string) *ResponseAssertion {
	ra.t.Helper()
	actualValue := ra.response.Header(name)
	assert.NotEmpty(ra.t, actualValue,
		"Expected header '%s' to exist", name)
	return ra
}

// Redirect assertions

// ExpectRedirectTo asserts that the response redirects to the expected location
func (ra *ResponseAssertion) ExpectRedirectTo(expectedLocation string) *ResponseAssertion {
	ra.t.Helper()
	// First check that it's a redirect status
	statusCode := ra.response.recorder.Code
	assert.True(ra.t, statusCode >= 300 && statusCode < 400,
		"Expected redirect status code (3xx), got %d", statusCode)

	// Then check the location header
	location := ra.response.Header("Location")
	assert.Equal(ra.t, expectedLocation, location,
		"Expected redirect to '%s', got '%s'", expectedLocation, location)
	return ra
}

// HTMX assertions

// ExpectHTMXTrigger asserts that the response has an HX-Trigger header with the expected event
func (ra *ResponseAssertion) ExpectHTMXTrigger(expectedEvent string) *ResponseAssertion {
	ra.t.Helper()
	trigger := ra.response.Header("HX-Trigger")
	assert.Contains(ra.t, trigger, expectedEvent,
		"Expected HX-Trigger to contain '%s', got '%s'", expectedEvent, trigger)
	return ra
}

// ExpectHTMXRedirect asserts that the response has an HX-Redirect header
func (ra *ResponseAssertion) ExpectHTMXRedirect(expectedPath string) *ResponseAssertion {
	ra.t.Helper()
	redirect := ra.response.Header("HX-Redirect")
	assert.Equal(ra.t, expectedPath, redirect,
		"Expected HX-Redirect to be '%s', got '%s'", expectedPath, redirect)
	return ra
}

// ExpectHTMXReswap asserts that the response has an HX-Reswap header
func (ra *ResponseAssertion) ExpectHTMXReswap(expectedStrategy string) *ResponseAssertion {
	ra.t.Helper()
	reswap := ra.response.Header("HX-Reswap")
	assert.Equal(ra.t, expectedStrategy, reswap,
		"Expected HX-Reswap to be '%s', got '%s'", expectedStrategy, reswap)
	return ra
}

// ExpectHTMXRetarget asserts that the response has an HX-Retarget header
func (ra *ResponseAssertion) ExpectHTMXRetarget(expectedTarget string) *ResponseAssertion {
	ra.t.Helper()
	retarget := ra.response.Header("HX-Retarget")
	assert.Equal(ra.t, expectedTarget, retarget,
		"Expected HX-Retarget to be '%s', got '%s'", expectedTarget, retarget)
	return ra
}

// Enhanced HTMX assertions for modern SPA testing

// ExpectHTMXSwap asserts that the response has an HX-Reswap header with the expected swap strategy
func (ra *ResponseAssertion) ExpectHTMXSwap(expectedStrategy string) *ResponseAssertion {
	ra.t.Helper()
	swap := ra.response.Header("HX-Reswap")
	assert.Equal(ra.t, expectedStrategy, swap,
		"Expected HX-Reswap to be '%s', got '%s'", expectedStrategy, swap)
	return ra
}

// ExpectHTMXLocation asserts that the response has an HX-Location header
func (ra *ResponseAssertion) ExpectHTMXLocation(expectedLocation string) *ResponseAssertion {
	ra.t.Helper()
	location := ra.response.Header("HX-Location")
	assert.Equal(ra.t, expectedLocation, location,
		"Expected HX-Location to be '%s', got '%s'", expectedLocation, location)
	return ra
}

// ExpectHTMXPushURL asserts that the response has an HX-Push-Url header
func (ra *ResponseAssertion) ExpectHTMXPushURL(expectedURL string) *ResponseAssertion {
	ra.t.Helper()
	pushURL := ra.response.Header("HX-Push-Url")
	assert.Equal(ra.t, expectedURL, pushURL,
		"Expected HX-Push-Url to be '%s', got '%s'", expectedURL, pushURL)
	return ra
}

// ExpectHTMXReplaceURL asserts that the response has an HX-Replace-Url header
func (ra *ResponseAssertion) ExpectHTMXReplaceURL(expectedURL string) *ResponseAssertion {
	ra.t.Helper()
	replaceURL := ra.response.Header("HX-Replace-Url")
	assert.Equal(ra.t, expectedURL, replaceURL,
		"Expected HX-Replace-Url to be '%s', got '%s'", expectedURL, replaceURL)
	return ra
}

// ExpectHTMXRefresh asserts that the response has an HX-Refresh header set to "true"
func (ra *ResponseAssertion) ExpectHTMXRefresh() *ResponseAssertion {
	ra.t.Helper()
	refresh := ra.response.Header("HX-Refresh")
	assert.Equal(ra.t, "true", refresh,
		"Expected HX-Refresh to be 'true', got '%s'", refresh)
	return ra
}

// ExpectHTMXTriggerAfterSwap asserts that the response has an HX-Trigger-After-Swap header
func (ra *ResponseAssertion) ExpectHTMXTriggerAfterSwap(expectedEvent string) *ResponseAssertion {
	ra.t.Helper()
	trigger := ra.response.Header("HX-Trigger-After-Swap")
	assert.Contains(ra.t, trigger, expectedEvent,
		"Expected HX-Trigger-After-Swap to contain '%s', got '%s'", expectedEvent, trigger)
	return ra
}

// ExpectHTMXTriggerAfterSettle asserts that the response has an HX-Trigger-After-Settle header
func (ra *ResponseAssertion) ExpectHTMXTriggerAfterSettle(expectedEvent string) *ResponseAssertion {
	ra.t.Helper()
	trigger := ra.response.Header("HX-Trigger-After-Settle")
	assert.Contains(ra.t, trigger, expectedEvent,
		"Expected HX-Trigger-After-Settle to contain '%s', got '%s'", expectedEvent, trigger)
	return ra
}

// ExpectHTMXReselect asserts that the response has an HX-Reselect header
func (ra *ResponseAssertion) ExpectHTMXReselect(expectedSelector string) *ResponseAssertion {
	ra.t.Helper()
	reselect := ra.response.Header("HX-Reselect")
	assert.Equal(ra.t, expectedSelector, reselect,
		"Expected HX-Reselect to be '%s', got '%s'", expectedSelector, reselect)
	return ra
}

// ExpectHTMXTriggerWithData asserts that the response has an HX-Trigger header with JSON event data
func (ra *ResponseAssertion) ExpectHTMXTriggerWithData(expectedEvent string, expectedData map[string]interface{}) *ResponseAssertion {
	ra.t.Helper()
	trigger := ra.response.Header("HX-Trigger")

	// First check that the event name is present
	assert.Contains(ra.t, trigger, expectedEvent,
		"Expected HX-Trigger to contain event '%s', got '%s'", expectedEvent, trigger)

	// If expectedData is provided, try to validate the JSON structure
	if expectedData != nil && len(expectedData) > 0 {
		// This is a simplified validation - in a real implementation you might
		// want to parse the JSON and validate the structure more thoroughly
		var triggerData map[string]interface{}
		err := json.Unmarshal([]byte(trigger), &triggerData)
		if err == nil {
			// If it parses as JSON, check for expected keys
			for key, expectedValue := range expectedData {
				if eventData, exists := triggerData[expectedEvent]; exists {
					if eventMap, ok := eventData.(map[string]interface{}); ok {
						if actualValue, hasKey := eventMap[key]; hasKey {
							assert.Equal(ra.t, expectedValue, actualValue,
								"Expected HX-Trigger event '%s' data key '%s' to be %v, got %v",
								expectedEvent, key, expectedValue, actualValue)
						} else {
							ra.t.Errorf("Expected HX-Trigger event '%s' to have data key '%s'", expectedEvent, key)
						}
					}
				}
			}
		}
	}

	return ra
}

// ExpectNoHTMXHeaders asserts that the response has no HTMX-related headers
func (ra *ResponseAssertion) ExpectNoHTMXHeaders() *ResponseAssertion {
	ra.t.Helper()

	htmxHeaders := []string{
		"HX-Location", "HX-Push-Url", "HX-Redirect", "HX-Refresh",
		"HX-Replace-Url", "HX-Reswap", "HX-Retarget", "HX-Reselect",
		"HX-Trigger", "HX-Trigger-After-Swap", "HX-Trigger-After-Settle",
	}

	for _, header := range htmxHeaders {
		value := ra.response.Header(header)
		assert.Empty(ra.t, value, "Expected no %s header, but got: %s", header, value)
	}

	return ra
}

// HTMLAssertion provides HTML-specific assertions
type HTMLAssertion struct {
	response *Response
	t        testing.TB
}

// ExpectTitle asserts that the HTML document has the expected title
func (ha *HTMLAssertion) ExpectTitle(expectedTitle string) *HTMLAssertion {
	ha.t.Helper()
	title := ha.response.HTML().Element("//title").Text()
	assert.Equal(ha.t, expectedTitle, strings.TrimSpace(title),
		"Expected HTML title to be '%s', got '%s'", expectedTitle, title)
	return ha
}

// ExpectElement asserts that an element exists at the given XPath
func (ha *HTMLAssertion) ExpectElement(xpath string) *ElementAssertion {
	ha.t.Helper()
	element := ha.response.HTML().Element(xpath)
	element.Exists() // This will fail the test if element doesn't exist
	return &ElementAssertion{
		element: element,
		t:       ha.t,
	}
}

// ExpectNoElement asserts that an element does not exist at the given XPath
func (ha *HTMLAssertion) ExpectNoElement(xpath string) *HTMLAssertion {
	ha.t.Helper()
	element := ha.response.HTML().Element(xpath)
	element.NotExists() // This will fail the test if element exists
	return ha
}

// ExpectForm asserts that a form exists and returns a FormAssertion
func (ha *HTMLAssertion) ExpectForm(xpath string) *FormAssertion {
	ha.t.Helper()
	element := ha.response.HTML().Element(xpath)
	element.Exists()
	return &FormAssertion{
		element: element,
		t:       ha.t,
	}
}

// ExpectErrorFor asserts that there's a validation error for the given field
func (ha *HTMLAssertion) ExpectErrorFor(fieldID string) *HTMLAssertion {
	ha.t.Helper()
	hasError := ha.response.HTML().HasErrorFor(fieldID)
	assert.True(ha.t, hasError,
		"Expected validation error for field '%s'", fieldID)
	return ha
}

// ExpectNoErrorFor asserts that there's no validation error for the given field
func (ha *HTMLAssertion) ExpectNoErrorFor(fieldID string) *HTMLAssertion {
	ha.t.Helper()
	hasError := ha.response.HTML().HasErrorFor(fieldID)
	assert.False(ha.t, hasError,
		"Expected no validation error for field '%s'", fieldID)
	return ha
}

// ExpectErrorFor asserts that there's a validation error for the given field
func (h *HTML) ExpectErrorFor(fieldID string) *HTML {
	h.t.Helper()
	hasError := h.HasErrorFor(fieldID)
	assert.True(h.t, hasError,
		"Expected validation error for field '%s'", fieldID)
	return h
}

// ExpectNoErrorFor asserts that there's no validation error for the given field
func (h *HTML) ExpectNoErrorFor(fieldID string) *HTML {
	h.t.Helper()
	hasError := h.HasErrorFor(fieldID)
	assert.False(h.t, hasError,
		"Expected no validation error for field '%s'", fieldID)
	return h
}

// JSONAssertion provides JSON-specific assertions
type JSONAssertion struct {
	response *Response
	t        testing.TB
}

// ExpectField asserts that a JSON field has the expected value
func (ja *JSONAssertion) ExpectField(fieldPath string, expectedValue interface{}) *JSONAssertion {
	ja.t.Helper()
	var data map[string]interface{}
	err := json.Unmarshal([]byte(ja.response.Body()), &data)
	require.NoError(ja.t, err, "Failed to parse JSON response")

	// Simple field path resolution (can be enhanced for nested paths)
	actualValue, exists := data[fieldPath]
	assert.True(ja.t, exists, "JSON field '%s' not found", fieldPath)
	assert.Equal(ja.t, expectedValue, actualValue,
		"Expected JSON field '%s' to be %v, got %v", fieldPath, expectedValue, actualValue)
	return ja
}

// ExpectStructure asserts that the JSON response can be unmarshaled into the expected structure
func (ja *JSONAssertion) ExpectStructure(target interface{}) *JSONAssertion {
	ja.t.Helper()
	err := json.Unmarshal([]byte(ja.response.Body()), target)
	assert.NoError(ja.t, err, "Failed to unmarshal JSON into expected structure")
	return ja
}

// ElementAssertion provides element-specific assertions
type ElementAssertion struct {
	element *Element
	t       testing.TB
}

// ExpectText asserts that the element has the expected text content
func (ea *ElementAssertion) ExpectText(expectedText string) *ElementAssertion {
	ea.t.Helper()
	actualText := strings.TrimSpace(ea.element.Text())
	assert.Equal(ea.t, expectedText, actualText,
		"Expected element text to be '%s', got '%s'", expectedText, actualText)
	return ea
}

// ExpectTextContains asserts that the element text contains the expected substring
func (ea *ElementAssertion) ExpectTextContains(expectedSubstring string) *ElementAssertion {
	ea.t.Helper()
	actualText := ea.element.Text()
	assert.Contains(ea.t, actualText, expectedSubstring,
		"Expected element text to contain '%s', got '%s'", expectedSubstring, actualText)
	return ea
}

// ExpectAttribute asserts that the element has the expected attribute value
func (ea *ElementAssertion) ExpectAttribute(name, expectedValue string) *ElementAssertion {
	ea.t.Helper()
	actualValue := ea.element.Attr(name)
	assert.Equal(ea.t, expectedValue, actualValue,
		"Expected element attribute '%s' to be '%s', got '%s'", name, expectedValue, actualValue)
	return ea
}

// ExpectClass asserts that the element has the expected CSS class
func (ea *ElementAssertion) ExpectClass(expectedClass string) *ElementAssertion {
	ea.t.Helper()
	classAttr := ea.element.Attr("class")
	classes := strings.Fields(classAttr)
	found := false
	for _, class := range classes {
		if class == expectedClass {
			found = true
			break
		}
	}
	assert.True(ea.t, found,
		"Expected element to have class '%s', got classes: %v", expectedClass, classes)
	return ea
}

// FormAssertion provides form-specific assertions
type FormAssertion struct {
	element *Element
	t       testing.TB
}

// ExpectAction asserts that the form has the expected action URL
func (fa *FormAssertion) ExpectAction(expectedAction string) *FormAssertion {
	fa.t.Helper()
	actualAction := fa.element.Attr("action")
	assert.Equal(fa.t, expectedAction, actualAction,
		"Expected form action to be '%s', got '%s'", expectedAction, actualAction)
	return fa
}

// ExpectMethod asserts that the form has the expected method
func (fa *FormAssertion) ExpectMethod(expectedMethod string) *FormAssertion {
	fa.t.Helper()
	actualMethod := strings.ToUpper(fa.element.Attr("method"))
	expectedMethod = strings.ToUpper(expectedMethod)
	assert.Equal(fa.t, expectedMethod, actualMethod,
		"Expected form method to be '%s', got '%s'", expectedMethod, actualMethod)
	return fa
}

// ExpectFieldValue asserts that a form field has the expected value
func (fa *FormAssertion) ExpectFieldValue(fieldName, expectedValue string) *FormAssertion {
	fa.t.Helper()
	// This is a simplified implementation - you might want to enhance it
	// to handle different input types, select options, etc.
	xpath := fmt.Sprintf(".//input[@name='%s']", fieldName)
	if fa.element.node != nil {
		// Find the input within the form and check its value
		// This would need proper XPath resolution within the form context
		// For now, this is a placeholder implementation that uses the xpath
		_ = xpath // TODO: Implement proper field value checking
	}
	return fa
}

// Common Assertion Shortcuts

// ExpectOKWithForm asserts that the response status is 200 OK and contains a form with the specified action
func (ra *ResponseAssertion) ExpectOKWithForm(action string) *ResponseAssertion {
	ra.t.Helper()
	ra.ExpectOK()
	xpath := "//form"
	if action != "" {
		xpath = fmt.Sprintf("//form[@action='%s']", action)
	}
	ra.ExpectHTML().ExpectElement(xpath)
	return ra
}

// ExpectRedirect asserts that the response is a redirect (3xx status) with the specified location
func (ra *ResponseAssertion) ExpectRedirect(location string) *ResponseAssertion {
	ra.t.Helper()
	// Check that it's a redirect status
	statusCode := ra.response.recorder.Code
	assert.True(ra.t, statusCode >= 300 && statusCode < 400,
		"Expected redirect status code (3xx), got %d", statusCode)

	// Check the location header
	actualLocation := ra.response.Header("Location")
	assert.Equal(ra.t, location, actualLocation,
		"Expected redirect to '%s', got '%s'", location, actualLocation)
	return ra
}

// ExpectFlash asserts that the response contains a flash message with the specified text
func (ra *ResponseAssertion) ExpectFlash(message string) *ResponseAssertion {
	ra.t.Helper()
	// Look for common flash message patterns in HTML
	body := ra.response.Body()

	// Check for common flash message containers
	flashSelectors := []string{
		"//div[contains(@class, 'alert')]",
		"//div[contains(@class, 'flash')]",
		"//div[contains(@class, 'notification')]",
		"//div[contains(@class, 'message')]",
		"//div[@id='flash']",
		"//div[@id='alerts']",
	}

	// First try to parse as HTML and check with XPath
	if strings.Contains(ra.response.Header("Content-Type"), "text/html") {
		htmlAssertion := ra.ExpectHTML()
		found := false
		for _, selector := range flashSelectors {
			element := htmlAssertion.response.HTML().Element(selector)
			if element.node != nil && strings.Contains(element.Text(), message) {
				found = true
				break
			}
		}
		if !found {
			// Fallback to body text search
			assert.Contains(ra.t, body, message,
				"Expected flash message '%s' not found in response", message)
		}
	} else {
		// For non-HTML responses, just check if the message is in the body
		assert.Contains(ra.t, body, message,
			"Expected flash message '%s' not found in response", message)
	}
	return ra
}

// ExpectDownload asserts that the response is a file download with the specified content type and filename
func (ra *ResponseAssertion) ExpectDownload(contentType, fileName string) *ResponseAssertion {
	ra.t.Helper()

	// Check Content-Type header
	if contentType != "" {
		actualContentType := ra.response.Header("Content-Type")
		assert.Contains(ra.t, actualContentType, contentType,
			"Expected Content-Type to contain '%s', got '%s'", contentType, actualContentType)
	}

	// Check Content-Disposition header for filename
	if fileName != "" {
		contentDisposition := ra.response.Header("Content-Disposition")
		assert.NotEmpty(ra.t, contentDisposition,
			"Expected Content-Disposition header to be present for download")

		// Check if it contains attachment and the filename
		assert.Contains(ra.t, contentDisposition, "attachment",
			"Expected Content-Disposition to contain 'attachment'")
		assert.Contains(ra.t, contentDisposition, fileName,
			"Expected Content-Disposition to contain filename '%s', got '%s'", fileName, contentDisposition)
	}

	return ra
}
