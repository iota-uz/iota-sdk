package controllers

import (
	"net/http"

	"github.com/benbjohnson/hashfs"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/presentation/assets"
)

type StaticFilesController struct{}

func (s *StaticFilesController) Register(r *mux.Router) {
	r.PathPrefix("/static").Handler(http.StripPrefix("/static", http.FileServer(http.Dir("internal/presentation/static"))))
	r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", hashfs.FileServer(assets.FS)))
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("internal/presentation/public")))
}

func NewStaticFilesController() Controller {
	return &StaticFilesController{}
}
