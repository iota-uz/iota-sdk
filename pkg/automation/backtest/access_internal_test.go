package backtest

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

type subjectsStub map[string]string

func (s subjectsStub) Load(_ context.Context, _ spi.Subject, ref engine.SubjectRef) (json.RawMessage, error) {
	raw, ok := s[ref.Key]
	if !ok {
		return nil, errors.New("not visible")
	}
	return json.RawMessage(raw), nil
}

// A stored snapshot is the reader's only when the record as the reader may read it now holds each of
// its fields at every depth; a subject that cannot be loaded allows no snapshot.
func TestSubjectOKChecksNestedFields(t *testing.T) {
	stub := subjectsStub{
		"o1": `{"order":{"name":"B","lines":[{"sku":"x","qty":2}]},"total":5}`,
	}
	r := &reader{s: &Svc{cfg: Config{Subjects: stub}}, subjects: map[engine.SubjectRef]any{}}
	ref := func(key string) engine.SubjectRef { return engine.SubjectRef{Entity: "order", Key: key} }
	ctx := context.Background()
	for _, c := range []struct {
		name, key, snapshot string
		ok                  bool
	}{
		{"the same fields, other values", "o1", `{"order":{"name":"A","lines":[{"sku":"y","qty":1}]},"total":4}`, true},
		{"fewer fields", "o1", `{"order":{"name":"A"}}`, true},
		{"a restricted nested field", "o1", `{"order":{"name":"A","secret":"s"},"total":4}`, false},
		{"a restricted field of a list's element", "o1", `{"order":{"lines":[{"sku":"x","cost":1}]}}`, false},
		{"a list of another length", "o1", `{"order":{"lines":[{"sku":"x"},{"sku":"y"}]}}`, false},
		{"an object where a scalar is now", "o1", `{"total":{"amount":5}}`, false},
		{"a subject not visible now", "o2", `{"order":{"name":"A"}}`, false},
		{"no snapshot", "o2", `null`, true},
	} {
		assert.Equal(t, c.ok, r.subjectOK(ctx, ref(c.key), json.RawMessage(c.snapshot)), c.name)
	}
	noLoader := &reader{s: &Svc{}, subjects: map[engine.SubjectRef]any{}}
	assert.False(t, noLoader.subjectOK(ctx, ref("o1"), json.RawMessage(`{"total":1}`)), "without a loader")
}
