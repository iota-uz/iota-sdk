package charts

import (
	"fmt"
	"strings"

	"github.com/a-h/templ"
)

// AbbreviatedCurrency generates a JavaScript formatter function that displays currency values
// with abbreviated suffixes (B for billions, M for millions, K for thousands).
//
// The generated function uses the global window.formatCurrency helper to handle abbreviation
// logic and appends the currency code.
//
// Parameters:
//   - locale: BCP 47 language tag (e.g., "en-US", "ru-RU") for number formatting
//   - currency: Currency code to append (e.g., "UZS", "USD")
//
// Example:
//
//	formatter := charts.AbbreviatedCurrency("ru-RU", "UZS")
//	// Generates: function(value) { return window.formatCurrency(value, 'ru-RU') + ' UZS'; }
//	// 1000000 → "1M UZS"
//	// 1500000 → "1.5M UZS"
func AbbreviatedCurrency(locale, currency string) templ.JSExpression {
	js := fmt.Sprintf("function(value) { return window.formatCurrency(value, '%s') + ' %s'; }", locale, currency)
	return templ.JSExpression(js)
}

// FullCurrency generates a JavaScript formatter function that displays currency values
// with full numeric representation (no abbreviation) using locale-aware number formatting.
//
// The generated function rounds the value to the nearest integer and formats it with
// thousands separators according to the locale.
//
// Parameters:
//   - locale: BCP 47 language tag (e.g., "en-US", "ru-RU") for number formatting
//   - currency: Currency code to append (e.g., "UZS", "USD")
//
// Example:
//
//	formatter := charts.FullCurrency("ru-RU", "UZS")
//	// Generates: function(value) { return Math.round(value).toLocaleString('ru-RU') + ' UZS'; }
//	// 1234567 → "1 234 567 UZS" (with non-breaking spaces in ru-RU)
func FullCurrency(locale, currency string) templ.JSExpression {
	js := fmt.Sprintf("function(value) { return Math.round(value).toLocaleString('%s') + ' %s'; }", locale, currency)
	return templ.JSExpression(js)
}

// Count generates a JavaScript formatter function that displays plain numeric values
// with locale-aware formatting (thousands separators).
//
// The generated function rounds the value to the nearest integer and formats it
// according to the locale, without any suffix.
//
// Parameters:
//   - locale: BCP 47 language tag (e.g., "en-US", "ru-RU") for number formatting
//
// Example:
//
//	formatter := charts.Count("en-US")
//	// Generates: function(value) { return Math.round(value).toLocaleString('en-US'); }
//	// 1234567 → "1,234,567"
func Count(locale string) templ.JSExpression {
	js := fmt.Sprintf("function(value) { return Math.round(value).toLocaleString('%s'); }", locale)
	return templ.JSExpression(js)
}

// LabeledCount generates a JavaScript formatter function that displays numeric values
// with a custom label suffix.
//
// The generated function rounds the value to the nearest integer, formats it with
// locale-aware thousands separators, and appends the specified label.
//
// Parameters:
//   - locale: BCP 47 language tag (e.g., "en-US", "ru-RU") for number formatting
//   - label: Text to append after the number (e.g., "Policies", "Contracts")
//
// Example:
//
//	formatter := charts.LabeledCount("en-US", "Policies")
//	// Generates: function(value) { return Math.round(value).toLocaleString('en-US') + ' Policies'; }
//	// 1234 → "1,234 Policies"
func LabeledCount(locale, label string) templ.JSExpression {
	js := fmt.Sprintf("function(value) { return Math.round(value).toLocaleString('%s') + ' %s'; }", locale, label)
	return templ.JSExpression(js)
}

// Conditional generates a JavaScript formatter function that applies different formatting
// based on a JavaScript condition.
//
// The generated function evaluates the condition against the value and applies either
// trueBranch or falseBranch formatter accordingly. This is useful for scenarios like:
//   - Different formatting for positive vs negative values
//   - Different units based on magnitude
//   - Conditional currency display
//
// Parameters:
//   - jsCondition: JavaScript condition expression (e.g., "value >= 1000000", "value < 0")
//   - trueBranch: Formatter to apply when condition is true
//   - falseBranch: Formatter to apply when condition is false
//
// Example:
//
//	// Use abbreviated format for large values, full format for small values
//	formatter := charts.Conditional(
//	    "value >= 1000000",
//	    charts.AbbreviatedCurrency("en-US", "USD"),
//	    charts.FullCurrency("en-US", "USD"),
//	)
//	// 500000 → "500,000 USD"
//	// 2500000 → "2.5M USD"
func Conditional(jsCondition string, trueBranch, falseBranch templ.JSExpression) templ.JSExpression {
	// Extract function bodies from the branch formatters
	trueBody := extractFunctionBody(string(trueBranch))
	falseBody := extractFunctionBody(string(falseBranch))

	js := fmt.Sprintf("function(value) { if (%s) { %s } else { %s } }",
		jsCondition, trueBody, falseBody)
	return templ.JSExpression(js)
}

// Percentage generates a JavaScript formatter function that displays values as percentages
// with a specified number of decimal places.
//
// The generated function formats the value using toFixed() for decimal precision and
// appends a percent sign.
//
// Parameters:
//   - decimals: Number of decimal places to display (e.g., 0 for "50%", 1 for "50.5%", 2 for "50.25%")
//
// Example:
//
//	formatter := charts.Percentage(1)
//	// Generates: function(value) { return value.toFixed(1) + '%'; }
//	// 45.67 → "45.7%"
//	// 100 → "100.0%"
func Percentage(decimals int) templ.JSExpression {
	js := fmt.Sprintf("function(value) { return value.toFixed(%d) + '%%'; }", decimals)
	return templ.JSExpression(js)
}

// ConditionalCurrency returns a formatter that uses abbreviated format for large values
// and full format for small values, based on a threshold.
//
// Parameters:
//   - locale: BCP 47 language tag for number formatting
//   - currency: Currency code to append
//   - threshold: Value above which abbreviated format is used
//
// Example:
//
//	formatter := charts.ConditionalCurrency("en-US", "USD", 1000000)
//	// 500000 → "500,000 USD"
//	// 2500000 → "2.5M USD"
func ConditionalCurrency(locale, currency string, threshold float64) templ.JSExpression {
	condition := fmt.Sprintf("value >= %f", threshold)
	return Conditional(
		condition,
		AbbreviatedCurrency(locale, currency),
		FullCurrency(locale, currency),
	)
}

// extractFunctionBody extracts the body of a JavaScript function string.
// Assumes input is in the form: "function(params) { body }"
func extractFunctionBody(fn string) string {
	// Find the opening brace
	start := strings.Index(fn, "{")
	if start == -1 {
		return "return value;" // fallback
	}

	// Find the matching closing brace (assumes balanced braces)
	end := strings.LastIndex(fn, "}")
	if end == -1 || end <= start {
		return "return value;" // fallback
	}

	// Extract body between braces and trim whitespace
	body := strings.TrimSpace(fn[start+1 : end])
	return body
}
