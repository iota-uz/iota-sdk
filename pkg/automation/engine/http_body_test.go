package engine_test

import (
	"context"
	"errors"
	nethttp "net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/builtin"
	sandbox "github.com/iota-uz/iota-sdk/pkg/automation/code"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
)

// bodies records the bodies of the requests it answers, by path.
type bodies struct {
	mu  sync.Mutex
	got map[string]string
}

func (b *bodies) Fetch(_ context.Context, req sandbox.FetchRequest) (*sandbox.FetchResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.got[req.URL] = string(req.Body)
	return &sandbox.FetchResponse{Status: 200, Header: nethttp.Header{"Content-Type": {"application/json"}}, Body: []byte(`{}`), URL: req.URL}, nil
}

// The step tells the http node how it evaluated body.value: a CEL string whose text is a JSON object is
// sent as a JSON string, a rendered template of one as that object.
func TestHTTPBodyKeepsTheTypeOfItsCELString(t *testing.T) {
	e := newEnv(t)
	rec := &bodies{got: map[string]string{}}
	require.NoError(t, builtin.Register(e.reg, builtin.Deps{Fetcher: rec,
		Runs: func() (engine.RunStarter, error) { return nil, errors.New("no runs") }}))
	p := e.proc()
	a := e.human()
	e.create(p, a, graphJSON("Bodies", `{"id": "created", "event": "acme.order.created.v1"}`,
		`{"id": "cel", "type": "http", "params": {"method": "POST", "url": "https://api.example.com/cel",
		   "body": {"kind": "json", "value": "'{\"amount\": ' + string(event.amount) + '}'"}}},
		 {"id": "tpl", "type": "http", "params": {"method": "POST", "url": "https://api.example.com/tpl",
		   "body": {"kind": "json", "value": "{\"amount\": {{ event.amount }}}"}}},
		 {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "created", "outcome": "fired"}, "to": "cel"},
		 {"from": {"node": "cel", "outcome": "success"}, "to": "tpl"}, {"from": {"node": "cel", "outcome": "failed"}, "to": "fin"},
		 {"from": {"node": "tpl", "outcome": "success"}, "to": "fin"}, {"from": {"node": "tpl", "outcome": "failed"}, "to": "fin"}`))
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 9}, 0)
	run := onlyRun(t, e, p, a)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends")
	rec.mu.Lock()
	defer rec.mu.Unlock()
	assert.Equal(t, `"{\"amount\": 9}"`, rec.got["https://api.example.com/cel"], "a CEL string is a JSON string")
	assert.Equal(t, `{"amount": 9}`, rec.got["https://api.example.com/tpl"], "a rendered template is the JSON it reads")
}
