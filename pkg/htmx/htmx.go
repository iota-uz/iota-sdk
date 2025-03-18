package htmx

import "net/http"

// Redirect sets the HX-Redirect header to redirect the client to a new URL.
func Redirect(w http.ResponseWriter, _ *http.Request, path string) {
	w.Header().Add("HX-Redirect", path)
	w.WriteHeader(http.StatusOK)
}

// IsHxRequest checks if the request is an HTMX request.
func IsHxRequest(r *http.Request) bool {
	return len(r.Header.Get("HX-Request")) > 0
}

// IsBoosted checks if the request was triggered by an element with hx-boost.
func IsBoosted(r *http.Request) bool {
	return len(r.Header.Get("HX-Boosted")) > 0
}

// Target returns the ID of the element that triggered the request.
func Target(r *http.Request) string {
	return r.Header.Get("HX-Target")
}

// Retarget sets the HX-Retarget header to specify a new target element.
func Retarget(w http.ResponseWriter, target string) {
	w.Header().Add("HX-Retarget", target)
}

// Location sets the HX-Location header to trigger a client-side navigation.
func Location(w http.ResponseWriter, path, target string) {
	if target == "" {
		w.Header().Add("HX-Location", path)
	} else {
		w.Header().Add("HX-Location", `{"path":"`+path+`", "target":"`+target+`"}`)
	}
}

// PushUrl sets the HX-Push-Url header to push a new URL into the browser history stack.
func PushUrl(w http.ResponseWriter, url string) {
	w.Header().Add("HX-Push-Url", url)
}

// ReplaceUrl sets the HX-Replace-Url header to replace the current URL in the browser location bar.
func ReplaceUrl(w http.ResponseWriter, url string) {
	w.Header().Add("HX-Replace-Url", url)
}

// Refresh sets the HX-Refresh header to true, instructing the client to perform a full page refresh.
func Refresh(w http.ResponseWriter) {
	w.Header().Add("HX-Refresh", "true")
}

// Reswap sets the HX-Reswap header to specify how the response will be swapped.
func Reswap(w http.ResponseWriter, swapStyle string) {
	w.Header().Add("HX-Reswap", swapStyle)
}

// Trigger sets the HX-Trigger header to trigger client-side events.
func Trigger(w http.ResponseWriter, event, detail string) {
	if detail == "" {
		w.Header().Add("HX-Trigger", event)
	} else {
		w.Header().Add("HX-Trigger", `{"`+event+`": `+detail+`}`)
	}
}

// IsHistoryRestoreRequest checks if the request is for history restoration after a miss in the local history cache.
func IsHistoryRestoreRequest(r *http.Request) bool {
	return r.Header.Get("HX-History-Restore-Request") == "true"
}

// CurrentUrl retrieves the current URL of the browser from the HX-Current-URL request header.
func CurrentUrl(r *http.Request) string {
	return r.Header.Get("HX-Current-URL")
}

// PromptResponse retrieves the user's response to an hx-prompt from the HX-Prompt request header.
func PromptResponse(r *http.Request) string {
	return r.Header.Get("HX-Prompt")
}
