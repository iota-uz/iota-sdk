package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/listext"
)

// Invoker runs a root invocation; *host.Host implements it.
type Invoker interface {
	Invoke(ctx context.Context, req host.Request) (host.Result, error)
}

const maxRPCBody = 1 << 20

// rpcRequest is the body of POST /_ext/rpc.
type rpcRequest struct {
	Kind     string          `json:"kind"`
	Resource string          `json:"resource"`
	Input    json.RawMessage `json:"input"`
}

// RemoteIssue and RemoteError are the wire error of the browser transport:
// the same {kind, code, params, issues} the contract declares, unchanged.
type remoteIssue struct {
	Code   string          `json:"code"`
	Phase  string          `json:"phase,omitempty"`
	Source string          `json:"source,omitempty"`
	Path   json.RawMessage `json:"path,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
}

type remoteError struct {
	Kind    string          `json:"kind"`
	Code    string          `json:"code"`
	Message string          `json:"message,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Issues  []remoteIssue   `json:"issues,omitempty"`
}

type rpcResponse struct {
	OK    bool            `json:"ok"`
	Value json.RawMessage `json:"value,omitempty"`
	Error *remoteError    `json:"error,omitempty"`
	// Invocation lets support find the recorded invocation.
	Invocation string `json:"invocation,omitempty"`
}

func writeRPC(w http.ResponseWriter, status int, body rpcResponse) {
	data, err := json.Marshal(body)
	if err != nil {
		// A Value that is not valid JSON cannot be sent as a result.
		status = http.StatusInternalServerError
		data = []byte(`{"ok":false,"error":{"kind":"internal","code":"ui.encode","message":"the response could not be encoded"}}`)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(append(data, '\n'))
}

func rpcFail(w http.ResponseWriter, status int, kind pool.ErrorKind, code, message string) {
	writeRPC(w, status, rpcResponse{Error: &remoteError{Kind: string(kind), Code: code, Message: message}})
}

// Codes of errors raised by the UI transport itself.
const (
	CodeUnauthenticated = "ui.unauthenticated"
	CodeBadRequest      = "ui.badRequest"
	CodeNotCallable     = "ui.notCallable"
	CodeUnknownResource = "ui.unknownResource"
	CodeNoBinding       = "ui.noBinding"
	CodeUnavailable     = "ui.unavailable"
)

func statusOf(kind pool.ErrorKind) int {
	switch kind {
	case pool.ErrKindDenied:
		return http.StatusForbidden
	case pool.ErrKindDomain, pool.ErrKindValidation:
		return http.StatusUnprocessableEntity
	case pool.ErrKindInternal, pool.ErrKindSuspend:
		return http.StatusInternalServerError
	}
	return http.StatusInternalServerError
}

// rpcHandler serves POST /_ext/rpc: it invokes a query or command as the
// session user, with the permissions the user holds now.
func (c *Controller) rpcHandler(w http.ResponseWriter, r *http.Request) {
	if c.cfg.Invoker == nil {
		rpcFail(w, http.StatusServiceUnavailable, pool.ErrKindInternal, CodeUnavailable, "extension runtime is not available")
		return
	}
	sub, err := listext.SubjectFromContext(r.Context())
	if err != nil {
		rpcFail(w, http.StatusUnauthorized, pool.ErrKindDenied, CodeUnauthenticated, "sign in required")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRPCBody))
	if err != nil {
		rpcFail(w, http.StatusRequestEntityTooLarge, pool.ErrKindValidation, CodeBadRequest, "request body is too large")
		return
	}
	var req rpcRequest
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || req.Resource == "" {
		rpcFail(w, http.StatusBadRequest, pool.ErrKindValidation, CodeBadRequest, "malformed request")
		return
	}
	res, ok, err := c.cfg.Catalog.Resolve(r.Context(), req.Resource)
	if err != nil {
		c.cfg.Logger.WithError(err).Error("extruntime ui: resolve contract")
		rpcFail(w, http.StatusInternalServerError, pool.ErrKindInternal, "runtime.catalog", "internal error")
		return
	}
	if !ok {
		rpcFail(w, http.StatusNotFound, pool.ErrKindDenied, CodeUnknownResource, "unknown resource")
		return
	}
	// The browser calls queries and commands only; operations are internal
	// building blocks reached through tx.call.
	switch req.Kind {
	case "query":
		ok = res.Resource.Kind == manifest.ResQuery
	case "command":
		ok = res.Resource.Kind == manifest.ResCommand && !res.Resource.Atomic
	case "atomic":
		ok = res.Resource.Kind == manifest.ResCommand && res.Resource.Atomic
	default:
		ok = false
	}
	if !ok {
		rpcFail(w, http.StatusForbidden, pool.ErrKindDenied, CodeNotCallable, "resource cannot be called from the browser")
		return
	}
	if !sub.Has(res.Resource.Permission) {
		rpcFail(w, http.StatusForbidden, pool.ErrKindDenied, "ui.permissionDenied", "permission denied")
		return
	}
	input := req.Input
	if len(input) == 0 {
		input = json.RawMessage("null")
	}
	result, err := c.cfg.Invoker.Invoke(r.Context(), host.Request{
		Contract: req.Resource,
		Input:    input,
		Subject:  sub,
		Trigger:  host.Trigger{Kind: host.TriggerUI, Ref: triggerRef(r)},
	})
	if err != nil {
		var we *pool.WireError
		switch {
		case errors.As(err, &we):
			out := &remoteError{Kind: string(we.Kind), Code: we.Code, Message: we.Message, Params: we.Params}
			for _, is := range we.Issues {
				out.Issues = append(out.Issues, remoteIssue{Code: is.Code, Phase: is.Phase, Source: is.Source, Path: is.Path, Params: is.Params})
			}
			writeRPC(w, statusOf(we.Kind), rpcResponse{Error: out, Invocation: result.InvocationID})
		case errors.Is(err, host.ErrNoBinding):
			rpcFail(w, http.StatusConflict, pool.ErrKindInternal, CodeNoBinding, "no active implementation")
		default:
			c.cfg.Logger.WithError(err).WithField("contract", req.Resource).Error("extruntime ui: invoke")
			writeRPC(w, http.StatusInternalServerError, rpcResponse{
				Error:      &remoteError{Kind: string(pool.ErrKindInternal), Code: "runtime.internal", Message: "internal error"},
				Invocation: result.InvocationID,
			})
		}
		return
	}
	out := result.Output
	if len(out) == 0 {
		out = json.RawMessage("null")
	}
	writeRPC(w, http.StatusOK, rpcResponse{OK: true, Value: out, Invocation: result.InvocationID})
}

// triggerRef identifies the calling page for the invocation record: the
// referring path, bounded; the header is client-controlled, so it is data only.
func triggerRef(r *http.Request) string {
	ref := r.Header.Get("Referer")
	if u, err := url.Parse(ref); err == nil && u.Path != "" {
		ref = u.Path
	}
	if len(ref) > 256 {
		ref = ref[:256]
	}
	return ref
}
