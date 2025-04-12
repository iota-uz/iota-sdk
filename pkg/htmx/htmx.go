package htmx

import "net/http"

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

// PushUrl sets the HX-Push-Url header to push a new URL into the browser history stack.
func PushUrl(w http.ResponseWriter, url string) {
	w.Header().Add("Hx-Push-Url", url)
}

// ReplaceUrl sets the HX-Replace-Url header to replace the current URL in the browser location bar.
func ReplaceUrl(w http.ResponseWriter, url string) {
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

// Trigger sets the HX-Trigger header to trigger client-side events.
func SetTrigger(w http.ResponseWriter, event, detail string) {
	if detail == "" {
		w.Header().Add("Hx-Trigger", event)
	} else {
		w.Header().Add("Hx-Trigger", `{"`+event+`": `+detail+`}`)
	}
}

// TriggerAfterSettle sets the HX-Trigger-After-Settle header to trigger client-side events after the settle step.
func TriggerAfterSettle(w http.ResponseWriter, event, detail string) {
	if detail == "" {
		w.Header().Add("Hx-Trigger-After-Settle", event)
	} else {
		w.Header().Add("Hx-Trigger-After-Settle", `{"`+event+`": `+detail+`}`)
	}
}

// TriggerAfterSwap sets the HX-Trigger-After-Swap header to trigger client-side events after the swap step.
func TriggerAfterSwap(w http.ResponseWriter, event, detail string) {
	if detail == "" {
		w.Header().Add("Hx-Trigger-After-Swap", event)
	} else {
		w.Header().Add("Hx-Trigger-After-Swap", `{"`+event+`": `+detail+`}`)
	}
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

// Target returns the ID of the element that triggered the request.
func Target(r *http.Request) string {
	return r.Header.Get("Hx-Target")
}

// CurrentUrl retrieves the current URL of the browser from the HX-Current-URL request header.
func CurrentUrl(r *http.Request) string {
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
