package controllers

import (
	"github.com/gorilla/mux"
	"net/http"
	"strconv"
)

func parseId(r *http.Request) (uint, error) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}
