package services_test

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/jobs/domain/aggregates/job"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

var _ upload.Upload = (*fakeUpload)(nil)

// fakeRepo is an in-memory job.Repository. Tenant scoping is enforced the
// same way the Postgres repository enforces it.
type fakeRepo struct {
	mu       sync.Mutex
	rows     []job.Job
	progress []progressWrite
	claimSeq int
}

type progressWrite struct {
	ID      uuid.UUID
	Percent int
	Phase   string
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{}
}

func (f *fakeRepo) Save(ctx context.Context, j job.Job) (job.Job, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}
	if j.TenantID() != tenantID {
		return nil, job.ErrNotFound
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if j.ID() == uuid.Nil {
		id := uuid.New()
		j.SetID(id)
		f.rows = append(f.rows, j)
		return j, nil
	}
	for i, row := range f.rows {
		if row.ID() == j.ID() {
			f.rows[i] = j
			return j, nil
		}
	}
	f.rows = append(f.rows, j)
	return j, nil
}

func (f *fakeRepo) get(ctx context.Context, id uuid.UUID) (job.Job, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, row := range f.rows {
		if row.ID() == id && row.TenantID() == tenantID {
			return row, nil
		}
	}
	return nil, job.ErrNotFound
}

func (f *fakeRepo) GetByID(ctx context.Context, id uuid.UUID) (job.Job, error) {
	return f.get(ctx, id)
}

func (f *fakeRepo) GetByIDForUser(ctx context.Context, id uuid.UUID, userID uint) (job.Job, error) {
	found, err := f.get(ctx, id)
	if err != nil {
		return nil, err
	}
	if found.UserID() != userID {
		return nil, job.ErrNotFound
	}
	return found, nil
}

func (f *fakeRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, row := range f.rows {
		if row.ID() == id && row.TenantID() == tenantID {
			f.rows = append(f.rows[:i], f.rows[i+1:]...)
			return nil
		}
	}
	return nil
}

func (f *fakeRepo) ListByUser(ctx context.Context, userID uint, limit int) ([]job.Job, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]job.Job, 0, limit)
	for _, row := range f.rows {
		if row.TenantID() == tenantID && row.UserID() == userID && len(out) < limit {
			out = append(out, row)
		}
	}
	return out, nil
}

func (f *fakeRepo) ClaimNext(ctx context.Context) (job.Job, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, row := range f.rows {
		if row.Status() == job.StatusQueued {
			claimed := row.MarkRunning()
			f.rows[i] = claimed
			f.claimSeq++
			return claimed, true, nil
		}
	}
	return nil, false, nil
}

func (f *fakeRepo) UpdateProgress(ctx context.Context, id uuid.UUID, percent int, phase string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.progress = append(f.progress, progressWrite{ID: id, Percent: percent, Phase: phase})
	return nil
}

func (f *fakeRepo) ListFinishedBefore(ctx context.Context, cutoff time.Time, limit int) ([]job.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]job.Job, 0)
	for _, row := range f.rows {
		fin := row.FinishedAt()
		if row.Status().IsTerminal() && fin != nil && fin.Before(cutoff) && len(out) < limit {
			out = append(out, row)
		}
	}
	return out, nil
}

func (f *fakeRepo) byID(id uuid.UUID) job.Job {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, row := range f.rows {
		if row.ID() == id {
			return row
		}
	}
	return nil
}

type storedCreate struct {
	Name string
	Data string
}

// fakeUploadStore records created result files.
type fakeUploadStore struct {
	mu         sync.Mutex
	nextID     uint
	created    []storedCreate
	deleted    []uint
	failCreate bool
}

func newFakeUploadStore() *fakeUploadStore {
	return &fakeUploadStore{nextID: 100}
}

func (f *fakeUploadStore) Create(ctx context.Context, data *upload.CreateDTO) (upload.Upload, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failCreate {
		return nil, errFakeUpload
	}
	raw, readErr := io.ReadAll(data.File)
	if readErr != nil {
		return nil, readErr
	}
	f.nextID++
	f.created = append(f.created, storedCreate{Name: data.Name, Data: string(raw)})
	return &fakeUpload{id: f.nextID, name: data.Name}, nil
}

func (f *fakeUploadStore) Delete(ctx context.Context, id uint) (upload.Upload, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, id)
	return &fakeUpload{id: id}, nil
}

type fakeUpload struct {
	upload.Upload
	id   uint
	name string
}

func (u *fakeUpload) ID() uint     { return u.id }
func (u *fakeUpload) Name() string { return u.name }

type fakeError string

func (e fakeError) Error() string { return string(e) }

const errFakeUpload = fakeError("fake upload failure")
