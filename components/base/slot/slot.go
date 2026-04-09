package slot

import (
	"context"
	"fmt"
	"sync"

	"github.com/a-h/templ"
)

type Key string

type Chunk struct {
	templ.Component
}

type RenderFunc func(ctx context.Context, push func(templ.Component))

type SlotSource interface {
	Name() Key
	Fallback() templ.Component
	Stream(ctx context.Context) <-chan Chunk
	Empty() bool
}

type Manager interface {
	Add(slot SlotSource)
	Define(name Key, render RenderFunc, opts ...SlotSourceOption)
	Async(name Key, fn func(ctx context.Context) (templ.Component, error), opts ...SlotSourceOption)
	Get(name Key) SlotSource
}

func NewManager() Manager {
	return &manager{
		slots: make(map[Key]SlotSource),
	}
}

func NewSlotSource(name Key, opts ...SlotSourceOption) SlotSource {
	source := &slotSource{
		name: name,
	}
	for _, opt := range opts {
		opt(source)
	}
	return source
}

type manager struct {
	mu    sync.RWMutex
	slots map[Key]SlotSource
}

func (m *manager) Get(name Key) SlotSource {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s, ok := m.slots[name]; ok {
		return s
	}
	return &noopSlotSource{name: name}
}

func (m *manager) Add(s SlotSource) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.slots[s.Name()] = s
}

func (m *manager) Define(name Key, render RenderFunc, opts ...SlotSourceOption) {
	options := []SlotSourceOption{
		WithSlotSourceRenderFunc(render),
	}
	opts = append(options, opts...)
	m.Add(NewSlotSource(name, opts...))
}

func (m *manager) Async(name Key, fn func(ctx context.Context) (templ.Component, error), opts ...SlotSourceOption) {
	m.Define(name, func(ctx context.Context, push func(templ.Component)) {
		c, err := fn(ctx)
		if err != nil {
			push(templ.Raw(fmt.Sprintf("error: %v", err)))
			return
		}
		push(c)
	}, opts...)
}

type slotSource struct {
	name       Key
	fallback   templ.Component
	renderFunc RenderFunc
}

type SlotSourceOption func(s *slotSource)

func (s *slotSource) Name() Key { return s.name }
func (s *slotSource) Fallback() templ.Component {
	if s.fallback == nil {
		return templ.Raw("")
	}
	return s.fallback
}
func (s *slotSource) Empty() bool { return false }

func (s *slotSource) Stream(ctx context.Context) <-chan Chunk {
	ch := make(chan Chunk)
	if s.renderFunc == nil {
		close(ch)
		return ch
	}
	go func() {
		defer close(ch)
		push := func(c templ.Component) {
			select {
			case ch <- Chunk{Component: c}:
			case <-ctx.Done():
				return
			}
		}
		defer func() {
			if r := recover(); r != nil {
				push(templ.Raw(fmt.Sprintf("panic: %v", r)))
			}
		}()
		s.renderFunc(ctx, push)
	}()
	return ch
}

type noopSlotSource struct {
	name Key
}

func (n *noopSlotSource) Name() Key                 { return n.name }
func (n *noopSlotSource) Fallback() templ.Component { return templ.Raw("") }
func (n *noopSlotSource) Empty() bool               { return true }
func (n *noopSlotSource) Stream(_ context.Context) <-chan Chunk {
	ch := make(chan Chunk)
	close(ch)
	return ch
}

func WithSlotSourceFallback(fallback templ.Component) SlotSourceOption {
	return func(s *slotSource) {
		s.fallback = fallback
	}
}

func WithSlotSourceRenderFunc(renderFunc RenderFunc) SlotSourceOption {
	return func(s *slotSource) {
		s.renderFunc = renderFunc
	}
}
