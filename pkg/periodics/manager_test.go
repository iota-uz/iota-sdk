package periodics

import (
	"context"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testPeriodicTask struct {
	name       string
	schedule   string
	runOnStart bool
	config     TaskConfig
	execute    func()
}

func (t *testPeriodicTask) Name() string       { return t.name }
func (t *testPeriodicTask) Schedule() string   { return t.schedule }
func (t *testPeriodicTask) Config() TaskConfig { return t.config }
func (t *testPeriodicTask) RunOnStart() bool   { return t.runOnStart }
func (t *testPeriodicTask) Execute(_ context.Context) error {
	if t.execute != nil {
		t.execute()
	}
	return nil
}

func TestManagerExecutorForTaskReusesWrappedExecutor(t *testing.T) {
	m := NewManager(logrus.New(), nil, uuid.Nil).(*manager)
	task := &testPeriodicTask{name: "reindex_spotlight", schedule: "* * * * *"}

	require.NoError(t, m.AddTask(task))

	exec1 := m.executorForTask(task)
	exec2 := m.executorForTask(task)

	assert.Equal(t, reflect.ValueOf(exec1).Pointer(), reflect.ValueOf(exec2).Pointer())
}

func TestManagerSharedExecutorPreventsConcurrentRunsAcrossTriggers(t *testing.T) {
	m := NewManager(logrus.New(), nil, uuid.Nil).(*manager)
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var runs atomic.Int32

	task := &testPeriodicTask{
		name:     "reindex_spotlight",
		schedule: "* * * * *",
		config: TaskConfig{
			EnableSkipIfRunning: BoolPtr(true),
		},
		execute: func() {
			runs.Add(1)
			started <- struct{}{}
			<-release
		},
	}

	require.NoError(t, m.AddTask(task))
	executor := m.executorForTask(task)

	done1 := make(chan struct{})
	go func() {
		defer close(done1)
		executor()
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first execution did not start")
	}

	done2 := make(chan struct{})
	go func() {
		defer close(done2)
		executor()
	}()

	select {
	case <-done2:
	case <-time.After(2 * time.Second):
		t.Fatal("second execution should have been skipped immediately")
	}

	close(release)

	select {
	case <-done1:
	case <-time.After(2 * time.Second):
		t.Fatal("first execution did not finish")
	}

	assert.Equal(t, int32(1), runs.Load())
}
