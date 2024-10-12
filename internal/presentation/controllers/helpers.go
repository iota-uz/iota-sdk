package controllers

import (
	"github.com/go-faster/errors"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

func parseID(r *http.Request) (uint, error) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		return 0, errors.Wrap(err, "Error parsing id")
	}
	return uint(id), nil
}
