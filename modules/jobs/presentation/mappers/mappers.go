// Package mappers provides this package.
package mappers

import (
	"github.com/iota-uz/iota-sdk/modules/jobs/domain/aggregates/job"
	"github.com/iota-uz/iota-sdk/modules/jobs/presentation/viewmodels"
)

// JobToViewModel maps a domain job. resultName/resultURL describe the stored
// result file and are empty when the job has none.
func JobToViewModel(j job.Job, resultName, resultURL string) viewmodels.JobViewModel {
	return viewmodels.JobViewModel{
		ID:         j.ID(),
		Kind:       j.Kind(),
		Status:     j.Status().String(),
		Progress:   j.Progress(),
		Phase:      j.Phase(),
		Error:      j.Error(),
		ResultName: resultName,
		ResultURL:  resultURL,
		CreatedAt:  j.CreatedAt(),
		FinishedAt: j.FinishedAt(),
	}
}
