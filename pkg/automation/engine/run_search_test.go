package engine_test

// The runs of a graph are found by their subject's name (RunFilter.SubjectQuery): the host's
// SubjectSearcher finds the subjects with the viewer's rights, the key prefix still matches.

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// namedOrders is a SubjectLoader with a SubjectSearcher over acme.order names.
type namedOrders struct {
	names map[string]string // key → name
	mu    sync.Mutex
	asked []string // entity/user of each search
}

func (n *namedOrders) Load(context.Context, spi.Subject, engine.SubjectRef) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

func (n *namedOrders) Search(_ context.Context, as spi.Subject, entity, query string, limit int) ([]engine.SubjectOption, error) {
	n.mu.Lock()
	n.asked = append(n.asked, entity)
	n.mu.Unlock()
	if as.UserID != 1 {
		return nil, nil // another user may see none of them
	}
	out := []engine.SubjectOption{}
	if entity != "acme.order" {
		return out, nil
	}
	for key, name := range n.names {
		if strings.Contains(strings.ToLower(name), strings.ToLower(query)) && len(out) < limit {
			out = append(out, engine.SubjectOption{Subject: engine.SubjectRef{Entity: entity, Key: key}, Label: name})
		}
	}
	return out, nil
}

func TestRunsAreFoundBySubjectName(t *testing.T) {
	e := newEnv(t)
	orders := &namedOrders{names: map[string]string{"o-1": "Ivanov LLC", "o-2": "Petrov & Sons"}}
	p := e.proc(func(c *engine.Config) { c.Subjects = orders })
	a := e.human()
	e.create(p, a, noteGraph("x"))
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	e.publish(e.tenant, orderCreated, "o-2", order{Amount: 1}, 0)
	p.until(func() bool { return len(e.runs(p, a, engine.RunFilter{})) == 2 }, "both runs start")

	keys := func(a engine.Actor, q string) []string {
		var out []string
		for _, r := range e.runs(p, a, engine.RunFilter{SubjectQuery: q}) {
			out = append(out, r.Subject.Key)
		}
		return out
	}
	assert.Equal(t, []string{"o-2"}, keys(a, "petrov"), "by name, case-insensitively")
	assert.Equal(t, []string{"o-1"}, keys(a, "  Ivanov "), "by name, trimmed")
	assert.Equal(t, []string{"o-1"}, keys(a, "O-1"), "by key prefix")
	assert.Empty(t, keys(a, "nobody"))
	assert.Empty(t, keys(a, "%"), "LIKE wildcards are literal")

	other := e.actor(e.tenant, 2, engine.OriginHuman)
	assert.Empty(t, keys(other, "petrov"), "a name matches only subjects the viewer may see")
	assert.Equal(t, []string{"o-2"}, keys(other, "o-2"), "the key still matches")

	orders.mu.Lock()
	defer orders.mu.Unlock()
	require.NotEmpty(t, orders.asked)
	for _, entity := range orders.asked {
		assert.Equal(t, "acme.order", entity, "only entities the tenant's runs are about are searched")
	}
}
