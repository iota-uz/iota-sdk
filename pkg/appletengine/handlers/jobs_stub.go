package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/applets"
	appletenginejobs "github.com/iota-uz/iota-sdk/pkg/appletengine/jobs"
	appletenginerpc "github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
)

type JobsStore interface {
	Enqueue(ctx context.Context, method string, params any) (map[string]any, error)
	Schedule(ctx context.Context, cronExpr string, method string, params any) (map[string]any, error)
	List(ctx context.Context) ([]map[string]any, error)
	Cancel(ctx context.Context, jobID string) (bool, error)
}

type jobRecord struct {
	ID         string
	Type       string
	Cron       string
	Method     string
	Params     any
	Status     string
	NextRunAt  *time.Time
	LastRunAt  *time.Time
	LastError  string
	LastStatus string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type memoryJobsStore struct {
	mu   sync.RWMutex
	jobs map[string]map[string]jobRecord
}

type JobsStub struct {
	store JobsStore
}

func NewJobsStub() *JobsStub {
	return &JobsStub{store: newMemoryJobsStore()}
}

func NewJobsStubWithStore(store JobsStore) *JobsStub {
	if store == nil {
		store = newMemoryJobsStore()
	}
	return &JobsStub{store: store}
}

func newMemoryJobsStore() *memoryJobsStore {
	return &memoryJobsStore{jobs: make(map[string]map[string]jobRecord)}
}

func (s *JobsStub) Register(registry *appletenginerpc.Registry, appletName string) error {
	methods := map[string]applets.Procedure[any, any]{
		"enqueue": {
			Handler: func(ctx context.Context, params any) (any, error) {
				payload, ok := params.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid params: %w", applets.ErrInvalid)
				}
				method, _ := payload["method"].(string)
				if method == "" {
					return nil, fmt.Errorf("method is required: %w", applets.ErrInvalid)
				}
				job, err := s.store.Enqueue(ctx, method, payload["params"])
				if err != nil {
					return nil, err
				}
				return job, nil
			},
		},
		"schedule": {
			Handler: func(ctx context.Context, params any) (any, error) {
				payload, ok := params.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid params: %w", applets.ErrInvalid)
				}
				cronExpr, _ := payload["cron"].(string)
				if cronExpr == "" {
					return nil, fmt.Errorf("cron is required: %w", applets.ErrInvalid)
				}
				method, _ := payload["method"].(string)
				if method == "" {
					return nil, fmt.Errorf("method is required: %w", applets.ErrInvalid)
				}
				job, err := s.store.Schedule(ctx, cronExpr, method, payload["params"])
				if err != nil {
					return nil, err
				}
				return job, nil
			},
		},
		"list": {
			Handler: func(ctx context.Context, _ any) (any, error) {
				jobs, err := s.store.List(ctx)
				if err != nil {
					return nil, err
				}
				return jobs, nil
			},
		},
		"cancel": {
			Handler: func(ctx context.Context, params any) (any, error) {
				payload, ok := params.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid params: %w", applets.ErrInvalid)
				}
				jobID, _ := payload["id"].(string)
				if jobID == "" {
					return nil, fmt.Errorf("id is required: %w", applets.ErrInvalid)
				}
				canceled, err := s.store.Cancel(ctx, jobID)
				if err != nil {
					return nil, err
				}
				return map[string]any{"ok": canceled}, nil
			},
		},
	}

	for op, procedure := range methods {
		router := applets.NewTypedRPCRouter()
		if err := applets.AddProcedure(router, fmt.Sprintf("%s.jobs.%s", appletName, op), procedure); err != nil {
			return err
		}
		for methodName, method := range router.Config().Methods {
			if err := registry.RegisterServerOnly(appletName, methodName, method, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *memoryJobsStore) Enqueue(ctx context.Context, method string, params any) (map[string]any, error) {
	record := s.newRecord("one_off", "", method, params, "queued")
	s.save(ctx, record)
	return toJobResponse(record), nil
}

func (s *memoryJobsStore) Schedule(ctx context.Context, cronExpr string, method string, params any) (map[string]any, error) {
	nextRunAt, err := appletenginejobs.NextRun(cronExpr, time.Now().UTC())
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", applets.ErrInvalid)
	}
	record := s.newRecord("scheduled", cronExpr, method, params, "scheduled")
	record.NextRunAt = &nextRunAt
	s.save(ctx, record)
	return toJobResponse(record), nil
}

func (s *memoryJobsStore) List(ctx context.Context) ([]map[string]any, error) {
	scope := scopeFromContext(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()
	scopeJobs := s.jobs[scope]
	if scopeJobs == nil {
		return []map[string]any{}, nil
	}
	out := make([]map[string]any, 0, len(scopeJobs))
	for _, record := range scopeJobs {
		out = append(out, toJobResponse(record))
	}
	return out, nil
}

func (s *memoryJobsStore) Cancel(ctx context.Context, jobID string) (bool, error) {
	scope := scopeFromContext(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	scopeJobs := s.jobs[scope]
	if scopeJobs == nil {
		return false, nil
	}
	record, ok := scopeJobs[jobID]
	if !ok {
		return false, nil
	}
	record.Status = "canceled"
	record.LastStatus = "canceled"
	record.LastError = ""
	record.NextRunAt = nil
	record.UpdatedAt = time.Now().UTC()
	scopeJobs[jobID] = record
	return true, nil
}

func (s *memoryJobsStore) newRecord(jobType, cronExpr, method string, params any, status string) jobRecord {
	now := time.Now().UTC()
	return jobRecord{
		ID:         uuid.NewString(),
		Type:       jobType,
		Cron:       cronExpr,
		Method:     method,
		Params:     params,
		Status:     status,
		LastStatus: status,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

func (s *memoryJobsStore) save(ctx context.Context, record jobRecord) {
	scope := scopeFromContext(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	scopeJobs := s.jobs[scope]
	if scopeJobs == nil {
		scopeJobs = make(map[string]jobRecord)
		s.jobs[scope] = scopeJobs
	}
	scopeJobs[record.ID] = record
}

func toJobResponse(record jobRecord) map[string]any {
	params := record.Params
	if raw, err := json.Marshal(params); err == nil {
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err == nil {
			params = decoded
		}
	}
	result := map[string]any{
		"id":         record.ID,
		"type":       record.Type,
		"cron":       record.Cron,
		"method":     record.Method,
		"params":     params,
		"status":     record.Status,
		"lastStatus": record.LastStatus,
		"lastError":  record.LastError,
		"createdAt":  record.CreatedAt.Format(time.RFC3339Nano),
		"updatedAt":  record.UpdatedAt.Format(time.RFC3339Nano),
	}
	if record.NextRunAt != nil {
		result["nextRunAt"] = record.NextRunAt.UTC().Format(time.RFC3339Nano)
	}
	if record.LastRunAt != nil {
		result["lastRunAt"] = record.LastRunAt.UTC().Format(time.RFC3339Nano)
	}
	return result
}
