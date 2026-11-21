package solid

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

// RPC error codes understood by the browser host (asHostError in
// @iota-uz/sdk/solid). A handler returns any other code through *RPCError;
// the screen reads it from the error details.
const (
	CodeUnauthenticated = "unauthenticated"
	CodeForbidden       = "forbidden"
	CodeValidation      = "validation"
	CodeFieldValidation = "field_validation"
	CodeUnavailable     = "unavailable"
	CodeProtocol        = "protocol"
	CodeNotFound        = "not_found"
	CodeInternal        = "internal"
)

// RPCError is an error a method returns to the screen as is: its code,
// message and data reach the browser. Any other error is logged and sent as
// CodeInternal without its text, so a wrapped driver or provider error never
// reaches the page.
type RPCError struct {
	Code    string
	Message string
	Data    any
}

func (e *RPCError) Error() string { return e.Code + ": " + e.Message }

// NewRPCError is an *RPCError.
func NewRPCError(code, message string, data any) *RPCError {
	return &RPCError{Code: code, Message: message, Data: data}
}

// Binding is a Go implementation of a declared RPC method.
type Binding struct {
	method Method
	call   func(ctx context.Context, params json.RawMessage) (any, error)
}

// Bind implements method with fn. The request and response types are the ones
// the method was declared with (solid.Query[Req, Resp] / solid.Action), so the
// generated TypeScript contract matches what fn decodes and returns.
func Bind[Request, Response any](method Method, fn func(context.Context, Request) (Response, error)) Binding {
	return Binding{method: method, call: func(ctx context.Context, params json.RawMessage) (any, error) {
		var req Request
		if len(bytes.TrimSpace(params)) > 0 && !bytes.Equal(bytes.TrimSpace(params), []byte("null")) {
			// Numbers inside untyped values (any, map[string]any) stay exact json.Number: an ID beyond
			// 2^53 must not pass through float64.
			dec := json.NewDecoder(bytes.NewReader(params))
			dec.UseNumber()
			if err := dec.Decode(&req); err != nil {
				return nil, NewRPCError(CodeValidation, "the request does not match the method's parameters", nil)
			}
		}
		return fn(ctx, req)
	}}
}

// RPCHandler serves Solid RPC methods over the JSON-RPC 2.0 envelope the
// browser transport (FetchRPCTransport) posts:
//
//	{"jsonrpc":"2.0","id":1,"method":"…","params":{…}}
//	→ {"jsonrpc":"2.0","id":1,"result":…} | {"jsonrpc":"2.0","id":1,"error":{"code":"…","message":"…","data":…}}
//
// Authentication, the tenant and the user are the request context's: the
// handler is mounted behind the application's session middleware.
type RPCHandler struct {
	methods map[string]Binding
	guard   func(*http.Request) error
	maxBody int64
	logger  *slog.Logger
}

// RPCOption configures an RPCHandler.
type RPCOption func(*RPCHandler)

// WithRPCGuard rejects a request before dispatch (a cross-site check); the
// error is answered with 403 and CodeForbidden.
func WithRPCGuard(guard func(*http.Request) error) RPCOption {
	return func(h *RPCHandler) { h.guard = guard }
}

// WithRPCMaxBody bounds the request body (default 1 MiB).
func WithRPCMaxBody(n int64) RPCOption { return func(h *RPCHandler) { h.maxBody = n } }

// WithRPCLogger logs internal errors (default slog.Default()).
func WithRPCLogger(l *slog.Logger) RPCOption {
	return func(h *RPCHandler) {
		if l != nil {
			h.logger = l
		}
	}
}

// NewRPCHandler serves the bindings; a method bound twice is an error.
func NewRPCHandler(bindings []Binding, opts ...RPCOption) (*RPCHandler, error) {
	h := &RPCHandler{methods: make(map[string]Binding, len(bindings)), maxBody: 1 << 20, logger: slog.Default()}
	methods := make([]Method, 0, len(bindings))
	for _, b := range bindings {
		if b.call == nil {
			return nil, fmt.Errorf("solid rpc: binding of %q has no implementation", b.method.Name)
		}
		if _, dup := h.methods[b.method.Name]; dup {
			return nil, fmt.Errorf("solid rpc: method %q is bound twice", b.method.Name)
		}
		h.methods[b.method.Name] = b
		methods = append(methods, b.method)
	}
	if err := validateMethods(methods); err != nil {
		return nil, fmt.Errorf("solid rpc: %w", err)
	}
	for _, opt := range opts {
		opt(h)
	}
	return h, nil
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcWireError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcWireError   `json:"error,omitempty"`
}

func (h *RPCHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.guard != nil {
		if err := h.guard(r); err != nil {
			h.write(w, http.StatusForbidden, rpcResponse{Error: &rpcWireError{Code: CodeForbidden, Message: "cross-site request refused"}})
			return
		}
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, h.maxBody))
	if err != nil {
		h.write(w, http.StatusRequestEntityTooLarge, rpcResponse{Error: &rpcWireError{Code: CodeProtocol, Message: "request body is too large"}})
		return
	}
	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil || req.Method == "" {
		h.write(w, http.StatusBadRequest, rpcResponse{Error: &rpcWireError{Code: CodeProtocol, Message: "not a JSON-RPC request"}})
		return
	}
	binding, ok := h.methods[req.Method]
	if !ok {
		h.write(w, http.StatusOK, rpcResponse{ID: req.ID, Error: &rpcWireError{Code: CodeProtocol, Message: "unknown method"}})
		return
	}
	result, err := binding.call(r.Context(), req.Params)
	if err != nil {
		var rpcErr *RPCError
		if !errors.As(err, &rpcErr) {
			h.logger.ErrorContext(r.Context(), "solid rpc: method failed", "method", req.Method, "error", err)
			rpcErr = NewRPCError(CodeInternal, "internal error", nil)
		}
		status := http.StatusOK
		switch rpcErr.Code {
		case CodeUnauthenticated:
			status = http.StatusUnauthorized
		case CodeForbidden:
			status = http.StatusForbidden
		}
		h.write(w, status, rpcResponse{ID: req.ID, Error: &rpcWireError{Code: rpcErr.Code, Message: rpcErr.Message, Data: rpcErr.Data}})
		return
	}
	h.write(w, http.StatusOK, rpcResponse{ID: req.ID, Result: result})
}

func (h *RPCHandler) write(w http.ResponseWriter, status int, resp rpcResponse) {
	resp.JSONRPC = "2.0"
	if len(resp.ID) == 0 {
		resp.ID = json.RawMessage("null")
	}
	data, err := json.Marshal(resp)
	if err != nil {
		h.logger.Error("solid rpc: encode response", "error", err)
		status = http.StatusInternalServerError
		data = []byte(`{"jsonrpc":"2.0","id":null,"error":{"code":"internal","message":"internal error"}}`)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(append(data, '\n'))
}
