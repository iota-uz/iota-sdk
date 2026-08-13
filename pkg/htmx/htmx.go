// Package htmx provides this package.
package htmx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf16"
)

// ================= Setters =================

// Redirect sets the HX-Redirect header to redirect the client to a new URL.
func Redirect(w http.ResponseWriter, path string) {
	w.Header().Add("Hx-Redirect", path)
	w.WriteHeader(http.StatusOK)
}

// Retarget sets the HX-Retarget header to specify a new target element.
func Retarget(w http.ResponseWriter, target string) {
	w.Header().Add("Hx-Retarget", target)
}

// Reselect sets the HX-Reselect header to specify which part of the response should be swapped in.
func Reselect(w http.ResponseWriter, selector string) {
	w.Header().Add("Hx-Reselect", selector)
}

// Location sets the HX-Location header to trigger a client-side navigation.
func Location(w http.ResponseWriter, path, target string) {
	if target == "" {
		w.Header().Add("Hx-Location", path)
	} else {
		w.Header().Add("Hx-Location", `{"path":"`+path+`", "target":"`+target+`"}`)
	}
}

// PushURL sets the HX-Push-Url header to push a new URL into the browser history stack.
func PushURL(w http.ResponseWriter, url string) {
	w.Header().Add("Hx-Push-Url", url)
}

// ReplaceURL sets the HX-Replace-Url header to replace the current URL in the browser location bar.
func ReplaceURL(w http.ResponseWriter, url string) {
	w.Header().Add("Hx-Replace-Url", url)
}

// Refresh sets the HX-Refresh header to true, instructing the client to perform a full page refresh.
func Refresh(w http.ResponseWriter) {
	w.Header().Add("Hx-Refresh", "true")
}

// Reswap sets the HX-Reswap header to specify how the response will be swapped.
func Reswap(w http.ResponseWriter, swapStyle string) {
	w.Header().Add("Hx-Reswap", swapStyle)
}

// escapeNonASCII rewrites every rune above U+007F as a \uXXXX escape sequence.
//
// Header values are transported as bytes and decoded by browsers as ISO-8859-1
// (XHR/fetch), so raw UTF-8 reaches htmx as mojibake. \uXXXX is valid JSON and
// JSON.parse restores the original string, so the payload survives intact while
// the header itself stays pure ASCII. Runes outside the BMP are emitted as a
// UTF-16 surrogate pair, as JSON requires.
//
// Escaping is applied to an assembled JSON document rather than to its parts:
// non-ASCII cannot legally appear outside a JSON string literal, so this cannot
// corrupt a valid document, and a caller-supplied payload is never re-parsed or
// re-marshalled. Invalid UTF-8 in the input is escaped as U+FFFD.
func escapeNonASCII(s string) string {
	if isASCII(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r < 0x80:
			b.WriteRune(r)
		case r > 0xFFFF:
			high, low := utf16.EncodeRune(r)
			fmt.Fprintf(&b, `\u%04x\u%04x`, high, low)
		default:
			fmt.Fprintf(&b, `\u%04x`, r)
		}
	}
	return b.String()
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// triggerHeaderValue builds the value for an HX-Trigger* header.
//
// An empty detail keeps the bare event-name form htmx accepts for detail-less
// events. Otherwise the event name is marshalled as a JSON string and the
// caller's detail is embedded verbatim as the event payload.
func triggerHeaderValue(event, detail string) string {
	if detail == "" {
		return event
	}
	name, _ := json.Marshal(event) // marshalling a string cannot fail
	return escapeNonASCII(`{` + string(name) + `:` + detail + `}`)
}

// SetTrigger sets the HX-Trigger header to trigger client-side events.
func SetTrigger(w http.ResponseWriter, event, detail string) {
	w.Header().Add("Hx-Trigger", triggerHeaderValue(event, detail))
}

// TriggerAfterSettle sets the HX-Trigger-After-Settle header to trigger client-side events after the settle step.
func TriggerAfterSettle(w http.ResponseWriter, event, detail string) {
	w.Header().Add("Hx-Trigger-After-Settle", triggerHeaderValue(event, detail))
}

// TriggerAfterSwap sets the HX-Trigger-After-Swap header to trigger client-side events after the swap step.
func TriggerAfterSwap(w http.ResponseWriter, event, detail string) {
	w.Header().Add("Hx-Trigger-After-Swap", triggerHeaderValue(event, detail))
}

// ================= Getters =================

// IsHxRequest checks if the request is an HTMX request.
func IsHxRequest(r *http.Request) bool {
	return r.Header.Get("Hx-Request") == "true"
}

// IsBoosted checks if the request was triggered by an element with hx-boost.
func IsBoosted(r *http.Request) bool {
	return r.Header.Get("Hx-Boosted") == "true"
}

// IsHistoryRestoreRequest checks if the request is for history restoration after a miss in the local history cache.
func IsHistoryRestoreRequest(r *http.Request) bool {
	return r.Header.Get("Hx-History-Restore-Request") == "true"
}

// WantsFullPage reports whether the handler should render a full HTML document
// (layout + <head>) rather than an HTMX fragment.
//
// It is true for a normal navigation and for a history-restore request: htmx
// fetches the latter on a local history-cache miss and swaps it into the
// history element, so it must receive the whole page or the layout shell is
// lost. Branch on this instead of a bare IsHxRequest check when deciding
// fragment-vs-page, so history restoration keeps the shell and styling intact.
func WantsFullPage(r *http.Request) bool {
	return !IsHxRequest(r) || IsHistoryRestoreRequest(r)
}

// Target returns the ID of the element that triggered the request.
func Target(r *http.Request) string {
	return r.Header.Get("Hx-Target")
}

// CurrentURL retrieves the current URL of the browser from the HX-Current-URL request header.
func CurrentURL(r *http.Request) string {
	return r.Header.Get("Hx-Current-Url")
}

// PromptResponse retrieves the user's response to an hx-prompt from the HX-Prompt request header.
func PromptResponse(r *http.Request) string {
	return r.Header.Get("Hx-Prompt")
}

// TriggerName retrieves the name of the triggered element from the HX-Trigger-Name request header.
func TriggerName(r *http.Request) string {
	return r.Header.Get("Hx-Trigger-Name")
}

// Trigger retrieves the ID of the triggered element from the HX-Trigger request header.
func Trigger(r *http.Request) string {
	return r.Header.Get("Hx-Trigger")
}

// SSEEvent creates a Server-Sent Event (SSE) formatted string.
func SSEEvent(html string, event ...string) string {
	var res string
	if len(event) > 2 {
		panic("too many events")
	}
	if len(event) > 0 {
		res = fmt.Sprintf("event: %s\n", event[0])
	}
	res += fmt.Sprintf("data: %s\n", html)
	return res
}

// ================= Toast Notifications =================

// ToastVariant represents the type of toast notification
type ToastVariant string

const (
	ToastVariantSuccess ToastVariant = "success"
	ToastVariantError   ToastVariant = "error"
	ToastVariantDanger  ToastVariant = "danger"
	ToastVariantWarning ToastVariant = "warning"
	ToastVariantInfo    ToastVariant = "info"
)

type toastDetail struct {
	Variant ToastVariant `json:"variant"`
	Title   string       `json:"title"`
	Message string       `json:"message"`
}

// toastPayload marshals the toast detail so that quotes, backslashes and control
// characters in a title or message stay valid JSON instead of breaking the
// client-side parse.
func toastPayload(variant ToastVariant, title, message string) string {
	detail, _ := json.Marshal(toastDetail{ // marshalling strings cannot fail
		Variant: variant,
		Title:   title,
		Message: message,
	})
	return string(detail)
}

// TriggerToast triggers a toast notification with the specified variant, title, and message.
// This uses the HX-Trigger header to dispatch a 'notify' event that the toast container listens for.
func TriggerToast(w http.ResponseWriter, variant ToastVariant, title, message string) {
	SetTrigger(w, "notify", toastPayload(variant, title, message))
}

// TriggerToastAfterSwap triggers a toast notification after the swap step.
func TriggerToastAfterSwap(w http.ResponseWriter, variant ToastVariant, title, message string) {
	TriggerAfterSwap(w, "notify", toastPayload(variant, title, message))
}

// TriggerToastAfterSettle triggers a toast notification after the settle step.
func TriggerToastAfterSettle(w http.ResponseWriter, variant ToastVariant, title, message string) {
	TriggerAfterSettle(w, "notify", toastPayload(variant, title, message))
}

// Convenience functions for common toast types

// ToastSuccess triggers a success toast notification.
func ToastSuccess(w http.ResponseWriter, title, message string) {
	TriggerToast(w, ToastVariantSuccess, title, message)
}

// ToastError triggers an error toast notification.
func ToastError(w http.ResponseWriter, title, message string) {
	TriggerToast(w, ToastVariantError, title, message)
}

// ToastWarning triggers a warning toast notification.
func ToastWarning(w http.ResponseWriter, title, message string) {
	TriggerToast(w, ToastVariantWarning, title, message)
}

// ToastInfo triggers an info toast notification.
func ToastInfo(w http.ResponseWriter, title, message string) {
	TriggerToast(w, ToastVariantInfo, title, message)
}
