package controllers

import (
	"errors"
	"net/http"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	jobscomponents "github.com/iota-uz/iota-sdk/components/jobs"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/jobs/domain/aggregates/job"
	"github.com/iota-uz/iota-sdk/modules/jobs/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/jobs/services"
)

func jobID(r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func writeJobError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, job.ErrNotFound):
		http.Error(w, "job not found", http.StatusNotFound)
	case errors.Is(err, services.ErrJobNotRetryable):
		http.Error(w, "job is not retryable", http.StatusConflict)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderItem(w http.ResponseWriter, r *http.Request, vm viewmodels.JobViewModel) {
	templ.Handler(jobscomponents.Item(vm), templ.WithStreaming()).ServeHTTP(w, r)
}

func jobJSON(j job.Job, resultName, resultURL string) map[string]any {
	payload := map[string]any{
		"id":       j.ID().String(),
		"kind":     j.Kind(),
		"status":   j.Status().String(),
		"progress": j.Progress(),
	}
	if j.Phase() != "" {
		payload["phase"] = j.Phase()
	}
	if j.Error() != "" {
		payload["error"] = j.Error()
	}
	if resultURL != "" {
		payload["resultName"] = resultName
		payload["resultUrl"] = resultURL
	}
	return payload
}

// uploadResult resolves the download link for a job's stored result file.
// It returns empty strings when the job has no result. The link is relative
// (rooted at the upload's storage path) so it survives proxy/domain changes.
func uploadResult(r *http.Request, uploadService *coreservices.UploadService, j job.Job) (string, string) {
	uploadID := j.ResultUploadID()
	if uploadID == nil || j.Status() != job.StatusDone {
		return "", ""
	}
	ctx := r.Context()
	exists, err := uploadService.Exists(ctx, *uploadID)
	if err != nil || !exists {
		return "", ""
	}
	found, err := uploadService.GetByID(ctx, *uploadID)
	if err != nil {
		return "", ""
	}
	return found.Name(), "/" + found.Path()
}
