package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/gorilla/mux"
	"github.com/iota-uz/applets"
	"github.com/sirupsen/logrus"
)

const defaultMaxBodyBytes int64 = 1 << 20

type request struct {
	ID     any             `json:"id,omitempty"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type response struct {
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
	JSONRPC string    `json:"jsonrpc,omitempty"`
}

type rpcError struct {
	// Code is a JSON-RPC error code. Protocol errors use standard integers (-32600, -32601, -32603).
	// Application errors use string codes ("forbidden", "validation", "not_found", etc.) for better DX.
	Code    any    `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type Dispatcher struct {
	registry       *Registry
	host           applets.HostServices
	logger         *logrus.Logger
	maxBodySize    int64
	beforeDispatch func(context.Context, string) error
	bunCaller      BunPublicCaller
}

type BunPublicCaller interface {
	CallPublicMethod(ctx context.Context, appletID, method string, params json.RawMessage, headers http.Header) (any, error)
}

type dispatchTransport int

const (
	transportPublic dispatchTransport = iota + 1
	transportInternal
)

func NewDispatcher(registry *Registry, host applets.HostServices, logger *logrus.Logger) *Dispatcher {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	return &Dispatcher{
		registry:    registry,
		host:        host,
		logger:      logger,
		maxBodySize: defaultMaxBodyBytes,
	}
}

func (d *Dispatcher) HandlePublicHTTP(w http.ResponseWriter, r *http.Request) {
	d.handleHTTP(w, r, transportPublic)
}

func (d *Dispatcher) HandleServerOnlyHTTP(w http.ResponseWriter, r *http.Request) {
	d.handleHTTP(w, r, transportInternal)
}

func (d *Dispatcher) SetBeforeDispatch(hook func(context.Context, string) error) {
	d.beforeDispatch = hook
}

func (d *Dispatcher) SetBunPublicCaller(caller BunPublicCaller) {
	d.bunCaller = caller
}

func (d *Dispatcher) handleHTTP(w http.ResponseWriter, r *http.Request, transport dispatchTransport) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, d.maxBodySize)
	defer func() { _ = r.Body.Close() }()

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			d.writeJSON(w, http.StatusRequestEntityTooLarge, response{
				Error:   &rpcError{Code: -32600, Message: "Invalid Request"},
				JSONRPC: "2.0",
			})
			return
		}
		d.writeJSON(w, http.StatusBadRequest, response{
			Error:   &rpcError{Code: -32600, Message: "Invalid Request"},
			JSONRPC: "2.0",
		})
		return
	}
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		d.writeJSON(w, http.StatusBadRequest, response{
			Error:   &rpcError{Code: -32600, Message: "Invalid Request"},
			JSONRPC: "2.0",
		})
		return
	}

	ctx := d.ctxFromHeaders(r.Context(), r.Header)
	if trimmed[0] == '[' {
		var batch []request
		if err := json.Unmarshal(trimmed, &batch); err != nil || len(batch) == 0 {
			d.writeJSON(w, http.StatusBadRequest, []response{{
				Error:   &rpcError{Code: -32600, Message: "Invalid Request"},
				JSONRPC: "2.0",
			}})
			return
		}
		out := make([]response, 0, len(batch))
		for _, req := range batch {
			out = append(out, d.dispatch(ctx, req, transport, r.Header, r))
		}
		d.writeJSON(w, http.StatusOK, out)
		return
	}

	var req request
	if err := json.Unmarshal(trimmed, &req); err != nil {
		d.writeJSON(w, http.StatusBadRequest, response{
			Error:   &rpcError{Code: -32600, Message: "Invalid Request"},
			JSONRPC: "2.0",
		})
		return
	}
	resp := d.dispatch(ctx, req, transport, r.Header, r)
	d.writeJSON(w, http.StatusOK, resp)
}

func (d *Dispatcher) dispatch(baseCtx context.Context, req request, transport dispatchTransport, headers http.Header, httpReq *http.Request) response {
	id := req.ID
	methodName := strings.TrimSpace(req.Method)
	if methodName == "" {
		return response{
			ID:      id,
			Error:   &rpcError{Code: -32600, Message: "Invalid Request"},
			JSONRPC: "2.0",
		}
	}

	method, ok := d.registry.Get(methodName)
	if !ok || !allowedOnTransport(method.Visibility, transport) {
		return response{
			ID:      id,
			Error:   &rpcError{Code: -32601, Message: "Method not found"},
			JSONRPC: "2.0",
		}
	}
	if d.beforeDispatch != nil {
		if err := d.beforeDispatch(baseCtx, method.AppletName); err != nil {
			if d.logger != nil {
				d.logger.WithField("method", methodName).WithError(err).Error("applet rpc pre-dispatch hook failed")
			}
			return response{
				ID:      id,
				Error:   &rpcError{Code: -32603, Message: "Internal error"},
				JSONRPC: "2.0",
			}
		}
	}

	exec := func(ctx context.Context) (any, error) {
		return method.Method.Handler(ctx, req.Params)
	}
	if method.Visibility == visibilityPublic && transport == transportPublic && method.Target == MethodTargetBun {
		if d.bunCaller == nil {
			return response{
				ID:      id,
				Error:   &rpcError{Code: -32603, Message: "Internal error"},
				JSONRPC: "2.0",
			}
		}
		exec = func(ctx context.Context) (any, error) {
			return d.bunCaller.CallPublicMethod(ctx, method.AppletName, method.Name, req.Params, headers)
		}
	}

	result, rpcErr := d.executeWithMiddleware(baseCtx, httpReq, method, exec)
	if rpcErr != nil {
		return response{
			ID:      id,
			Error:   rpcErr,
			JSONRPC: "2.0",
		}
	}

	return response{
		ID:      id,
		Result:  result,
		JSONRPC: "2.0",
	}
}

func allowedOnTransport(v visibility, transport dispatchTransport) bool {
	switch transport {
	case transportPublic:
		return v == visibilityPublic
	case transportInternal:
		// Internal transport can reach full registry (public + server-only).
		return v == visibilityPublic || v == visibilityServerOnly
	default:
		return false
	}
}

func (d *Dispatcher) executeWithMiddleware(baseCtx context.Context, httpReq *http.Request, method Method, execute func(context.Context) (any, error)) (any, *rpcError) {
	ctx := WithAppletID(baseCtx, method.AppletName)

	var result any
	var handlerErr error
	ran := false

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ran = true
		if err := d.requirePermissions(r.Context(), method.Method.RequirePermissions); err != nil {
			handlerErr = err
			return
		}
		result, handlerErr = execute(r.Context())
	})

	wrapped := applyMiddleware(finalHandler, method.Middlewares)
	reqClone := httpReq.Clone(ctx)
	recorder := httptest.NewRecorder()
	wrapped.ServeHTTP(recorder, reqClone)

	if !ran {
		switch {
		case recorder.Code == http.StatusUnauthorized:
			return nil, &rpcError{Code: "unauthorized", Message: "authentication required"}
		case recorder.Code == http.StatusForbidden:
			return nil, &rpcError{Code: "forbidden", Message: "permission denied"}
		case recorder.Code == http.StatusTooManyRequests:
			return nil, &rpcError{Code: "rate_limited", Message: "too many requests"}
		case recorder.Code >= 400 && recorder.Code < 500:
			return nil, &rpcError{Code: "forbidden", Message: "request blocked by middleware"}
		default:
			return nil, &rpcError{Code: -32603, Message: "Internal error"}
		}
	}
	if handlerErr != nil {
		if d.logger != nil {
			d.logger.WithField("method", method.Name).WithError(handlerErr).Error("applet rpc handler error")
		}
		return nil, mapExecutionError(handlerErr)
	}
	return result, nil
}

type rpcErrorCarrier interface {
	RPCCode() any
	RPCMessage() string
}

type rpcErrorDetailsCarrier interface {
	RPCDetails() any
}

func mapExecutionError(err error) *rpcError {
	var carrier rpcErrorCarrier
	if errors.As(err, &carrier) {
		out := &rpcError{
			Code:    carrier.RPCCode(),
			Message: carrier.RPCMessage(),
		}
		var detailsCarrier rpcErrorDetailsCarrier
		if errors.As(err, &detailsCarrier) {
			out.Details = detailsCarrier.RPCDetails()
		}
		return out
	}

	code := mapErrorCode(err)
	return &rpcError{
		Code:    code,
		Message: mapErrorMessage(code),
	}
}

func applyMiddleware(final http.Handler, middlewares []mux.MiddlewareFunc) http.Handler {
	h := final
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

func (d *Dispatcher) requirePermissions(ctx context.Context, required []string) error {
	if len(required) == 0 {
		return nil
	}
	if d.host == nil {
		return fmt.Errorf("missing host services: %w", applets.ErrPermissionDenied)
	}
	user, err := d.host.ExtractUser(ctx)
	if err != nil || user == nil {
		return fmt.Errorf("extract user: %w", applets.ErrPermissionDenied)
	}
	for _, permission := range required {
		if permission == "" {
			continue
		}
		if !user.HasPermission(permission) {
			return fmt.Errorf("missing permission %q: %w", permission, applets.ErrPermissionDenied)
		}
	}
	return nil
}

func mapErrorCode(err error) any {
	switch {
	case errors.Is(err, applets.ErrValidation):
		return "validation"
	case errors.Is(err, applets.ErrInvalid):
		return "invalid"
	case errors.Is(err, applets.ErrNotFound):
		return "not_found"
	case errors.Is(err, applets.ErrPermissionDenied):
		return "forbidden"
	case errors.Is(err, applets.ErrInternal):
		return "internal"
	}
	type classifier interface {
		ErrorKind() string
	}
	var c classifier
	if errors.As(err, &c) {
		if kind := strings.TrimSpace(c.ErrorKind()); kind != "" {
			return kind
		}
	}
	return "error"
}

func mapErrorMessage(code any) string {
	switch code {
	case "forbidden":
		return "permission denied"
	case "validation":
		return "validation failed"
	case "invalid":
		return "invalid request"
	case "not_found":
		return "resource not found"
	case "internal":
		return "internal error"
	default:
		return "request failed"
	}
}

func (d *Dispatcher) writeJSON(w http.ResponseWriter, status int, payload any) {
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

func (d *Dispatcher) ctxFromHeaders(ctx context.Context, headers http.Header) context.Context {
	if tenantID := strings.TrimSpace(headers.Get("X-Iota-Tenant-Id")); tenantID != "" {
		ctx = WithTenantID(ctx, tenantID)
	}
	if userID := strings.TrimSpace(headers.Get("X-Iota-User-Id")); userID != "" {
		ctx = WithUserID(ctx, userID)
	}
	if requestID := strings.TrimSpace(headers.Get("X-Iota-Request-Id")); requestID != "" {
		ctx = WithRequestID(ctx, requestID)
	}
	return ctx
}
