package controllers

import (
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/upload"
	"github.com/iota-agency/iota-sdk/pkg/presentation/mappers"
	"github.com/iota-agency/iota-sdk/pkg/presentation/templates/components"
	"github.com/iota-agency/iota-sdk/pkg/services"
	"github.com/iota-agency/iota-sdk/pkg/types"
)

type UploadController struct {
	app           application.Application
	uploadService *services.UploadService
	basePath      string
}

func NewUploadController(app application.Application) application.Controller {
	return &UploadController{
		app:           app,
		uploadService: app.Service(services.UploadService{}).(*services.UploadService),
		basePath:      "/uploads",
	}
}

func (c *UploadController) Register(r *mux.Router) {
	conf := configuration.Use()
	router := r.PathPrefix(c.basePath).Subrouter()
	router.HandleFunc("", c.Create).Methods(http.MethodPost)

	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fullPath := filepath.Join(workDir, conf.UploadsPath)
	prefix := path.Join("/", conf.UploadsPath, "/")
	r.PathPrefix(prefix).Handler(http.StripPrefix(prefix, http.FileServer(http.Dir(fullPath))))
}

func (c *UploadController) Create(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto := &upload.CreateDTO{
		File: file,
		Name: header.Filename,
		Size: int(header.Size),
	}

	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("WarehouseUnits.New.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	id := r.FormValue("_id")
	name := r.FormValue("_name")
	formName := r.FormValue("_formName")
	if errorsMap, ok := dto.Ok(pageCtx.UniTranslator); !ok {
		_, _, err := dto.ToEntity()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		templ.Handler(components.UploadPreview(&components.UploadInputProps{ID: id, Upload: nil, Errors: errorsMap, Form: formName, Name: name}), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	upload, err := c.uploadService.Create(r.Context(), dto)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	templ.Handler(components.UploadPreview(&components.UploadInputProps{ID: id, Upload: mappers.UploadToViewModel(upload), Form: formName, Name: name}), templ.WithStreaming()).ServeHTTP(w, r)
}
