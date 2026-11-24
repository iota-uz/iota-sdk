package engine_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
)

func TestDryRunEventCountUsesTheBatchRuleOfTheParser(t *testing.T) {
	raw := func(s string) json.RawMessage { return json.RawMessage(s) }
	assert.Equal(t, 3, engine.DryRunEventCount([]json.RawMessage{raw(`{"events": [{}, {}]}`), raw(`{"kind": "a"}`)}))
	// An event with its own payload field events is one event.
	assert.Equal(t, 1, engine.DryRunEventCount([]json.RawMessage{raw(`{"kind": "a", "events": [1, 2, 3, 4]}`)}))
	assert.Equal(t, 2, engine.DryRunEventCount([]json.RawMessage{raw(`{"events": null}`), raw(`{"timeout": true}`)}))
}
