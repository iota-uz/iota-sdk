package backtest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
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
// its fields at every depth, and each item of a stored list as it is (a loader may filter items by
// rights: an item at the same place proves nothing); a subject that cannot be loaded allows no snapshot.
func TestSubjectOKChecksNestedFields(t *testing.T) {
	stub := subjectsStub{
		"o1": `{"order":{"name":"B","lines":[{"sku":"x","qty":2},{"sku":"y","qty":1}]},"total":5}`,
	}
	r := &reader{s: &Svc{cfg: Config{Subjects: stub}}, subjects: map[engine.SubjectRef]any{}}
	ref := func(key string) engine.SubjectRef { return engine.SubjectRef{Entity: "order", Key: key} }
	ctx := context.Background()
	for _, c := range []struct {
		name, key, snapshot string
		ok                  bool
	}{
		{"the same fields, other values", "o1", `{"order":{"name":"A"},"total":4}`, true},
		{"fewer fields", "o1", `{"order":{"name":"A"}}`, true},
		{"the items of a list, unchanged", "o1", `{"order":{"lines":[{"sku":"x","qty":2}]}}`, true},
		{"fewer items of a list", "o1", `{"order":{"lines":[{"sku":"y","qty":1}]}}`, true},
		{"a restricted nested field", "o1", `{"order":{"name":"A","secret":"s"},"total":4}`, false},
		{"a restricted field of a list's item", "o1", `{"order":{"lines":[{"sku":"x","qty":2,"cost":1}]}}`, false},
		{"two equal items where one is now", "o1", `{"order":{"lines":[{"sku":"y","qty":1},{"sku":"y","qty":1}]}}`, false},
		{"an item replaced by another at its place", "o1", `{"order":{"lines":[{"sku":"z","qty":2},{"sku":"y","qty":1}]}}`, false},
		{"an object where a scalar is now", "o1", `{"total":{"amount":5}}`, false},
		{"a subject not visible now", "o2", `{"order":{"name":"A"}}`, false},
		{"no snapshot", "o2", `null`, true},
	} {
		assert.Equal(t, c.ok, r.subjectOK(ctx, ref(c.key), json.RawMessage(c.snapshot)), c.name)
	}
	noLoader := &reader{s: &Svc{}, subjects: map[engine.SubjectRef]any{}}
	assert.False(t, noLoader.subjectOK(ctx, ref("o1"), json.RawMessage(`{"total":1}`)), "without a loader")
}

// A source's own key never meets a key derived from content; distinct keys stay distinct.
func TestSourceKeysStayApartFromContentKeys(t *testing.T) {
	assert.Equal(t, "<1@x>", sourceKey("<1@x>"))
	assert.Empty(t, sourceKey(""))
	derived := contentKey(Event{Kind: "k"}, []byte(`{}`))
	assert.NotEqual(t, derived, sourceKey(derived))
	assert.NotEqual(t, sourceKey("key:"+derived), sourceKey(derived))
}

// A source's refusal of the request (a filter of an import) reaches the author; other refusals do not.
func TestPublicErrorShowsInvalidRequests(t *testing.T) {
	bad := &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "the filter is an object with from and subject"}
	assert.Equal(t, bad.Message, publicError(fmt.Errorf("import: %w", bad)))
	assert.Equal(t, "the job could not be completed", publicError(&catalog.CallError{Code: catalog.CallUnauthorized, Message: "token for x expired"}))
}
