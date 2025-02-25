package shared

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
)

func HxRedirect(w http.ResponseWriter, _ *http.Request, path string) {
	w.Header().Add("Hx-Redirect", path)
	w.WriteHeader(http.StatusOK)
}

func IsHxRequest(r *http.Request) bool {
	return len(r.Header.Get("Hx-Request")) > 0
}

func IsHXBoosted(r *http.Request) bool {
	return len(r.Header.Get("Hx-Boosted")) > 0
}

func Redirect(w http.ResponseWriter, r *http.Request, path string) {
	if IsHxRequest(r) {
		HxRedirect(w, r, path)
		return
	}
	http.Redirect(w, r, path, http.StatusFound)
}

func ParseID(r *http.Request) (uint, error) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		return 0, errors.Wrap(err, "Error parsing id")
	}
	return uint(id), nil
}

func SetFlash(w http.ResponseWriter, name string, value []byte) {
	c := &http.Cookie{Name: name, Value: base64.URLEncoding.EncodeToString(value)}
	http.SetCookie(w, c)
}

func SetFlashMap[K comparable, V any](w http.ResponseWriter, name string, value map[K]V) {
	errors, err := json.Marshal(value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	SetFlash(w, name, errors)
}
