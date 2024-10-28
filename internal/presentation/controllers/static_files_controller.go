package controllers

import (
	"github.com/iota-agency/iota-erp/internal/modules/shared"
	"net/http"

	"github.com/benbjohnson/hashfs"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/presentation/assets"
)

type StaticFilesController struct{}

func (s *StaticFilesController) Register(r *mux.Router) {
	r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", hashfs.FileServer(assets.FS)))
}

func NewStaticFilesController() shared.Controller {
	return &StaticFilesController{}
}
