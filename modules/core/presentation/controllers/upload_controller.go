package controllers

import (
	"context"
	"fmt"
	"io"
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
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(middleware.Authorize())
	router.Use(middleware.ProvideLocalizer(c.app.Bundle()))
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

	// Create a component that includes both the preview and form update
	component := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		// Render the preview
		if err := components.UploadPreview(props).Render(ctx, w); err != nil {
			return err
		}

		// If we have uploads and field name, render an out-of-band update for the form field
		if len(props.Uploads) > 0 && props.Name != "" {
			upload := props.Uploads[0]
			oobHTML := fmt.Sprintf(`<div id="field-%s" hx-swap-oob="true"><input type="hidden" name="%s" value="%s"/></div>`,
				props.Name, props.Name, upload.ID)
			if _, err := w.Write([]byte(oobHTML)); err != nil {
				return err
			}
		}

		return nil
	})

	templ.Handler(component, templ.WithStreaming()).ServeHTTP(w, r)
}
