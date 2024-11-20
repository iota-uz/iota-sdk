package controllers

import (
	"github.com/benbjohnson/hashfs"
	"github.com/iota-agency/iota-sdk/pkg/multifs"
	"github.com/iota-agency/iota-sdk/pkg/shared"
	"net/http"

	"github.com/gorilla/mux"
)

type StaticFilesController struct {
	fsInstances []*hashfs.FS
}

func (s *StaticFilesController) Register(r *mux.Router) {
	handler := http.StripPrefix("/assets/", http.FileServer(multifs.New(s.fsInstances...)))
	r.PathPrefix("/assets/").Handler(handler)
}

func NewStaticFilesController(fsInstances []*hashfs.FS) shared.Controller {
	return &StaticFilesController{
		fsInstances: fsInstances,
	}
}
