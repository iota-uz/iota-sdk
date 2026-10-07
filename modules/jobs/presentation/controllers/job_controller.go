// Package controllers provides this package.
package controllers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	jobscomponents "github.com/iota-uz/iota-sdk/components/jobs"
	"github.com/iota-uz/iota-sdk/modules/jobs/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/jobs"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

type JobController struct {
	runner   *jobs.Runner
	basePath string
}

func NewJobController(runner *jobs.Runner) application.Controller {
	return &JobController{
		runner:   runner,
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
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/result", di.H(c.Result)).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/retry", di.H(c.Retry)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", di.H(c.Delete)).Methods(http.MethodDelete)
}

// Create enqueues a job. HTMX responses render the polling job item (the
// standard trigger button targets the operations stack with hx-swap
// beforeend); plain requests get the job id as JSON.
func (c *JobController) Create(
	r *http.Request,
	w http.ResponseWriter,
) {
	dto, err := dtos.ParseCreateJobDTO(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	created, err := c.runner.Enqueue(r.Context(), dto.Kind, dto.Params)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, jobs.ErrUnknownJobKind) {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}

	htmx.SetTrigger(w, "jobs:enqueued", created.ID.String())
	if !htmx.IsHxRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     created.ID.String(),
			"status": created.Status.String(),
		})
		return
	}
	templ.Handler(jobscomponents.Item(created), templ.WithStreaming()).ServeHTTP(w, r)
}

// Get returns a single job. HTMX responses render the self-polling job item;
// plain requests get JSON.
func (c *JobController) Get(
	r *http.Request,
	w http.ResponseWriter,
) {
	id, ok := jobID(r)
	if !ok {
		http.Error(w, "invalid job id", http.StatusBadRequest)
		return
	}
	found, err := c.runner.Get(r.Context(), id)
	if err != nil {
		writeJobError(w, err)
		return
	}
	if !htmx.IsHxRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jobJSON(found))
		return
	}
	templ.Handler(jobscomponents.Item(found), templ.WithStreaming()).ServeHTTP(w, r)
}

// Mine renders the signed-in user's recent jobs for the operations stack.
func (c *JobController) Mine(
	r *http.Request,
	w http.ResponseWriter,
) {
	list, err := c.runner.ListMine(r.Context(), jobs.DefaultListLimit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	templ.Handler(jobscomponents.List(list), templ.WithStreaming()).ServeHTTP(w, r)
}

// Result streams the stored result bytes as a download.
func (c *JobController) Result(
	r *http.Request,
	w http.ResponseWriter,
) {
	id, ok := jobID(r)
	if !ok {
		http.Error(w, "invalid job id", http.StatusBadRequest)
		return
	}
	name, data, err := c.runner.GetResult(r.Context(), id)
	if err != nil {
		writeJobError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename="+asciiFilename(name))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// Retry requeues a failed job and re-renders it.
func (c *JobController) Retry(
	r *http.Request,
	w http.ResponseWriter,
) {
	id, ok := jobID(r)
	if !ok {
		http.Error(w, "invalid job id", http.StatusBadRequest)
		return
	}
	requeued, err := c.runner.Retry(r.Context(), id)
	if err != nil {
		writeJobError(w, err)
		return
	}
	templ.Handler(jobscomponents.Item(requeued), templ.WithStreaming()).ServeHTTP(w, r)
}

// Delete dismisses a job from the operations stack.
func (c *JobController) Delete(
	r *http.Request,
	w http.ResponseWriter,
) {
	id, ok := jobID(r)
	if !ok {
		http.Error(w, "invalid job id", http.StatusBadRequest)
		return
	}
	if err := c.runner.Delete(r.Context(), id); err != nil {
		writeJobError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
