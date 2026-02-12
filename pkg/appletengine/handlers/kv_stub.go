package handlers

import (
	"context"
	"fmt"
	"sync"

	"github.com/iota-uz/applets"
	appletenginerpc "github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
)

type KVStore interface {
	Get(ctx context.Context, key string) (any, error)
	Set(ctx context.Context, key string, value any, ttlSeconds *int) error
	Delete(ctx context.Context, key string) (bool, error)
	MGet(ctx context.Context, keys []string) ([]any, error)
}

type KVStub struct {
	store KVStore
}

type memoryKVStore struct {
	mu    sync.RWMutex
	store map[string]map[string]any
}

func NewKVStub() *KVStub {
	return &KVStub{store: newMemoryKVStore()}
}

func NewKVStubWithStore(store KVStore) *KVStub {
	if store == nil {
		store = newMemoryKVStore()
	}
	return &KVStub{store: store}
}

func newMemoryKVStore() *memoryKVStore {
	return &memoryKVStore{
		store: make(map[string]map[string]any),
	}
}

func (s *KVStub) Register(registry *appletenginerpc.Registry, appletName string) error {
	methods := map[string]applets.Procedure[any, any]{
		"get": {
			Handler: func(ctx context.Context, params any) (any, error) {
				p, ok := params.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid params: %w", applets.ErrInvalid)
				}
				key, _ := p["key"].(string)
				if key == "" {
					return nil, fmt.Errorf("key is required: %w", applets.ErrInvalid)
				}
				return s.store.Get(ctx, key)
			},
		},
		"set": {
			Handler: func(ctx context.Context, params any) (any, error) {
				p, ok := params.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid params: %w", applets.ErrInvalid)
				}
				key, _ := p["key"].(string)
				if key == "" {
					return nil, fmt.Errorf("key is required: %w", applets.ErrInvalid)
				}
				var ttlSeconds *int
				if ttlRaw, ok := p["ttlSeconds"]; ok {
					switch v := ttlRaw.(type) {
					case float64:
						ttl := int(v)
						ttlSeconds = &ttl
					case int:
						ttl := v
						ttlSeconds = &ttl
					}
				}
				if err := s.store.Set(ctx, key, p["value"], ttlSeconds); err != nil {
					return nil, err
				}
				return map[string]any{"ok": true}, nil
			},
		},
		"del": {
			Handler: func(ctx context.Context, params any) (any, error) {
				p, ok := params.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid params: %w", applets.ErrInvalid)
				}
				key, _ := p["key"].(string)
				if key == "" {
					return nil, fmt.Errorf("key is required: %w", applets.ErrInvalid)
				}
				return s.store.Delete(ctx, key)
			},
		},
		"mget": {
			Handler: func(ctx context.Context, params any) (any, error) {
				p, ok := params.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid params: %w", applets.ErrInvalid)
				}
				raw, _ := p["keys"].([]any)
				keys := make([]string, 0, len(raw))
				for _, item := range raw {
					if key, ok := item.(string); ok && key != "" {
						keys = append(keys, key)
					}
				}
				return s.store.MGet(ctx, keys)
			},
		},
	}

	for op, procedure := range methods {
		router := applets.NewTypedRPCRouter()
		if err := applets.AddProcedure(router, fmt.Sprintf("%s.kv.%s", appletName, op), procedure); err != nil {
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

func (s *memoryKVStore) Get(ctx context.Context, key string) (any, error) {
	scope := scopeFromContext(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()
	bucket := s.store[scope]
	if bucket == nil {
		return nil, nil
	}
	value, ok := bucket[key]
	if !ok {
		return nil, nil
	}
	return value, nil
}

func (s *memoryKVStore) Set(ctx context.Context, key string, value any, _ *int) error {
	scope := scopeFromContext(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket := s.store[scope]
	if bucket == nil {
		bucket = make(map[string]any)
		s.store[scope] = bucket
	}
	bucket[key] = value
	return nil
}

func (s *memoryKVStore) Delete(ctx context.Context, key string) (bool, error) {
	scope := scopeFromContext(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket := s.store[scope]
	if bucket == nil {
		return false, nil
	}
	if _, exists := bucket[key]; !exists {
		return false, nil
	}
	delete(bucket, key)
	return true, nil
}

func (s *memoryKVStore) MGet(ctx context.Context, keys []string) ([]any, error) {
	values := make([]any, 0, len(keys))
	for _, key := range keys {
		value, err := s.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func scopeFromContext(ctx context.Context) string {
	tenantID, ok := appletenginerpc.TenantIDFromContext(ctx)
	if !ok {
		tenantID = "default"
	}
	appletID, ok := appletenginerpc.AppletIDFromContext(ctx)
	if !ok {
		appletID = "unknown"
	}
	return tenantID + "::" + appletID
}
