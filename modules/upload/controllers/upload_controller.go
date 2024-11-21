package controllers

import (
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/modules/upload/services"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/shared"
)

type UploadController struct {
	app           *application.Application
	uploadService *services.UploadService
	basePath      string
}

func NewUploadController(app *application.Application) shared.Controller {
	return &UploadController{
		app:           app,
		uploadService: app.Service(services.UploadService{}).(*services.UploadService),
		basePath:      "/uploads",
	}
}

func (c *UploadController) Register(r *mux.Router) {
	// router := r.PathPrefix(c.basePath).Subrouter()
	// router.Use(middleware.RequireAuthorization())
	// router.HandleFunc("", c.List).Methods(http.MethodGet)
	// router.HandleFunc("", c.Create).Methods(http.MethodPost)
	// router.HandleFunc("/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)
	// router.HandleFunc("/{id:[0-9]+}", c.PostEdit).Methods(http.MethodPost)
	// router.HandleFunc("/{id:[0-9]+}", c.Delete).Methods(http.MethodDelete)
	// router.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)
}
