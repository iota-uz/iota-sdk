package handlers

import (
	"context"
	"fmt"
	"sync"

	"github.com/iota-uz/applets"
	appletenginerpc "github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
)

type KVStub struct {
	mu    sync.RWMutex
	store map[string]map[string]any
}

type kvGetParams struct {
	Key string `json:"key"`
}

type kvSetParams struct {
	Key        string `json:"key"`
	Value      any    `json:"value"`
	TTLSeconds *int   `json:"ttlSeconds,omitempty"`
}

type kvDelParams struct {
	Key string `json:"key"`
}

type kvMGetParams struct {
	Keys []string `json:"keys"`
}

func NewKVStub() *KVStub {
	return &KVStub{
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
				return s.get(ctx, key), nil
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
				s.set(ctx, key, p["value"])
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
				return s.del(ctx, key), nil
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
				return s.mget(ctx, keys), nil
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

func (s *KVStub) get(ctx context.Context, key string) any {
	scope := scopeFromContext(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()
	bucket := s.store[scope]
	if bucket == nil {
		return nil
	}
	value, ok := bucket[key]
	if !ok {
		return nil
	}
	return value
}

func (s *KVStub) set(ctx context.Context, key string, value any) {
	scope := scopeFromContext(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket := s.store[scope]
	if bucket == nil {
		bucket = make(map[string]any)
		s.store[scope] = bucket
	}
	bucket[key] = value
}

func (s *KVStub) del(ctx context.Context, key string) bool {
	scope := scopeFromContext(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket := s.store[scope]
	if bucket == nil {
		return false
	}
	if _, exists := bucket[key]; !exists {
		return false
	}
	delete(bucket, key)
	return true
}

func (s *KVStub) mget(ctx context.Context, keys []string) []any {
	values := make([]any, 0, len(keys))
	for _, key := range keys {
		values = append(values, s.get(ctx, key))
	}
	return values
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
