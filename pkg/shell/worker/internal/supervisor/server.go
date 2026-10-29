// Package supervisor serves the authenticated internal API of the shell
// worker. Each accepted run goes to a fresh child process; the supervisor
// itself never executes guest scripts.
package supervisor

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/shell/worker/internal/protocol"
)

const RunsPath = "/v1/runs"

type Config struct {
	Token        string
	ChildCommand []string
	QueueSize    int
	RetryAfter   time.Duration
	KillGrace    time.Duration
}

type Server struct {
	cfg    Config
	queue  *queue
	logger *slog.Logger
}

func New(cfg Config, logger *slog.Logger) (*Server, error) {
	switch {
	case cfg.Token == "":
		return nil, errors.New("supervisor: token is required")
	case len(cfg.ChildCommand) == 0:
		return nil, errors.New("supervisor: child command is required")
	case cfg.QueueSize < 1:
		return nil, errors.New("supervisor: queue size must be positive")
	}
	return &Server{cfg: cfg, queue: newQueue(cfg.QueueSize), logger: logger}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+RunsPath, s.handleRun)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		s.writeError(w, http.StatusUnauthorized, &protocol.Error{Code: protocol.CodeUnauthorized, Message: "invalid token"})
		return
	}

	if err := s.queue.reserve(); err != nil {
		retry := s.cfg.RetryAfter.Milliseconds()
		w.Header().Set("Retry-After", strconv.FormatInt(max(retry/1000, 1), 10))
		s.writeError(w, http.StatusTooManyRequests, &protocol.Error{Code: protocol.CodeBusy, Message: err.Error(), RetryAfterMS: retry})
		return
	}
	req, ok := s.readRequest(w, r)
	if !ok {
		s.queue.leave()
		return
	}
	release, err := s.queue.acquire(r.Context())
	if err != nil {
		return
	}
	defer release()

	runID := newRunID()
	started := time.Now()
	res, out, err := s.runChild(r.Context(), req)
	duration := time.Since(started)
	if err != nil {
		if r.Context().Err() == nil {
			s.logger.Error("shell run failed", "run_id", runID, "error", err)
			s.writeError(w, http.StatusServiceUnavailable, &protocol.Error{Code: protocol.CodeDependencyUnavailable, Message: "shell run could not be started"})
		} else {
			s.logger.Info("shell run abandoned by client", "run_id", runID, "duration_ms", duration.Milliseconds())
		}
		return
	}
	if out != nil {
		limit := out.limit
		res = &protocol.Result{Stderr: out.message, LimitHit: &limit, Outputs: []protocol.OutputFile{}}
		s.logger.Warn("shell run terminated", "run_id", runID, "limit_hit", out.limit, "detail", out.detail)
	}
	res.RunID = runID
	res.DurationMS = duration.Milliseconds()
	attrs := []any{"run_id", runID, "duration_ms", res.DurationMS, "outputs", len(res.Outputs)}
	if res.ExitCode != nil {
		attrs = append(attrs, "exit_code", *res.ExitCode)
	}
	if res.LimitHit != nil {
		attrs = append(attrs, "limit_hit", *res.LimitHit)
	}
	s.logger.Info("shell run finished", attrs...)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(res); err != nil {
		s.logger.Warn("shell result not delivered", "run_id", runID, "error", err)
	}
}

func (s *Server) readRequest(w http.ResponseWriter, r *http.Request) (*protocol.Request, bool) {
	var req protocol.Request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, protocol.MaxRequestBytes)).Decode(&req); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			s.writeError(w, http.StatusRequestEntityTooLarge, &protocol.Error{Code: protocol.CodeLimitExceeded, Message: "request is too large"})
			return nil, false
		}
		s.writeError(w, http.StatusBadRequest, &protocol.Error{Code: protocol.CodeInvalidInput, Message: "request is not valid JSON"})
		return nil, false
	}
	if verr := req.Validate(); verr != nil {
		status := http.StatusBadRequest
		if verr.Code == protocol.CodeLimitExceeded {
			status = http.StatusRequestEntityTooLarge
		}
		s.writeError(w, status, verr)
		return nil, false
	}
	return &req, true
}

func (s *Server) authorized(r *http.Request) bool {
	got := []byte(r.Header.Get("Authorization"))
	want := []byte("Bearer " + s.cfg.Token)
	return subtle.ConstantTimeCompare(got, want) == 1
}

func newRunID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) writeError(w http.ResponseWriter, status int, perr *protocol.Error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(perr); err != nil {
		s.logger.Warn("shell error not delivered", "code", perr.Code, "error", err)
	}
}
