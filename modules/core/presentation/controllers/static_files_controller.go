package controllers

import (
	"github.com/benbjohnson/hashfs"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/multifs"
	"net/http"
)

type StaticFilesController struct {
	fsInstances []*hashfs.FS
}

func (s *StaticFilesController) Key() string {
	return "/assets"
}

func (s *StaticFilesController) Register(r *mux.Router) {
	fsHandler := http.StripPrefix("/assets/", http.FileServer(multifs.New(s.fsInstances...)))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		fsHandler.ServeHTTP(w, r)
	})
	r.PathPrefix("/assets/").Handler(handler)
}

func NewStaticFilesController(fsInstances []*hashfs.FS) application.Controller {
	return &StaticFilesController{
		fsInstances: fsInstances,
	}
}
