// Package models provides this package.
package models

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Job struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	UserID         uint
	Kind           string
	Params         []byte
	Status         string
	Progress       int
	Phase          sql.NullString
	Attempt        int
	ResultUploadID sql.NullInt64
	Error          sql.NullString
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StartedAt      sql.NullTime
	FinishedAt     sql.NullTime
}

// DecodeParams returns the stored params map, tolerating a NULL/empty column.
func (m *Job) DecodeParams() map[string]any {
	params := map[string]any{}
	if len(m.Params) > 0 {
		_ = json.Unmarshal(m.Params, &params)
	}
	return params
}
