// Package controllers provides this package.
package controllers

import (
	"context"
	"log"
	"mime/multipart"
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
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/multifs"
)

type UploadController struct {
	uploadService *services.UploadService
	cfg           *uploadsconfig.Config
	basePath      string
}

func NewUploadController(uploadService *services.UploadService, cfg *uploadsconfig.Config) application.Controller {
	return &UploadController{
		uploadService: uploadService,
		cfg:           cfg,
		basePath:      "/uploads",
	}
}

func (c *UploadController) Key() string {
	return "/upload"
}

func (c *UploadController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(middleware.Authorize())
	router.HandleFunc("", c.Create).Methods(http.MethodPost)

	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fullPath := filepath.Join(workDir, c.cfg.Path)
	prefix := path.Join("/", c.cfg.Path, "/")
	neuteredFS := multifs.NewNeuteredFileSystem(http.Dir(fullPath))
	r.PathPrefix(prefix).Handler(http.StripPrefix(prefix, http.FileServer(neuteredFS)))
}

func (c *UploadController) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(c.cfg.MaxMemory); err != nil {
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
			File: file,
			Name: header.Filename,
			Size: int(header.Size),
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
