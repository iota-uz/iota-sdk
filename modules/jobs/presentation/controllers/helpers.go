package controllers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/jobs"
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
	case errors.Is(err, jobs.ErrNotFound):
		http.Error(w, "job not found", http.StatusNotFound)
	case errors.Is(err, jobs.ErrNotRetryable):
		http.Error(w, "job is not retryable", http.StatusConflict)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func jobJSON(j jobs.Job) map[string]any {
	payload := map[string]any{
		"id":       j.ID.String(),
		"kind":     j.Kind,
		"status":   j.Status.String(),
		"progress": j.Progress,
	}
	if j.Phase != "" {
		payload["phase"] = j.Phase
	}
	if j.Error != "" {
		payload["error"] = j.Error
	}
	if j.ResultURL != "" {
		payload["resultUrl"] = j.ResultURL
	} else if j.HasStoredResult() {
		payload["resultName"] = j.ResultName
		payload["resultUrl"] = "/jobs/" + j.ID.String() + "/result"
	}
	return payload
}

// asciiFilename makes the Content-Disposition filename header-safe.
func asciiFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r > 32 && r < 127 && r != '"' && r != '\\' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "download"
	}
	return b.String()
}
