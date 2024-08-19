package controllers

import "net/http"

func hxRedirect(w http.ResponseWriter, _ *http.Request, path string) {
	w.Header().Add("HX-Redirect", path)
	w.WriteHeader(http.StatusOK)
}
