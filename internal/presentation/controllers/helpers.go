package controllers

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

func parseID(r *http.Request) (uint, error) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}
