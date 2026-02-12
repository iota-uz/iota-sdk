package controllers

import (
	"context"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"

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
	"github.com/iota-uz/iota-sdk/pkg/multifs"
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
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(middleware.Authorize())
	router.Use(middleware.ProvideLocalizer(c.app))
	router.HandleFunc("", c.Create).Methods(http.MethodPost)

	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fullPath := filepath.Join(workDir, conf.UploadsPath)
	prefix := path.Join("/", conf.UploadsPath, "/")
	neuteredFS := multifs.NewNeuteredFileSystem(http.Dir(fullPath))
	r.PathPrefix(prefix).Handler(http.StripPrefix(prefix, http.FileServer(neuteredFS)))
}

func (c *UploadController) Create(w http.ResponseWriter, r *http.Request) {
	// Create context with 5-minute timeout for uploads
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	r = r.WithContext(ctx)

	conf := configuration.Use()
	if err := r.ParseMultipartForm(conf.MaxUploadMemory); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	files, ok := r.MultipartForm.File["file"]
	if !ok {
		http.Error(w, "No file(s) found", http.StatusBadRequest)
		return
	}

	id := r.FormValue("_id")
	name := r.FormValue("_name")
	formName := r.FormValue("_formName")
	multiple := r.FormValue("_multiple") == "true"

	// Get source from form or context
	source := r.FormValue("_source")
	if source == "" {
		source = composables.UseUploadSource(r.Context())
	}

	// Check upload permission for this source
	if err := composables.CheckUploadToSource(r.Context(), source, r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	dtos := make([]*upload.CreateDTO, 0, len(files))
	for _, header := range files {
		file, err := header.Open()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer func(file multipart.File) {
			if err := file.Close(); err != nil {
				log.Println(err)
			}
		}(file)

		dto := &upload.CreateDTO{
			File:   file,
			Name:   header.Filename,
			Size:   int(header.Size),
			Source: source,
		}

		// TODO: proper error handling
		if _, ok := dto.Ok(r.Context()); !ok {
			_, _, err := dto.ToEntity()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			props := &components.UploadInputProps{
				ID:       id,
				Uploads:  nil,
				Error:    "",
				Name:     name,
				Form:     formName,
				Multiple: multiple,
			}
			templ.Handler(components.UploadTarget(props), templ.WithStreaming()).ServeHTTP(w, r)
			return
		}
		dtos = append(dtos, dto)
	}

	var uploadEntities []upload.Upload
	err := composables.InTx(r.Context(), func(txCtx context.Context) error {
		var err error
		uploadEntities, err = c.uploadService.CreateMany(txCtx, dtos)
		return err
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	props := &components.UploadInputProps{
		ID:       id,
		Uploads:  mapping.MapViewModels(uploadEntities, mappers.UploadToViewModel),
		Name:     name,
		Form:     formName,
		Multiple: multiple,
	}

	templ.Handler(components.UploadTarget(props), templ.WithStreaming()).ServeHTTP(w, r)
}
