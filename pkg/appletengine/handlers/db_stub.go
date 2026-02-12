package handlers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/iota-uz/applets"
	appletenginerpc "github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
)

type documentRecord struct {
	ID    string
	Table string
	Value any
}

type DBStub struct {
	mu       sync.RWMutex
	records  map[string]map[string]documentRecord
	sequence int64
}

func NewDBStub() *DBStub {
	return &DBStub{
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
				return s.get(ctx, id), nil
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
				return s.query(ctx, table), nil
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
				return s.insert(ctx, table, value), nil
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
				return s.replace(ctx, id, p["value"], false)
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
				return s.replace(ctx, id, p["value"], true)
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
				return s.del(ctx, id), nil
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

func (s *DBStub) get(ctx context.Context, id string) any {
	scope := scopeFromContext(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()
	scopeRecords := s.records[scope]
	if scopeRecords == nil {
		return nil
	}
	record, ok := scopeRecords[id]
	if !ok {
		return nil
	}
	return map[string]any{
		"_id":   record.ID,
		"table": record.Table,
		"value": record.Value,
	}
}

func (s *DBStub) query(ctx context.Context, table string) []any {
	scope := scopeFromContext(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()
	scopeRecords := s.records[scope]
	if scopeRecords == nil {
		return []any{}
	}
	out := make([]any, 0)
	for _, record := range scopeRecords {
		if record.Table != table {
			continue
		}
		out = append(out, map[string]any{
			"_id":   record.ID,
			"table": record.Table,
			"value": record.Value,
		})
	}
	return out
}

func (s *DBStub) insert(ctx context.Context, table string, value any) any {
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
	record := documentRecord{
		ID:    id,
		Table: table,
		Value: value,
	}
	scopeRecords[id] = record
	return map[string]any{
		"_id":   record.ID,
		"table": record.Table,
		"value": record.Value,
	}
}

func (s *DBStub) replace(ctx context.Context, id string, value any, strict bool) (any, error) {
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
	return map[string]any{
		"_id":   record.ID,
		"table": record.Table,
		"value": record.Value,
	}, nil
}

func (s *DBStub) del(ctx context.Context, id string) bool {
	scope := scopeFromContext(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	scopeRecords := s.records[scope]
	if scopeRecords == nil {
		return false
	}
	if _, exists := scopeRecords[id]; !exists {
		return false
	}
	delete(scopeRecords, id)
	return true
}
