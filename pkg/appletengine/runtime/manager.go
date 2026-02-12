package runtime

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	appletenginerpc "github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
	"github.com/sirupsen/logrus"
)

const (
	healthPath             = "/__health"
	healthCheckTimeout     = 8 * time.Second
	healthCheckPollDelay   = 150 * time.Millisecond
	maxRestartBackoff      = 30 * time.Second
	defaultShutdownTimeout = 3 * time.Second
	maxUnixSocketPath      = 100
)

type AppletProcess struct {
	AppletID     string
	EntryPoint   string
	AppletSocket string
	Cmd          *exec.Cmd
	StartedAt    time.Time
}

type Manager struct {
	mu              sync.Mutex
	baseDir         string
	logger          *logrus.Logger
	dispatcher      *appletenginerpc.Dispatcher
	engineSocket    string
	engineListener  net.Listener
	engineHTTP      *http.Server
	processes       map[string]*AppletProcess
	restartAttempts map[string]int
	entrypoints     map[string]string
	fileStores      map[string]FileStore
	shuttingDown    bool
}

type FileStore interface {
	Store(ctx context.Context, name, contentType string, data []byte) (map[string]any, error)
	Get(ctx context.Context, id string) (map[string]any, bool, error)
	Delete(ctx context.Context, id string) (bool, error)
}

func NewManager(baseDir string, dispatcher *appletenginerpc.Dispatcher, logger *logrus.Logger) *Manager {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	if baseDir == "" {
		baseDir = filepath.Join(os.TempDir(), "iota-applet-engine")
	}
	return &Manager{
		baseDir:         baseDir,
		dispatcher:      dispatcher,
		logger:          logger,
		processes:       make(map[string]*AppletProcess),
		restartAttempts: make(map[string]int),
		entrypoints:     make(map[string]string),
		fileStores:      make(map[string]FileStore),
	}
}

func (m *Manager) RegisterApplet(appletID, entryPoint string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entrypoints[appletID] = entryPoint
}

func (m *Manager) RegisterFileStore(appletID string, store FileStore) {
	if store == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fileStores[appletID] = store
}

func (m *Manager) EnsureStarted(ctx context.Context, appletID, entryPoint string) (*AppletProcess, error) {
	if !EnabledForApplet(appletID) {
		return nil, nil
	}
	if strings.TrimSpace(entryPoint) == "" {
		m.mu.Lock()
		entryPoint = m.entrypoints[appletID]
		m.mu.Unlock()
	}
	if strings.TrimSpace(entryPoint) == "" {
		return nil, fmt.Errorf("entry point is required for applet %q", appletID)
	}
	if err := m.ensureEngineSocket(); err != nil {
		return nil, err
	}

	m.mu.Lock()
	if m.shuttingDown {
		m.mu.Unlock()
		return nil, fmt.Errorf("runtime manager is shutting down")
	}
	if process := m.processes[appletID]; process != nil && isRunning(process.Cmd) {
		m.mu.Unlock()
		return process, nil
	}
	m.mu.Unlock()

	process, err := m.startProcess(ctx, appletID, entryPoint)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.processes[appletID] = process
	m.restartAttempts[appletID] = 0
	m.mu.Unlock()

	go m.monitor(appletID)

	return process, nil
}

func (m *Manager) monitor(appletID string) {
	for {
		m.mu.Lock()
		if m.shuttingDown {
			m.mu.Unlock()
			return
		}
		process := m.processes[appletID]
		m.mu.Unlock()

		if process == nil || process.Cmd == nil {
			return
		}

		err := process.Cmd.Wait()
		if m.isShuttingDown() {
			return
		}

		if err == nil {
			m.logger.WithField("applet", appletID).Info("bun applet process exited")
		} else {
			m.logger.WithField("applet", appletID).WithError(err).Error("bun applet process crashed")
		}

		attempt := m.bumpRestartAttempt(appletID)
		backoff := restartBackoff(attempt)
		m.logger.WithField("applet", appletID).WithField("backoff", backoff.String()).Warn("restarting bun applet process")

		timer := time.NewTimer(backoff)
		<-timer.C

		if m.isShuttingDown() {
			return
		}

		restartCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		next, startErr := m.startProcess(restartCtx, appletID, process.EntryPoint)
		cancel()
		if startErr != nil {
			m.logger.WithField("applet", appletID).WithError(startErr).Error("failed to restart bun applet process")
			continue
		}

		m.mu.Lock()
		m.processes[appletID] = next
		m.restartAttempts[appletID] = 0
		m.mu.Unlock()
	}
}

func (m *Manager) startProcess(ctx context.Context, appletID, entryPoint string) (*AppletProcess, error) {
	if err := os.MkdirAll(m.baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create runtime directory: %w", err)
	}

	appletSocket := m.resolveSocketPath(fmt.Sprintf("%s.sock", appletID))
	_ = os.Remove(appletSocket)

	bunBin := strings.TrimSpace(os.Getenv("IOTA_APPLET_ENGINE_BUN_BIN"))
	if bunBin == "" {
		bunBin = "bun"
	}

	cmd := exec.CommandContext(ctx, bunBin, "run", entryPoint)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("IOTA_APPLET_ID=%s", appletID),
		fmt.Sprintf("IOTA_ENGINE_SOCKET=%s", m.EngineSocketPath()),
		fmt.Sprintf("IOTA_APPLET_SOCKET=%s", appletSocket),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start bun applet %q: %w", appletID, err)
	}

	process := &AppletProcess{
		AppletID:     appletID,
		EntryPoint:   entryPoint,
		AppletSocket: appletSocket,
		Cmd:          cmd,
		StartedAt:    time.Now(),
	}

	if err := waitForHealth(ctx, appletSocket, healthCheckTimeout); err != nil {
		_ = terminateProcess(process.Cmd, defaultShutdownTimeout)
		return nil, fmt.Errorf("bun applet %q health check failed: %w", appletID, err)
	}

	m.logger.WithField("applet", appletID).WithField("entrypoint", entryPoint).Info("bun applet process started")
	return process, nil
}

func (m *Manager) ensureEngineSocket() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.engineListener != nil {
		return nil
	}
	if err := os.MkdirAll(m.baseDir, 0o755); err != nil {
		return fmt.Errorf("create runtime directory: %w", err)
	}
	socketPath := m.resolveSocketPath("engine.sock")
	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("listen engine socket: %w", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", m.dispatcher.HandleServerOnlyHTTP)
	mux.HandleFunc("/files/store", m.handleFileStore)
	mux.HandleFunc("/files/get", m.handleFileGet)
	mux.HandleFunc("/files/delete", m.handleFileDelete)
	server := &http.Server{Handler: mux}
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			m.logger.WithError(serveErr).Error("engine unix socket server stopped unexpectedly")
		}
	}()

	m.engineSocket = socketPath
	m.engineListener = listener
	m.engineHTTP = server
	return nil
}

func (m *Manager) EngineSocketPath() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.engineSocket
}

func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	m.shuttingDown = true
	processes := make([]*AppletProcess, 0, len(m.processes))
	for _, p := range m.processes {
		processes = append(processes, p)
	}
	server := m.engineHTTP
	listener := m.engineListener
	m.mu.Unlock()

	for _, process := range processes {
		if process == nil || process.Cmd == nil {
			continue
		}
		if process.Cmd.Process != nil && isRunning(process.Cmd) {
			_ = process.Cmd.Process.Signal(syscall.SIGTERM)
		}
	}
	time.Sleep(defaultShutdownTimeout)
	for _, process := range processes {
		if process == nil || process.Cmd == nil || process.Cmd.Process == nil {
			continue
		}
		if isRunning(process.Cmd) {
			_ = process.Cmd.Process.Kill()
		}
	}
	if server != nil {
		_ = server.Shutdown(ctx)
	}
	if listener != nil {
		_ = listener.Close()
	}
	return nil
}

func (m *Manager) DispatchJob(ctx context.Context, appletID, tenantID, jobID, method string, params any) error {
	process, err := m.EnsureStarted(ctx, appletID, "")
	if err != nil {
		return err
	}
	if process == nil {
		return fmt.Errorf("applet runtime %q is disabled", appletID)
	}
	payload := map[string]any{
		"jobId":    jobID,
		"method":   method,
		"params":   params,
		"applet":   appletID,
		"tenantId": tenantID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal job payload: %w", err)
	}
	statusCode, err := m.postToAppletSocket(ctx, process.AppletSocket, "/__job", body, map[string]string{
		"X-Iota-Tenant-Id":  tenantID,
		"X-Iota-Request-Id": fmt.Sprintf("job-%s", jobID),
	})
	if err != nil {
		return err
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("applet job endpoint returned status %d", statusCode)
	}
	return nil
}

func (m *Manager) DispatchWebsocketEvent(ctx context.Context, appletID, tenantID, connectionID, event string, data []byte) error {
	process, err := m.EnsureStarted(ctx, appletID, "")
	if err != nil {
		return err
	}
	if process == nil {
		return fmt.Errorf("applet runtime %q is disabled", appletID)
	}
	payload := map[string]any{
		"appletId":     appletID,
		"tenantId":     tenantID,
		"connectionId": connectionID,
		"event":        event,
	}
	if len(data) > 0 {
		payload["dataBase64"] = encodeBase64(data)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal websocket event payload: %w", err)
	}
	statusCode, err := m.postToAppletSocket(ctx, process.AppletSocket, "/__ws", body, map[string]string{
		"X-Iota-Tenant-Id":  tenantID,
		"X-Iota-Request-Id": fmt.Sprintf("ws-%s", connectionID),
	})
	if err != nil {
		return err
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("applet websocket endpoint returned status %d", statusCode)
	}
	return nil
}

func (m *Manager) isShuttingDown() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.shuttingDown
}

func (m *Manager) bumpRestartAttempt(appletID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.restartAttempts[appletID]++
	return m.restartAttempts[appletID]
}

func restartBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return time.Second
	}
	backoff := time.Duration(1<<minInt(attempt, 5)) * time.Second
	if backoff > maxRestartBackoff {
		return maxRestartBackoff
	}
	return backoff
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func isRunning(cmd *exec.Cmd) bool {
	if cmd == nil || cmd.Process == nil {
		return false
	}
	if cmd.ProcessState == nil {
		return true
	}
	return !cmd.ProcessState.Exited()
}

func terminateProcess(cmd *exec.Cmd, timeout time.Duration) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if !isRunning(cmd) {
		return nil
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()

	select {
	case err := <-waitDone:
		if err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		return nil
	case <-time.After(timeout):
		return cmd.Process.Kill()
	}
}

func waitForHealth(ctx context.Context, socketPath string, timeout time.Duration) error {
	dialer := &net.Dialer{}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}
	client := &http.Client{Transport: transport}
	defer transport.CloseIdleConnections()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix"+healthPath, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
		}
		time.Sleep(healthCheckPollDelay)
	}
	return fmt.Errorf("health endpoint %s not ready within %s", healthPath, timeout)
}

func (m *Manager) postToAppletSocket(ctx context.Context, socketPath, path string, body []byte, headers map[string]string) (int, error) {
	dialer := &net.Dialer{}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{Transport: transport}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://unix"+path, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("build applet request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("dispatch job to applet: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func (m *Manager) fileStoreFromRequest(ctx context.Context, r *http.Request) (FileStore, context.Context, error) {
	appletID := strings.TrimSpace(r.Header.Get("X-Iota-Applet-Id"))
	if appletID == "" {
		appletID = strings.TrimSpace(r.URL.Query().Get("applet"))
	}
	if appletID == "" {
		return nil, ctx, fmt.Errorf("missing X-Iota-Applet-Id")
	}
	tenantID := strings.TrimSpace(r.Header.Get("X-Iota-Tenant-Id"))
	if tenantID == "" {
		tenantID = "default"
	}
	m.mu.Lock()
	store := m.fileStores[appletID]
	m.mu.Unlock()
	if store == nil {
		return nil, ctx, fmt.Errorf("file store is not configured for applet %q", appletID)
	}
	ctx = appletenginerpc.WithAppletID(ctx, appletID)
	ctx = appletenginerpc.WithTenantID(ctx, tenantID)
	return store, ctx, nil
}

func (m *Manager) handleFileStore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	store, ctx, err := m.fileStoreFromRequest(r.Context(), r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer func() { _ = r.Body.Close() }()
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read payload", http.StatusBadRequest)
		return
	}
	fileName := strings.TrimSpace(r.Header.Get("X-Iota-File-Name"))
	if fileName == "" {
		fileName = strings.TrimSpace(r.URL.Query().Get("name"))
	}
	contentType := strings.TrimSpace(r.Header.Get("X-Iota-Content-Type"))
	if contentType == "" {
		contentType = strings.TrimSpace(r.Header.Get("Content-Type"))
	}
	result, err := store.Store(ctx, fileName, contentType, payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, http.StatusOK, result)
}

func (m *Manager) handleFileGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	store, ctx, err := m.fileStoreFromRequest(r.Context(), r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fileID := strings.TrimSpace(r.URL.Query().Get("id"))
	if fileID == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	result, found, err := store.Get(ctx, fileID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		writeJSONResponse(w, http.StatusOK, nil)
		return
	}
	writeJSONResponse(w, http.StatusOK, result)
}

func (m *Manager) handleFileDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	store, ctx, err := m.fileStoreFromRequest(r.Context(), r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fileID := strings.TrimSpace(r.URL.Query().Get("id"))
	if fileID == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	ok, err := store.Delete(ctx, fileID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"ok": ok})
}

func writeJSONResponse(w http.ResponseWriter, status int, payload any) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func (m *Manager) resolveSocketPath(fileName string) string {
	candidate := filepath.Join(m.baseDir, fileName)
	if len(candidate) < maxUnixSocketPath {
		return candidate
	}
	sum := sha1.Sum([]byte(m.baseDir))
	hash := hex.EncodeToString(sum[:])[:12]
	shortDir := filepath.Join("/tmp", "iota-ae-"+hash)
	_ = os.MkdirAll(shortDir, 0o755)
	return filepath.Join(shortDir, fileName)
}
