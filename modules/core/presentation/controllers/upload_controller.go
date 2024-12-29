package controllers

import (
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/components"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
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

func (c *UploadController) Key() string {
	return "/upload"
}

func (c *UploadController) Register(r *mux.Router) {
	conf := configuration.Use()
	// TODO: middleware
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(middleware.Authorize())
	router.Use(middleware.WithTransaction())
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
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	files, ok := r.MultipartForm.File["file"]
	if !ok {
		http.Error(w, "No file(s) found", http.StatusBadRequest)
		return
	}

	uniTranslator, err := composables.UseUniLocalizer(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	id := r.FormValue("_id")
	name := r.FormValue("_name")
	formName := r.FormValue("_formName")

	dtos := make([]*upload.CreateDTO, 0, len(files))
	for _, header := range files {
		file, err := header.Open()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		dto := &upload.CreateDTO{
			File: file,
			Name: header.Filename,
			Size: int(header.Size),
		}

		// TODO: proper error handling
		if _, ok := dto.Ok(uniTranslator); !ok {
			_, _, err := dto.ToEntity()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			props := &components.UploadInputProps{
				ID:      id,
				Uploads: nil,
				Error:   "",
				Form:    formName,
				Name:    name,
			}
			templ.Handler(components.UploadPreview(props), templ.WithStreaming()).ServeHTTP(w, r)
			return
		}
		dtos = append(dtos, dto)
	}

	uploadEntities, err := c.uploadService.CreateMany(r.Context(), dtos)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	props := &components.UploadInputProps{
		ID:      id,
		Uploads: mapping.MapViewModels(uploadEntities, mappers.UploadToViewModel),
		Form:    formName,
		Name:    name,
	}
	templ.Handler(components.UploadPreview(props), templ.WithStreaming()).ServeHTTP(w, r)
}
