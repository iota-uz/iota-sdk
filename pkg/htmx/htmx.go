package htmx

import "net/http"

func Redirect(w http.ResponseWriter, _ *http.Request, path string) {
	w.Header().Add("Hx-Redirect", path)
	w.WriteHeader(http.StatusOK)
}

func IsHxRequest(r *http.Request) bool {
	return len(r.Header.Get("Hx-Request")) > 0
}

func IsBoosted(r *http.Request) bool {
	return len(r.Header.Get("Hx-Boosted")) > 0
}
