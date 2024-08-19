package controllers

import "net/http"

func hxRedirect(w http.ResponseWriter, _ *http.Request, path string) {
	w.Header().Add("HX-Redirect", path)
	w.WriteHeader(http.StatusOK)
}

func redirect(w http.ResponseWriter, r *http.Request, path string) {
	isHxRequest := len(r.Header.Get("HX-Request")) > 0
	if isHxRequest {
		hxRedirect(w, r, path)
		return
	}
	http.Redirect(w, r, path, http.StatusFound)
}
