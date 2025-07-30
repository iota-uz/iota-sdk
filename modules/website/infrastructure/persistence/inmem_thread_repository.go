package persistence

import (
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/chatthread"
)

type SafeMap[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
}

func NewSafeMap[K comparable, V any]() *SafeMap[K, V] {
	return &SafeMap[K, V]{
		m: make(map[K]V),
	}
}

func (s *SafeMap[K, V]) Set(key K, value V) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
}

func (s *SafeMap[K, V]) Get(key K) (V, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	val, found := s.m[key]
	return val, found
}

func (s *SafeMap[K, V]) Delete(key K) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
}

type InmemThreadRepository struct {
	mu      sync.RWMutex
	storage *SafeMap[uuid.UUID, chatthread.ChatThread]
}

func NewInmemThreadRepository() *InmemThreadRepository {
	return &InmemThreadRepository{
		storage: NewSafeMap[uuid.UUID, chatthread.ChatThread](),
	}
}

func (r *InmemThreadRepository) GetByID(ctx context.Context, id uuid.UUID) (chatthread.ChatThread, error) {
	thread, found := r.storage.Get(id)
	if !found {
		return nil, chatthread.ErrChatThreadNotFound
	}
	return thread, nil
}

func (r *InmemThreadRepository) Save(ctx context.Context, thread chatthread.ChatThread) (chatthread.ChatThread, error) {
	r.storage.Set(thread.ID(), thread)
	return thread, nil
}

func (r *InmemThreadRepository) Delete(ctx context.Context, id uuid.UUID) error {
	r.storage.Delete(id)
	return nil
}

func (r *InmemThreadRepository) List(ctx context.Context) ([]chatthread.ChatThread, error) {
	return slices.Collect(maps.Values(r.storage.m)), nil
}
