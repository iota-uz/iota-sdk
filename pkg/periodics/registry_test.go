package periodics

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagerRegistry_Register(t *testing.T) {
	reg := NewManagerRegistry()

	m1 := NewManager(nil, nil, [16]byte{})
	m2 := NewManager(nil, nil, [16]byte{})

	require.NoError(t, reg.Register("mod1", m1))
	require.NoError(t, reg.Register("mod2", m2))

	all := reg.All()
	assert.Len(t, all, 2)
	assert.Equal(t, m1, all["mod1"])
	assert.Equal(t, m2, all["mod2"])
}

func TestManagerRegistry_Register_Duplicate(t *testing.T) {
	reg := NewManagerRegistry()
	m := NewManager(nil, nil, [16]byte{})

	require.NoError(t, reg.Register("mod1", m))

	err := reg.Register("mod1", m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mod1")
}

func TestManagerRegistry_All_ReturnsCopy(t *testing.T) {
	reg := NewManagerRegistry()
	m := NewManager(nil, nil, [16]byte{})
	require.NoError(t, reg.Register("mod1", m))

	all := reg.All()
	all["injected"] = m

	assert.Len(t, reg.All(), 1, "All() should return a copy, not the internal map")
}

func TestManagerRegistry_ConcurrentAccess(t *testing.T) {
	reg := NewManagerRegistry()
	const n = 100
	var wg sync.WaitGroup

	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m := NewManager(nil, nil, [16]byte{})
			_ = reg.Register(fmt.Sprintf("mod%d", i), m)
			_ = reg.All()
		}(i)
	}
	wg.Wait()

	all := reg.All()
	assert.Len(t, all, n, "all %d unique registrations should succeed", n)
}

func TestGetRegisteredTasks(t *testing.T) {
	m := NewManager(nil, nil, [16]byte{})

	tasks := m.GetRegisteredTasks()
	assert.Empty(t, tasks)
}

func TestManagerRegistry_StopAll(t *testing.T) {
	reg := NewManagerRegistry()
	m := NewManager(nil, nil, [16]byte{})
	require.NoError(t, reg.Register("mod1", m))

	err := reg.StopAll(context.Background())
	assert.NoError(t, err, "StopAll on non-started managers should not error")
}

func TestGetTaskScheduleInfo_NoCron(t *testing.T) {
	m := NewManager(nil, nil, [16]byte{})

	info := m.GetTaskScheduleInfo()
	assert.Empty(t, info)
}
