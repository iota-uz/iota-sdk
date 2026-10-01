// Package viewmodels provides this package.
package viewmodels

import (
	"time"

	"github.com/google/uuid"
)

// JobViewModel is the presentation snapshot of a job, rendered by the
// operations widget components. ResultURL is empty unless the job finished
// with a stored result file.
type JobViewModel struct {
	ID         uuid.UUID
	Kind       string
	Status     string
	Progress   int
	Phase      string
	Error      string
	ResultName string
	ResultURL  string
	CreatedAt  time.Time
	FinishedAt *time.Time
}
