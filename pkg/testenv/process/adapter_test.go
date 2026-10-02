package process

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/dbctl/testdb"
	"github.com/iota-uz/iota-sdk/pkg/testenv"
	"github.com/stretchr/testify/require"
)

type memoryResources struct {
	mu    sync.Mutex
	owned map[string]bool
}

func (r *memoryResources) Prepare(_ context.Context, id string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.owned[id] = true
	return nil, nil
}
func (r *memoryResources) Dispose(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.owned, id)
	return nil
}

func TestServerChild(t *testing.T) {
	if os.Getenv("TESTENV_PROCESS_CHILD") != "1" {
		return
	}
	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, "ready") })
	if err := http.ListenAndServe("127.0.0.1:"+os.Getenv("HTTP_PORT"), nil); err != nil {
		os.Exit(2)
	}
}

// Falsely green if descriptors alone are compared without reaching both live processes.
func TestCoordinatorOwnsSeparateProcesses(t *testing.T) {
	r := &memoryResources{owned: map[string]bool{}}
	a, err := New(Config{Command: []string{os.Args[0], "-test.run=^TestServerChild$"}, Environment: []string{"TESTENV_PROCESS_CHILD=1"}, Artifacts: t.TempDir(), Revision: "test", Manifest: testdb.Manifest{SchemaFingerprint: "schema", BaselineFingerprint: "seed", BuildRevision: "test"}, Resources: r, ReadyPath: "/ready"})
	require.NoError(t, err)
	c := testenv.NewCoordinator(a)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	spec := testenv.Spec{RunID: "test", Slot: "one", Isolation: "attempt", SchemaFingerprint: "schema", BaselineFingerprint: "seed"}
	one, err := c.Start(ctx, spec)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, c.Stop(context.Background(), one.EnvironmentID)) })
	spec.Slot = "two"
	two, err := c.Start(ctx, spec)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, c.Stop(context.Background(), two.EnvironmentID)) })
	require.NotEqual(t, one.BaseURL, two.BaseURL)
	require.NoError(t, c.Stop(ctx, one.EnvironmentID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, two.BaseURL+"/ready", nil)
	require.NoError(t, err)
	response, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusOK, response.StatusCode)
	r.mu.Lock()
	require.False(t, r.owned[one.EnvironmentID])
	require.True(t, r.owned[two.EnvironmentID])
	r.mu.Unlock()
}

// Falsely green if only the error is asserted and the prepared resources leak.
func TestFailedExecutableDisposesResources(t *testing.T) {
	r := &memoryResources{owned: map[string]bool{}}
	a, err := New(Config{Command: []string{"/nonexistent-testenv-executable"}, Artifacts: t.TempDir(), Revision: "test", Manifest: testdb.Manifest{SchemaFingerprint: "schema", BaselineFingerprint: "seed", BuildRevision: "test"}, Resources: r})
	require.NoError(t, err)
	c := testenv.NewCoordinator(a)
	_, err = c.Start(context.Background(), testenv.Spec{RunID: "test", Slot: "one", Isolation: "attempt", SchemaFingerprint: "schema", BaselineFingerprint: "seed"})
	require.Error(t, err)
	r.mu.Lock()
	require.Empty(t, r.owned)
	r.mu.Unlock()
}
