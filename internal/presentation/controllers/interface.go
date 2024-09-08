package controllers

import "github.com/gorilla/mux"

type Controller interface {
	Register(r *mux.Router)
}
