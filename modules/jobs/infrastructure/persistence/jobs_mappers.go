// Package persistence provides this package.
package persistence

import (
	"database/sql"
	"encoding/json"

	"github.com/iota-uz/iota-sdk/modules/jobs/domain/aggregates/job"
	"github.com/iota-uz/iota-sdk/modules/jobs/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

func JobToDomain(m *models.Job) job.Job {
	opts := []job.Option{
		job.WithID(m.ID),
		job.WithTenantID(m.TenantID),
		job.WithUserID(m.UserID),
		job.WithParams(m.DecodeParams()),
		job.WithStatus(job.Status(m.Status)),
		job.WithProgress(m.Progress),
		job.WithAttempt(m.Attempt),
		job.WithCreatedAt(m.CreatedAt),
		job.WithUpdatedAt(m.UpdatedAt),
	}
	if m.Phase.Valid {
		opts = append(opts, job.WithPhase(m.Phase.String))
	}
	if m.Error.Valid {
		opts = append(opts, job.WithError(m.Error.String))
	}
	if m.ResultUploadID.Valid {
		id := uint(m.ResultUploadID.Int64)
		opts = append(opts, job.WithResultUploadID(&id))
	}
	if m.StartedAt.Valid {
		startedAt := m.StartedAt.Time
		opts = append(opts, job.WithStartedAt(&startedAt))
	}
	if m.FinishedAt.Valid {
		finishedAt := m.FinishedAt.Time
		opts = append(opts, job.WithFinishedAt(&finishedAt))
	}
	return job.New(m.Kind, opts...)
}

func JobToModel(j job.Job) *models.Job {
	params := j.Params()
	if params == nil {
		params = map[string]any{}
	}
	rawParams, err := json.Marshal(params)
	if err != nil {
		rawParams = []byte("{}")
	}
	var resultUploadID sql.NullInt64
	if id := j.ResultUploadID(); id != nil {
		resultUploadID = mapping.UintToSQLNullInt64(*id)
	}
	return &models.Job{
		ID:             j.ID(),
		TenantID:       j.TenantID(),
		UserID:         j.UserID(),
		Kind:           j.Kind(),
		Params:         rawParams,
		Status:         j.Status().String(),
		Progress:       j.Progress(),
		Phase:          mapping.ValueToSQLNullString(j.Phase()),
		Attempt:        j.Attempt(),
		ResultUploadID: resultUploadID,
		Error:          mapping.ValueToSQLNullString(j.Error()),
		CreatedAt:      j.CreatedAt(),
		UpdatedAt:      j.UpdatedAt(),
		StartedAt:      mapping.PointerToSQLNullTime(j.StartedAt()),
		FinishedAt:     mapping.PointerToSQLNullTime(j.FinishedAt()),
	}
}
