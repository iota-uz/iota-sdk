package job_test

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/jobs/domain/aggregates/job"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runningJob(t *testing.T) job.Job {
	t.Helper()
	started := time.Now()
	j := job.New(
		"export",
		job.WithProgress(10),
		job.WithStartedAt(&started),
	)
	j = j.MarkRunning()
	require.Equal(t, job.StatusRunning, j.Status())
	return j
}

func TestJob_MarkRunning_FromQueued(t *testing.T) {
	j := job.New("export")

	after := j.MarkRunning()

	assert.Equal(t, job.StatusRunning, after.Status())
	assert.NotNil(t, after.StartedAt())
}

func TestJob_MarkRunning_FromTerminal_IsInert(t *testing.T) {
	j := job.New("export", job.WithStatus(job.StatusDone))

	after := j.MarkRunning()

	assert.Equal(t, job.StatusDone, after.Status())
}

func TestJob_MarkDone_SetsResultAndProgress(t *testing.T) {
	j := runningJob(t)
	uploadID := uint(42)

	after := j.MarkDone(&uploadID)

	assert.Equal(t, job.StatusDone, after.Status())
	require.NotNil(t, after.ResultUploadID())
	assert.Equal(t, uint(42), *after.ResultUploadID())
	assert.Equal(t, 100, after.Progress())
	assert.NotNil(t, after.FinishedAt())
}

func TestJob_MarkFailed_KeepsErrorForUI(t *testing.T) {
	j := runningJob(t)

	after := j.MarkFailed("boom")

	assert.Equal(t, job.StatusFailed, after.Status())
	assert.Equal(t, "boom", after.Error())
	assert.NotNil(t, after.FinishedAt())
	assert.True(t, after.Status().IsTerminal())
}

func TestJob_Requeue_FromFailedResetsState(t *testing.T) {
	j := runningJob(t).MarkFailed("boom")

	after := j.Requeue()

	assert.Equal(t, job.StatusQueued, after.Status())
	assert.Equal(t, "", after.Error())
	assert.Equal(t, 0, after.Progress())
	assert.Nil(t, after.FinishedAt())
	assert.Nil(t, after.ResultUploadID())
}

func TestJob_Requeue_FromNonFailed_IsInert(t *testing.T) {
	j := job.New("export")

	after := j.Requeue()

	assert.Equal(t, job.StatusQueued, after.Status())
}

func TestJob_UpdateProgress_ClampsAndKeepsPhase(t *testing.T) {
	j := runningJob(t)

	after := j.UpdateProgress(250, "writing rows")

	assert.Equal(t, 100, after.Progress())
	assert.Equal(t, "writing rows", after.Phase())
}

func TestJob_UpdateProgress_WhenTerminal_IsInert(t *testing.T) {
	uploadID := uint(1)
	j := runningJob(t).MarkDone(&uploadID)

	after := j.UpdateProgress(50, "late")

	assert.Equal(t, 100, after.Progress())
	assert.Equal(t, "", after.Phase())
}
