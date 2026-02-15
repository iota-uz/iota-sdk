package handlers

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/iota-uz/applets"
	appletenginerpc "github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
)

type DBStore interface {
	Get(ctx context.Context, id string) (any, error)
	Query(ctx context.Context, table string, options DBQueryOptions) ([]any, error)
	Insert(ctx context.Context, table string, value any) (any, error)
	Patch(ctx context.Context, id string, value any) (any, error)
	Replace(ctx context.Context, id string, value any) (any, error)
	Delete(ctx context.Context, id string) (bool, error)
}

type documentRecord struct {
	ID        string
	Table     string
	Value     any
	CreatedAt time.Time
	UpdatedAt time.Time
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
				result, err := s.store.Get(ctx, id)
				if errors.Is(err, applets.ErrNotFound) {
					return nullResult(), nil
				}
				return result, err
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
				options, err := parseDBQueryOptions(p["query"])
				if err != nil {
					return nil, fmt.Errorf("invalid query options: %w", applets.ErrInvalid)
				}
				return s.store.Query(ctx, table, options)
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
				result, err := s.store.Patch(ctx, id, p["value"])
				if errors.Is(err, applets.ErrNotFound) {
					return nullResult(), nil
				}
				return result, err
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
	scope, err := scopeFromContext(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	scopeRecords := s.records[scope]
	if scopeRecords == nil {
		return nil, fmt.Errorf("document not found: %w", applets.ErrNotFound)
	}
	record, ok := scopeRecords[id]
	if !ok {
		return nil, fmt.Errorf("document not found: %w", applets.ErrNotFound)
	}
	return dbRecordResponse(record), nil
}

func (s *memoryDBStore) Query(ctx context.Context, table string, options DBQueryOptions) ([]any, error) {
	scope, err := scopeFromContext(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	scopeRecords := s.records[scope]
	if scopeRecords == nil {
		return []any{}, nil
	}
	filtered := make([]documentRecord, 0)
	for _, record := range scopeRecords {
		if record.Table != table {
			continue
		}
		if !matchesQuery(record.Value, options) {
			continue
		}
		filtered = append(filtered, record)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if options.Order == "asc" {
			return filtered[i].UpdatedAt.Before(filtered[j].UpdatedAt)
		}
		return filtered[i].UpdatedAt.After(filtered[j].UpdatedAt)
	})
	if options.Limit > 0 && len(filtered) > options.Limit {
		filtered = filtered[:options.Limit]
	}
	out := make([]any, 0, len(filtered))
	for _, record := range filtered {
		out = append(out, dbRecordResponse(record))
	}
	return out, nil
}

func (s *memoryDBStore) Insert(ctx context.Context, table string, value any) (any, error) {
	scope, err := scopeFromContext(ctx)
	if err != nil {
		return nil, err
	}
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
	now := time.Now().UTC()
	record.CreatedAt = now
	record.UpdatedAt = now
	scopeRecords[id] = record
	return dbRecordResponse(record), nil
}

func (s *memoryDBStore) Patch(ctx context.Context, id string, value any) (any, error) {
	return s.update(ctx, id, value)
}

func (s *memoryDBStore) Replace(ctx context.Context, id string, value any) (any, error) {
	return s.update(ctx, id, value)
}

func (s *memoryDBStore) update(ctx context.Context, id string, value any) (any, error) {
	scope, err := scopeFromContext(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	scopeRecords := s.records[scope]
	if scopeRecords == nil {
		return nil, fmt.Errorf("document not found: %w", applets.ErrNotFound)
	}
	record, ok := scopeRecords[id]
	if !ok {
		return nil, fmt.Errorf("document not found: %w", applets.ErrNotFound)
	}
	record.Value = value
	record.UpdatedAt = time.Now().UTC()
	scopeRecords[id] = record
	return dbRecordResponse(record), nil
}

func (s *memoryDBStore) Delete(ctx context.Context, id string) (bool, error) {
	scope, err := scopeFromContext(ctx)
	if err != nil {
		return false, err
	}
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

func matchesQuery(value any, options DBQueryOptions) bool {
	if options.Index != nil {
		if !constraintMatches(value, options.Index.DBConstraint) {
			return false
		}
	}
	for _, filter := range options.Filter {
		if !constraintMatches(value, filter) {
			return false
		}
	}
	return true
}

func constraintMatches(value any, constraint DBConstraint) bool {
	if constraint.Op != "eq" {
		return false
	}
	actual, ok := nestedFieldValue(value, constraint.Field)
	if !ok {
		return false
	}
	return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", constraint.Value)
}

func nestedFieldValue(value any, fieldPath string) (any, bool) {
	current := value
	for _, segment := range strings.Split(strings.TrimSpace(fieldPath), ".") {
		asMap, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := asMap[segment]
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func nullResult() any {
	var v *struct{}
	return v
}
