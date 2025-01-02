package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"sync"
	"time"

	"github.com/99designs/gqlgen/graphql/errcode"
	"github.com/99designs/gqlgen/graphql/executor"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/gorilla/websocket"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	app application.Application
}

func NewBaseServer(schema graphql.ExecutableSchema) *Handler {
	ex := executor.New(schema)
	srv := NewHandler(ex)
	// for _, schema := range app.GraphSchemas() {
	// 	srv.execs = append(srv.execs, executor.New(schema.Value))
	// }
	return srv
}

func (h MyPOST) Do(w http.ResponseWriter, r *http.Request, exec graphql.GraphExecutor) {
	ctx := r.Context()
	execs := ctx.Value(execsContextKey).([]*executor.Executor)
	writeHeaders(w, h.ResponseHeaders)
	params := pool.Get().(*graphql.RawParams)
	defer func() {
		params.Headers = nil
		params.ReadTime = graphql.TraceTiming{}
		params.Extensions = nil
		params.OperationName = ""
		params.Query = ""
		params.Variables = nil

		pool.Put(params)
	}()
	params.Headers = r.Header
	start := graphql.Now()
	params.ReadTime = graphql.TraceTiming{
		Start: start,
		End:   graphql.Now(),
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		gqlErr := gqlerror.Errorf("could not read request body: %+v", err)
		resp := exec.DispatchError(ctx, gqlerror.List{gqlErr})
		writeJson(w, resp)
		return
	}

	bodyReader := bytes.NewReader(bodyBytes)
	if err := jsonDecode(bodyReader, &params); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		gqlErr := gqlerror.Errorf(
			"json request body could not be decoded: %+v body:%s",
			err,
			string(bodyBytes),
		)
		resp := exec.DispatchError(ctx, gqlerror.List{gqlErr})
		writeJson(w, resp)
		return
	}

	execsLen := len(execs)
	for i, ex := range execs {
		rc, opErr := ex.CreateOperationContext(ctx, params)
		if opErr != nil {
			if i < execsLen && rc.Operation == nil {
				continue
			}
			w.WriteHeader(statusFor(opErr))
			resp := ex.DispatchError(graphql.WithOperationContext(ctx, rc), opErr)
			writeJson(w, resp)
			return
		}

		var responses graphql.ResponseHandler
		responses, ctx = ex.DispatchOperation(ctx, rc)
		writeJson(w, responses(ctx))
		return
	}
}

type MyPOST struct {
	// Map of all headers that are added to graphql response. If not
	// set, only one header: Content-Type: application/json will be set.
	ResponseHeaders map[string][]string
}

var _ graphql.Transport = MyPOST{}

func (h MyPOST) Supports(r *http.Request) bool {
	if r.Header.Get("Upgrade") != "" {
		return false
	}

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return false
	}

	return r.Method == http.MethodPost && mediaType == "application/json"
}

func getRequestBody(r *http.Request) (string, error) {
	if r == nil || r.Body == nil {
		return "", nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", fmt.Errorf("unable to get Request Body %w", err)
	}
	return string(body), nil
}

var pool = sync.Pool{
	New: func() any {
		return &graphql.RawParams{}
	},
}

func writeHeaders(w http.ResponseWriter, headers map[string][]string) {
	if len(headers) == 0 {
		headers = map[string][]string{
			"Content-Type": {"application/json"},
		}
	}

	for key, values := range headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
}

func writeJson(w io.Writer, response *graphql.Response) {
	b, err := json.Marshal(response)
	if err != nil {
		panic(fmt.Errorf("unable to marshal %s: %w", string(response.Data), err))
	}
	w.Write(b)
}

func writeJsonError(w io.Writer, msg string) {
	writeJson(w, &graphql.Response{Errors: gqlerror.List{{Message: msg}}})
}

func writeJsonErrorf(w io.Writer, format string, args ...any) {
	writeJson(w, &graphql.Response{Errors: gqlerror.List{{Message: fmt.Sprintf(format, args...)}}})
}

func writeJsonGraphqlError(w io.Writer, err ...*gqlerror.Error) {
	writeJson(w, &graphql.Response{Errors: err})
}

func jsonDecode(r io.Reader, val any) error {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	return dec.Decode(val)
}
func statusFor(errs gqlerror.List) int {
	switch errcode.GetErrorKind(errs) {
	case errcode.KindProtocol:
		return http.StatusUnprocessableEntity
	default:
		return http.StatusOK
	}
}

const execsContextKey = "execs"

type Handler struct {
	execs      []*executor.Executor
	transports []graphql.Transport
}

func NewHandler(rootExecutor *executor.Executor) *Handler {
	server := &Handler{}
	server.execs = append(server.execs, rootExecutor)

	server.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
		Upgrader: websocket.Upgrader{
			// TODO: Add origin check
			CheckOrigin: func(_ *http.Request) bool {
				return true
			},
		},
	})
	server.AddTransport(transport.Options{})
	server.AddTransport(transport.GET{})
	server.AddTransport(MyPOST{})
	server.AddTransport(transport.MultipartForm{})

	// TODO: make LRU work
	// srv.SetQueryCache(lru.New(1000))

	server.Use(map[*executor.Executor]graphql.HandlerExtension{
		rootExecutor: extension.Introspection{},
	})
	return server
}

func (s *Handler) AddExecutor(execs ...*executor.Executor) {
	s.execs = append(s.execs, execs...)
}

func (s *Handler) AddTransport(transport graphql.Transport) {
	s.transports = append(s.transports, transport)
}

func (s *Handler) SetErrorPresenter(funcs map[*executor.Executor]graphql.ErrorPresenterFunc) {
	for k, f := range funcs {
		k.SetErrorPresenter(f)
	}
}

func (s *Handler) SetRecoverFunc(funcs map[*executor.Executor]graphql.RecoverFunc) {
	for k, f := range funcs {
		k.SetRecoverFunc(f)
	}
}

func (s *Handler) SetQueryCache(caches map[*executor.Executor]graphql.Cache[*ast.QueryDocument]) {
	for k, c := range caches {
		k.SetQueryCache(c)
	}
}

func (s *Handler) SetParserTokenLimit(limits map[*executor.Executor]int) {
	for k, l := range limits {
		k.SetParserTokenLimit(l)
	}
}

func (s *Handler) SetDisableSuggestion(values map[*executor.Executor]bool) {
	// for k, b := range values {
	// 	k.SetDisableSuggestion(b)
	// }
}

func (s *Handler) Use(extensions map[*executor.Executor]graphql.HandlerExtension) {
	for k, ex := range extensions {
		k.Use(ex)
	}
}

// AroundFields is a convenience method for creating an extension that only implements field middleware
func (s *Handler) AroundFields(funcs map[*executor.Executor]graphql.FieldMiddleware) {
	for k, f := range funcs {
		k.AroundFields(f)
	}
}

// AroundRootFields is a convenience method for creating an extension that only implements field middleware
func (s *Handler) AroundRootFields(funcs map[*executor.Executor]graphql.RootFieldMiddleware) {
	for k, f := range funcs {
		k.AroundRootFields(f)
	}
}

// AroundOperations is a convenience method for creating an extension that only implements operation middleware
func (s *Handler) AroundOperations(funcs map[*executor.Executor]graphql.OperationMiddleware) {
	for k, f := range funcs {
		k.AroundOperations(f)
	}
}

// AroundResponses is a convenience method for creating an extension that only implements response middleware
func (s *Handler) AroundResponses(funcs map[*executor.Executor]graphql.ResponseMiddleware) {
	for k, f := range funcs {
		k.AroundResponses(f)
	}
}

func (s *Handler) getTransport(r *http.Request) graphql.Transport {
	for _, t := range s.transports {
		if t.Supports(r) {
			return t
		}
	}
	return nil
}

func (s *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			err := s.execs[0].PresentRecoveredError(r.Context(), err)
			gqlErr, _ := err.(*gqlerror.Error)
			resp := &graphql.Response{Errors: []*gqlerror.Error{gqlErr}}
			b, _ := json.Marshal(resp)
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write(b)
		}
	}()

	r = r.WithContext(graphql.StartOperationTrace(r.Context()))

	transport := s.getTransport(r)
	if transport == nil {
		sendErrorf(w, http.StatusBadRequest, "transport not supported")
		return
	}

	r = r.WithContext(context.WithValue(r.Context(), execsContextKey, s.execs))

	transport.Do(w, r, s.execs[0])
}

func sendError(w http.ResponseWriter, code int, errors ...*gqlerror.Error) {
	w.WriteHeader(code)
	b, err := json.Marshal(&graphql.Response{Errors: errors})
	if err != nil {
		panic(err)
	}
	_, _ = w.Write(b)
}

func sendErrorf(w http.ResponseWriter, code int, format string, args ...any) {
	sendError(w, code, &gqlerror.Error{Message: fmt.Sprintf(format, args...)})
}

type OperationFunc func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler

func (r OperationFunc) ExtensionName() string {
	return "InlineOperationFunc"
}

func (r OperationFunc) Validate(schema graphql.ExecutableSchema) error {
	if r == nil {
		return errors.New("OperationFunc can not be nil")
	}
	return nil
}

func (r OperationFunc) InterceptOperation(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
	return r(ctx, next)
}

type ResponseFunc func(ctx context.Context, next graphql.ResponseHandler) *graphql.Response

func (r ResponseFunc) ExtensionName() string {
	return "InlineResponseFunc"
}

func (r ResponseFunc) Validate(schema graphql.ExecutableSchema) error {
	if r == nil {
		return errors.New("ResponseFunc can not be nil")
	}
	return nil
}

func (r ResponseFunc) InterceptResponse(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
	return r(ctx, next)
}

type FieldFunc func(ctx context.Context, next graphql.Resolver) (any, error)

func (f FieldFunc) ExtensionName() string {
	return "InlineFieldFunc"
}

func (f FieldFunc) Validate(schema graphql.ExecutableSchema) error {
	if f == nil {
		return errors.New("FieldFunc can not be nil")
	}
	return nil
}

func (f FieldFunc) InterceptField(ctx context.Context, next graphql.Resolver) (any, error) {
	return f(ctx, next)
}
