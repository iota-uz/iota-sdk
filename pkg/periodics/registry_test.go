package periodics

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

func TestManagerRegistry_Register(t *testing.T) {
	reg := NewManagerRegistry()

	m1 := NewManager(nil, nil, [16]byte{})
	m2 := NewManager(nil, nil, [16]byte{})

	if err := reg.Register("mod1", m1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := reg.Register("mod2", m2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	all := reg.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 managers, got %d", len(all))
	}
	if all["mod1"] != m1 || all["mod2"] != m2 {
		t.Fatal("managers don't match")
	}
}

func TestManagerRegistry_Register_Duplicate(t *testing.T) {
	reg := NewManagerRegistry()
	m := NewManager(nil, nil, [16]byte{})

	if err := reg.Register("mod1", m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err := reg.Register("mod1", m)
	if err == nil {
		t.Fatal("expected error on duplicate registration")
	}
}

func TestManagerRegistry_All_ReturnsCopy(t *testing.T) {
	reg := NewManagerRegistry()
	m := NewManager(nil, nil, [16]byte{})
	_ = reg.Register("mod1", m)

	all := reg.All()
	all["injected"] = m

	// Original should be unchanged
	if len(reg.All()) != 1 {
		t.Fatal("All() should return a copy, not the internal map")
	}
}

func TestManagerRegistry_ConcurrentAccess(t *testing.T) {
	reg := NewManagerRegistry()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m := NewManager(nil, nil, [16]byte{})
			_ = reg.Register(fmt.Sprintf("mod%d", i), m)
			_ = reg.All()
		}(i)
	}
	wg.Wait()
}

func TestGetRegisteredTasks(t *testing.T) {
	m := NewManager(nil, nil, [16]byte{})

	tasks := m.GetRegisteredTasks()
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestManagerRegistry_StopAll(t *testing.T) {
	reg := NewManagerRegistry()
	m := NewManager(nil, nil, [16]byte{})
	_ = reg.Register("mod1", m)

	// StopAll on non-started managers should not error
	err := reg.StopAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetTaskScheduleInfo_NoCron(t *testing.T) {
	m := NewManager(nil, nil, [16]byte{})

	info := m.GetTaskScheduleInfo()
	if len(info) != 0 {
		t.Fatalf("expected empty schedule info, got %d entries", len(info))
	}
}
