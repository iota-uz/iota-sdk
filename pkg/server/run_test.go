package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type routeController struct {
	routes map[string]http.HandlerFunc
}

func (c routeController) Register(r *mux.Router) {
	for path, h := range c.routes {
		r.HandleFunc(path, h)
	}
}

func (c routeController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("test.routes", 0)
}

type fakeApp struct {
	application.Application
	controllers []application.Controller
}

func (a fakeApp) Controllers() []application.Controller { return a.controllers }
func (a fakeApp) Middleware() []mux.MiddlewareFunc      { return nil }

type recordingLifecycle struct {
	record func(string)
}

func (l recordingLifecycle) Shutdown(context.Context) error {
	l.record("runtime")
	return nil
}

func newTestServer(routes map[string]http.HandlerFunc) *HTTPServer {
	return NewHTTPServer(fakeApp{controllers: []application.Controller{routeController{routes: routes}}}, http.NotFoundHandler(), http.NotFoundHandler())
}

func get(client *http.Client, url string) (*http.Response, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func startRun(t *testing.T, s *HTTPServer, opts ...RunOption) (context.CancelFunc, string, <-chan error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	opts = append([]RunOption{WithListener(ln), WithSignals()}, opts...)
	go func() { done <- s.Run(ctx, "", opts...) }()
	return cancel, "http://" + ln.Addr().String(), done
}

func TestRun_DrainsInFlightRequestsThenRunsStopHooksInReverseOrder(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	release := make(chan struct{})
	var mu sync.Mutex
	var events []string
	record := func(e string) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, e)
	}

	s := newTestServer(map[string]http.HandlerFunc{
		"/slow": func(w http.ResponseWriter, _ *http.Request) {
			close(started)
			<-release
			record("request finished")
			_, _ = io.WriteString(w, "done")
		},
	})
	cancel, base, done := startRun(t, s,
		WithStopHook("first", func(context.Context) error { record("first"); return nil }),
		WithStopHook("second", func(context.Context) error { record("second"); return nil }),
		WithRuntime(recordingLifecycle{record: record}),
	)

	type result struct {
		body string
		err  error
	}
	resp := make(chan result, 1)
	go func() {
		r, err := get(nil, base+"/slow")
		if err != nil {
			resp <- result{err: err}
			return
		}
		defer func() { _ = r.Body.Close() }()
		b, err := io.ReadAll(r.Body)
		resp <- result{body: string(b), err: err}
	}()
	<-started
	cancel()

	require.Eventually(t, s.Draining, time.Second, 5*time.Millisecond)
	select {
	case err := <-done:
		t.Fatalf("Run returned before the in-flight request finished: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)

	got := <-resp
	require.NoError(t, got.err)
	assert.Equal(t, "done", got.body)
	require.NoError(t, <-done)
	assert.Equal(t, []string{"request finished", "second", "first", "runtime"}, events)
}

func TestRun_ShutdownTimeoutClosesStuckRequestsAndStillStops(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	stuck := make(chan struct{})
	defer close(stuck)
	stopped := make(chan struct{})
	s := newTestServer(map[string]http.HandlerFunc{
		"/stuck": func(http.ResponseWriter, *http.Request) {
			close(started)
			<-stuck
		},
	})
	cancel, base, done := startRun(t, s,
		WithShutdownTimeout(50*time.Millisecond),
		WithRuntime(recordingLifecycle{record: func(string) { close(stopped) }}),
	)
	go func() {
		r, err := get(nil, base+"/stuck")
		if err == nil {
			_ = r.Body.Close()
		}
	}()
	<-started
	cancel()

	err := <-done
	require.Error(t, err)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	select {
	case <-stopped:
	default:
		t.Fatal("runtime shutdown did not run after the shutdown timeout")
	}
}

func TestRun_ReadinessReportsUnavailableWhileDraining(t *testing.T) {
	t.Parallel()

	s := newTestServer(map[string]http.HandlerFunc{
		"/health": func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") },
	})
	cancel, base, done := startRun(t, s, WithDrainDelay(300*time.Millisecond))

	r, err := get(nil, base+"/health")
	require.NoError(t, err)
	_ = r.Body.Close()
	assert.Equal(t, http.StatusOK, r.StatusCode)

	cancel()
	require.Eventually(t, s.Draining, time.Second, 5*time.Millisecond)
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	r, err = get(client, base+"/health")
	require.NoError(t, err)
	body, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, r.StatusCode)
	assert.Contains(t, string(body), "shutting_down")
	require.NoError(t, <-done)
}

func TestRun_StopHookErrorsAreReturned(t *testing.T) {
	t.Parallel()

	s := newTestServer(nil)
	boom := errors.New("boom")
	cancel, _, done := startRun(t, s, WithStopHook("broken", func(context.Context) error { return boom }))
	cancel()
	err := <-done
	require.ErrorIs(t, err, boom)
	assert.Contains(t, err.Error(), `stop hook "broken"`)
}

func TestRun_ListenErrorStillTearsDown(t *testing.T) {
	t.Parallel()

	busy, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = busy.Close() }()

	stopped := false
	s := newTestServer(nil)
	err = s.Run(context.Background(), busy.Addr().String(), WithSignals(),
		WithRuntime(recordingLifecycle{record: func(string) { stopped = true }}))
	require.Error(t, err)
	assert.True(t, stopped)
}

func TestServer_WriteTimeoutDoesNotCutEventStreams(t *testing.T) {
	t.Parallel()

	s := newTestServer(map[string]http.HandlerFunc{
		"/events": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher := w.(http.Flusher)
			for i := range 4 {
				_, _ = fmt.Fprintf(w, "data: %d\n\n", i)
				flusher.Flush()
				time.Sleep(60 * time.Millisecond)
			}
		},
		"/slow": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "partial")
			w.(http.Flusher).Flush()
			time.Sleep(250 * time.Millisecond)
			_, _ = io.WriteString(w, strings.Repeat("x", 1<<20))
		},
	})
	s.Timeouts.Write = 100 * time.Millisecond
	cancel, base, done := startRun(t, s)
	defer func() {
		cancel()
		<-done
	}()

	r, err := get(nil, base+"/events")
	require.NoError(t, err)
	body, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	require.NoError(t, err)
	assert.Equal(t, 4, strings.Count(string(body), "data: "))

	// A regular response that outlives WriteTimeout is cut off: either the
	// request fails outright or the body comes back truncated.
	r, err = get(nil, base+"/slow")
	if err == nil {
		body, readErr := io.ReadAll(r.Body)
		_ = r.Body.Close()
		assert.True(t, readErr != nil || len(body) < 1<<20, "regular responses must still honour WriteTimeout")
	}
}
