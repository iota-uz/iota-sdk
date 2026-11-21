package solid

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type echoReq struct {
	Name string `json:"name"`
}
type echoResp struct {
	Hello string `json:"hello"`
}

func rpcHandler(t *testing.T, opts ...RPCOption) *RPCHandler {
	t.Helper()
	h, err := NewRPCHandler([]Binding{
		Bind(Query[echoReq, echoResp]("demo.echo"), func(_ context.Context, r echoReq) (echoResp, error) {
			return echoResp{Hello: r.Name}, nil
		}),
		Bind(Action[echoReq, echoResp]("demo.fail"), func(_ context.Context, r echoReq) (echoResp, error) {
			if r.Name == "typed" {
				return echoResp{}, NewRPCError(CodeForbidden, "no", map[string]string{"code": CodeForbidden})
			}
			return echoResp{}, errors.New("pq: password=hunter2")
		}),
	}, opts...)
	require.NoError(t, err)
	return h
}

func call(h http.Handler, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(body)))
	return rec
}

func TestRPCHandlerDispatchesAndHidesInternalErrors(t *testing.T) {
	t.Parallel()
	h := rpcHandler(t)
	rec := call(h, `{"jsonrpc":"2.0","id":7,"method":"demo.echo","params":{"name":"ann"}}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"jsonrpc":"2.0","id":7,"result":{"hello":"ann"}}`, rec.Body.String())

	rec = call(h, `{"jsonrpc":"2.0","id":8,"method":"demo.fail","params":{"name":"x"}}`)
	require.NotContains(t, rec.Body.String(), "hunter2")
	require.Contains(t, rec.Body.String(), `"code":"internal"`)

	rec = call(h, `{"jsonrpc":"2.0","id":9,"method":"demo.fail","params":{"name":"typed"}}`)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.JSONEq(t, `{"jsonrpc":"2.0","id":9,"error":{"code":"forbidden","message":"no","data":{"code":"forbidden"}}}`, rec.Body.String())

	rec = call(h, `{"jsonrpc":"2.0","id":10,"method":"demo.missing"}`)
	require.Contains(t, rec.Body.String(), `"code":"protocol"`)
	rec = call(h, `not json`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	rec = call(h, `{"jsonrpc":"2.0","id":11,"method":"demo.echo","params":{"name":5}}`)
	require.Contains(t, rec.Body.String(), `"code":"validation"`)
}

func TestRPCHandlerGuardAndDuplicates(t *testing.T) {
	t.Parallel()
	h := rpcHandler(t, WithRPCGuard(func(*http.Request) error { return errors.New("cross-site") }))
	rec := call(h, `{"jsonrpc":"2.0","id":1,"method":"demo.echo"}`)
	require.Equal(t, http.StatusForbidden, rec.Code)

	m := Query[echoReq, echoResp]("same")
	fn := func(context.Context, echoReq) (echoResp, error) { return echoResp{}, nil }
	_, err := NewRPCHandler([]Binding{Bind(m, fn), Bind(m, fn)})
	require.Error(t, err)
}
