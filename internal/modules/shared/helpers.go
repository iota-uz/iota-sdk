package shared

import (
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	"net/http"
	"strconv"
)

func HxRedirect(w http.ResponseWriter, _ *http.Request, path string) {
	w.Header().Add("Hx-Redirect", path)
	w.WriteHeader(http.StatusOK)
}

func Redirect(w http.ResponseWriter, r *http.Request, path string) {
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	if isHxRequest {
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

// MapViewModels maps entities to view models
func MapViewModels[T any, V any](
	entities []T,
	mapFunc func(T) *V,
) []*V {
	viewModels := make([]*V, len(entities))
	for i, entity := range entities {
		viewModels[i] = mapFunc(entity)
	}
	return viewModels
}
