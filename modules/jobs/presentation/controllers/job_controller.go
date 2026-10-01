// Package controllers provides this package.
package controllers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	jobscomponents "github.com/iota-uz/iota-sdk/components/jobs"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/jobs/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/jobs/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/jobs/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/jobs/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

type JobController struct {
	basePath string
}

func NewJobController() application.Controller {
	return &JobController{
		basePath: "/jobs",
	}
}

func (c *JobController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("jobs.job", 0, application.Route("", c.basePath))
}

func (c *JobController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("", di.H(c.Create)).Methods(http.MethodPost)
	router.HandleFunc("/mine", di.H(c.Mine)).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", di.H(c.Get)).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/retry", di.H(c.Retry)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", di.H(c.Delete)).Methods(http.MethodDelete)
}

// Create enqueues a job. HTMX responses render the polling job item (the
// standard trigger button targets the operations stack with hx-swap
// beforeend); plain requests get the job id as JSON.
func (c *JobController) Create(
	r *http.Request,
	w http.ResponseWriter,
	jobService *services.JobService,
) {
	dto, err := dtos.ParseCreateJobDTO(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	created, err := jobService.Enqueue(r.Context(), dto.Kind, dto.Params)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, services.ErrUnknownJobKind) {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}

	htmx.SetTrigger(w, "jobs:enqueued", created.ID().String())
	if !htmx.IsHxRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     created.ID().String(),
			"status": created.Status().String(),
		})
		return
	}
	renderItem(w, r, mappers.JobToViewModel(created, "", ""))
}

// Get returns a single job. HTMX responses render the self-polling job item;
// plain requests get JSON.
func (c *JobController) Get(
	r *http.Request,
	w http.ResponseWriter,
	jobService *services.JobService,
	uploadService *coreservices.UploadService,
) {
	id, ok := jobID(r)
	if !ok {
		http.Error(w, "invalid job id", http.StatusBadRequest)
		return
	}
	found, err := jobService.Get(r.Context(), id)
	if err != nil {
		writeJobError(w, err)
		return
	}

	if !htmx.IsHxRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		name, url := uploadResult(r, uploadService, found)
		_ = json.NewEncoder(w).Encode(jobJSON(found, name, url))
		return
	}
	name, url := uploadResult(r, uploadService, found)
	renderItem(w, r, mappers.JobToViewModel(found, name, url))
}

// Mine renders the signed-in user's recent jobs for the operations stack.
func (c *JobController) Mine(
	r *http.Request,
	w http.ResponseWriter,
	jobService *services.JobService,
	uploadService *coreservices.UploadService,
) {
	jobs, err := jobService.ListMine(r.Context(), services.DefaultMyJobsLimit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	vms := make([]viewmodels.JobViewModel, 0, len(jobs))
	for _, j := range jobs {
		name, url := uploadResult(r, uploadService, j)
		vms = append(vms, mappers.JobToViewModel(j, name, url))
	}
	templ.Handler(jobscomponents.List(vms), templ.WithStreaming()).ServeHTTP(w, r)
}

// Retry requeues a failed job and re-renders it.
func (c *JobController) Retry(
	r *http.Request,
	w http.ResponseWriter,
	jobService *services.JobService,
) {
	id, ok := jobID(r)
	if !ok {
		http.Error(w, "invalid job id", http.StatusBadRequest)
		return
	}
	requeued, err := jobService.Retry(r.Context(), id)
	if err != nil {
		writeJobError(w, err)
		return
	}
	renderItem(w, r, mappers.JobToViewModel(requeued, "", ""))
}

// Delete dismisses a job from the operations stack.
func (c *JobController) Delete(
	r *http.Request,
	w http.ResponseWriter,
	jobService *services.JobService,
) {
	id, ok := jobID(r)
	if !ok {
		http.Error(w, "invalid job id", http.StatusBadRequest)
		return
	}
	if err := jobService.Delete(r.Context(), id); err != nil {
		writeJobError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
