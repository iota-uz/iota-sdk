package handlers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/iota-uz/applets"
	appletenginerpc "github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
)

type DBStore interface {
	Get(ctx context.Context, id string) (any, error)
	Query(ctx context.Context, table string) ([]any, error)
	Insert(ctx context.Context, table string, value any) (any, error)
	Patch(ctx context.Context, id string, value any) (any, error)
	Replace(ctx context.Context, id string, value any) (any, error)
	Delete(ctx context.Context, id string) (bool, error)
}

type documentRecord struct {
	ID    string
	Table string
	Value any
}

type memoryDBStore struct {
	mu       sync.RWMutex
	records  map[string]map[string]documentRecord
	sequence int64
}

type DBStub struct {
	store DBStore
}

func NewDBStub() *DBStub {
	return &DBStub{store: newMemoryDBStore()}
}

func NewDBStubWithStore(store DBStore) *DBStub {
	if store == nil {
		store = newMemoryDBStore()
	}
	return &DBStub{store: store}
}

func newMemoryDBStore() *memoryDBStore {
	return &memoryDBStore{
		records: make(map[string]map[string]documentRecord),
	}
}

func (s *DBStub) Register(registry *appletenginerpc.Registry, appletName string) error {
	methods := map[string]applets.Procedure[any, any]{
		"get": {
			Handler: func(ctx context.Context, params any) (any, error) {
				p, ok := params.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid params: %w", applets.ErrInvalid)
				}
				id, _ := p["id"].(string)
				if id == "" {
					return nil, fmt.Errorf("id is required: %w", applets.ErrInvalid)
				}
				return s.store.Get(ctx, id)
			},
		},
		"query": {
			Handler: func(ctx context.Context, params any) (any, error) {
				p, ok := params.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid params: %w", applets.ErrInvalid)
				}
				table, _ := p["table"].(string)
				if table == "" {
					return nil, fmt.Errorf("table is required: %w", applets.ErrInvalid)
				}
				return s.store.Query(ctx, table)
			},
		},
		"insert": {
			Handler: func(ctx context.Context, params any) (any, error) {
				p, ok := params.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid params: %w", applets.ErrInvalid)
				}
				table, _ := p["table"].(string)
				if table == "" {
					return nil, fmt.Errorf("table is required: %w", applets.ErrInvalid)
				}
				value := p["value"]
				return s.store.Insert(ctx, table, value)
			},
		},
		"patch": {
			Handler: func(ctx context.Context, params any) (any, error) {
				p, ok := params.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid params: %w", applets.ErrInvalid)
				}
				id, _ := p["id"].(string)
				if id == "" {
					return nil, fmt.Errorf("id is required: %w", applets.ErrInvalid)
				}
				return s.store.Patch(ctx, id, p["value"])
			},
		},
		"replace": {
			Handler: func(ctx context.Context, params any) (any, error) {
				p, ok := params.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid params: %w", applets.ErrInvalid)
				}
				id, _ := p["id"].(string)
				if id == "" {
					return nil, fmt.Errorf("id is required: %w", applets.ErrInvalid)
				}
				return s.store.Replace(ctx, id, p["value"])
			},
		},
		"delete": {
			Handler: func(ctx context.Context, params any) (any, error) {
				p, ok := params.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid params: %w", applets.ErrInvalid)
				}
				id, _ := p["id"].(string)
				if id == "" {
					return nil, fmt.Errorf("id is required: %w", applets.ErrInvalid)
				}
				return s.store.Delete(ctx, id)
			},
		},
	}

	for op, procedure := range methods {
		router := applets.NewTypedRPCRouter()
		if err := applets.AddProcedure(router, fmt.Sprintf("%s.db.%s", appletName, op), procedure); err != nil {
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

func (s *memoryDBStore) Get(ctx context.Context, id string) (any, error) {
	scope := scopeFromContext(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()
	scopeRecords := s.records[scope]
	if scopeRecords == nil {
		return nil, nil
	}
	record, ok := scopeRecords[id]
	if !ok {
		return nil, nil
	}
	return dbRecordResponse(record), nil
}

func (s *memoryDBStore) Query(ctx context.Context, table string) ([]any, error) {
	scope := scopeFromContext(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()
	scopeRecords := s.records[scope]
	if scopeRecords == nil {
		return []any{}, nil
	}
	out := make([]any, 0)
	for _, record := range scopeRecords {
		if record.Table != table {
			continue
		}
		out = append(out, dbRecordResponse(record))
	}
	return out, nil
}

func (s *memoryDBStore) Insert(ctx context.Context, table string, value any) (any, error) {
	scope := scopeFromContext(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sequence++
	id := fmt.Sprintf("%s-%d-%d", table, time.Now().UnixNano(), s.sequence)
	scopeRecords := s.records[scope]
	if scopeRecords == nil {
		scopeRecords = make(map[string]documentRecord)
		s.records[scope] = scopeRecords
	}
	record := documentRecord{ID: id, Table: table, Value: value}
	scopeRecords[id] = record
	return dbRecordResponse(record), nil
}

func (s *memoryDBStore) Patch(ctx context.Context, id string, value any) (any, error) {
	return s.update(ctx, id, value, false)
}

func (s *memoryDBStore) Replace(ctx context.Context, id string, value any) (any, error) {
	return s.update(ctx, id, value, true)
}

func (s *memoryDBStore) update(ctx context.Context, id string, value any, strict bool) (any, error) {
	scope := scopeFromContext(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	scopeRecords := s.records[scope]
	if scopeRecords == nil {
		if strict {
			return nil, fmt.Errorf("document not found: %w", applets.ErrNotFound)
		}
		return nil, nil
	}
	record, ok := scopeRecords[id]
	if !ok {
		if strict {
			return nil, fmt.Errorf("document not found: %w", applets.ErrNotFound)
		}
		return nil, nil
	}
	record.Value = value
	scopeRecords[id] = record
	return dbRecordResponse(record), nil
}

func (s *memoryDBStore) Delete(ctx context.Context, id string) (bool, error) {
	scope := scopeFromContext(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	scopeRecords := s.records[scope]
	if scopeRecords == nil {
		return false, nil
	}
	if _, exists := scopeRecords[id]; !exists {
		return false, nil
	}
	delete(scopeRecords, id)
	return true, nil
}

func dbRecordResponse(record documentRecord) map[string]any {
	return map[string]any{
		"_id":   record.ID,
		"table": record.Table,
		"value": record.Value,
	}
}
