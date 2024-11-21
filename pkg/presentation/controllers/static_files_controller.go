package controllers

import (
	"embed"
	"github.com/benbjohnson/hashfs"
	"github.com/iota-agency/iota-sdk/pkg/multifs"
	"github.com/iota-agency/iota-sdk/pkg/shared"
	"net/http"

	"github.com/gorilla/mux"
)

type StaticFilesController struct {
	fsInstances []*embed.FS
}

func (s *StaticFilesController) Register(r *mux.Router) {
	fsInstances := make([]*hashfs.FS, len(s.fsInstances))
	for i, fs := range s.fsInstances {
		fsInstances[i] = hashfs.NewFS(fs)
	}
	handler := http.StripPrefix("/assets/", http.FileServer(multifs.New(fsInstances...)))
	r.PathPrefix("/assets/").Handler(handler)
}

func NewStaticFilesController(fsInstances []*embed.FS) shared.Controller {
	return &StaticFilesController{
		fsInstances: fsInstances,
	}
}
