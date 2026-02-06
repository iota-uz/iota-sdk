package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/interfaces/graph/authorizers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

type UploadAPIController struct {
	app             application.Application
	authorizer      types.UploadsAuthorizer
	defaultTenantID uuid.UUID
}

type UploadAPIControllerOption func(*UploadAPIController)

// WithAPIUploadsAuthorizer sets a custom uploads authorizer for the API controller
func WithAPIUploadsAuthorizer(auth types.UploadsAuthorizer) UploadAPIControllerOption {
	return func(c *UploadAPIController) {
		c.authorizer = auth
	}
}

// WithDefaultTenantID sets a fallback tenant ID for unauthenticated requests.
// This is required in single-tenant deployments where public uploads need a tenant context.
func WithDefaultTenantID(id uuid.UUID) UploadAPIControllerOption {
	return func(c *UploadAPIController) {
		c.defaultTenantID = id
	}
}

func NewUploadAPIController(app application.Application, opts ...UploadAPIControllerOption) application.Controller {
	c := &UploadAPIController{
		app:        app,
		authorizer: authorizers.NewDefaultUploadsAuthorizer(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *UploadAPIController) Key() string {
	return "/api/uploads"
}

func (c *UploadAPIController) Register(r *mux.Router) {
	router := r.PathPrefix(c.Key()).Subrouter()
	router.Use(middleware.Authorize())
	router.Use(middleware.ProvideUser())
	if c.defaultTenantID != uuid.Nil {
		router.Use(c.ensureTenantID())
	}
	router.Use(middleware.ProvideLocalizer(c.app))
	router.Use(middleware.WithTransaction())
	router.HandleFunc("", c.Create).Methods(http.MethodPost)
}

// ensureTenantID injects the default tenant ID when no tenant is set by auth middleware.
func (c *UploadAPIController) ensureTenantID() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, err := composables.UseTenantID(r.Context()); err != nil {
				r = r.WithContext(composables.WithTenantID(r.Context(), c.defaultTenantID))
			}
			next.ServeHTTP(w, r)
		})
	}
}

type UploadAPIResponse struct {
	ID       uint              `json:"id"`
	URL      string            `json:"url"`
	Hash     string            `json:"hash"`
	Path     string            `json:"path"`
	Name     string            `json:"name"`
	Slug     string            `json:"slug"`
	Mimetype string            `json:"mimetype"`
	Type     string            `json:"type"`
	Size     int               `json:"size"`
	GeoPoint *GeoPointResponse `json:"geoPoint,omitempty"`
}

type GeoPointResponse struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

func (c *UploadAPIController) Create(w http.ResponseWriter, r *http.Request) {
	conf := configuration.Use()
	if err := r.ParseMultipartForm(conf.MaxUploadMemory); err != nil {
		c.writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	files, ok := r.MultipartForm.File["file"]
	if !ok || len(files) == 0 {
		c.writeJSONError(w, http.StatusBadRequest, "No file found")
		return
	}

	// Get optional fields
	slug := r.FormValue("slug")
	latStr := r.FormValue("lat")
	lngStr := r.FormValue("lng")

	// Check authorization
	if slug != "" {
		if err := c.authorizer.CanUploadFileWithSlug(r.Context()); err != nil {
			c.writeJSONError(w, http.StatusForbidden, "Unauthorized to upload file with custom slug")
			return
		}
	} else {
		if err := c.authorizer.CanUploadFile(r.Context()); err != nil {
			c.writeJSONError(w, http.StatusForbidden, "Unauthorized to upload file")
			return
		}
	}

	// Process the first file
	header := files[0]
	file, err := header.Open()
	if err != nil {
		c.writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer func() {
		_ = file.Close()
	}()

	// Build DTO
	dto := &upload.CreateDTO{
		File: file,
		Name: header.Filename,
		Size: int(header.Size),
		Slug: slug,
	}

	// Parse geo coordinates if provided
	if latStr != "" && lngStr != "" {
		lat, err := strconv.ParseFloat(latStr, 64)
		if err != nil {
			c.writeJSONError(w, http.StatusBadRequest, "Invalid latitude value")
			return
		}
		lng, err := strconv.ParseFloat(lngStr, 64)
		if err != nil {
			c.writeJSONError(w, http.StatusBadRequest, "Invalid longitude value")
			return
		}
		dto.GeoPoint = &upload.GeoPoint{
			Lat: lat,
			Lng: lng,
		}
	}

	// Validate DTO
	if _, ok := dto.Ok(r.Context()); !ok {
		c.writeJSONError(w, http.StatusBadRequest, "Validation failed")
		return
	}

	// Create upload
	uploadService := c.app.Service(services.UploadService{}).(*services.UploadService)
	uploadEntity, err := uploadService.Create(r.Context(), dto)
	if err != nil {
		c.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Build response
	response := UploadAPIResponse{
		ID:   uploadEntity.ID(),
		URL:  uploadEntity.URL().String(),
		Hash: uploadEntity.Hash(),
		Path: uploadEntity.Path(),
		Name: uploadEntity.Name(),
		Slug: uploadEntity.Slug(),
		Size: uploadEntity.Size().Bytes(),
		Type: uploadEntity.Type().String(),
	}

	if uploadEntity.Mimetype() != nil {
		response.Mimetype = uploadEntity.Mimetype().String()
	}

	if gp := uploadEntity.GeoPoint(); gp != nil {
		response.GeoPoint = &GeoPointResponse{
			Lat: gp.Lat(),
			Lng: gp.Lng(),
		}
	}

	c.writeJSON(w, response)
}

func (c *UploadAPIController) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (c *UploadAPIController) writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
