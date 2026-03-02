package persistence

import "time"

type RunRecord struct {
	ID                string
	Operation         string
	Mode              string
	StartedAt         time.Time
	FinishedAt        *time.Time
	Actor             string
	Status            string
	TargetFingerprint string
	PolicyHash        string
	Error             *string
}

type StepRecord struct {
	RunID      string
	StepID     string
	Status     string
	StartedAt  time.Time
	FinishedAt *time.Time
	Error      *string
}

type ArtifactRecord struct {
	RunID        string
	ArtifactType string
	PayloadJSON  string
	CreatedAt    time.Time
}
