package engine

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// A step applies the effects it was prepared for only while the step is the prepared one and the subject's
// variables the run-as may read are unchanged: a right lost in between is honored by the fresh evaluation.
func TestPreparedEffectsAreUsedOnlyForTheSameStepAndTheSameVariables(t *testing.T) {
	tok := &tokenRow{ID: uuid.New(), StepSeq: 2}
	prepared := []graph.Effect{{Node: "n", Params: map[string]any{"stamp": "prepared"}}}
	fresh := []graph.Effect{{Node: "n", Params: map[string]any{"stamp": "fresh"}}}
	vars := map[string]any{"order": map[string]any{"owner": 7}}
	x := &execution{prepared: &preparedStep{token: tok.ID, seq: 3, node: "n", varsHash: hashVars(vars), PreparedEffect: PreparedEffect{Effects: prepared}}}

	m := &mover{x: x, varsHash: hashVars(vars)}
	assert.Equal(t, prepared, m.preparedEffects(tok, "n", fresh), "the same step and variables: what was prepared is applied")

	m.varsHash = hashVars(map[string]any{"order": map[string]any{"owner": 8}})
	assert.Equal(t, fresh, m.preparedEffects(tok, "n", fresh), "the variables the run-as reads changed: the fresh evaluation decides")
	m.varsHash = hashVars(map[string]any{}) // a subject the run-as can no longer read
	assert.Equal(t, fresh, m.preparedEffects(tok, "n", fresh))

	m.varsHash = hashVars(vars)
	assert.Equal(t, fresh, m.preparedEffects(tok, "other", fresh), "another node")
	later := &tokenRow{ID: tok.ID, StepSeq: 3}
	assert.Equal(t, fresh, m.preparedEffects(later, "n", fresh), "another step")
	assert.Equal(t, fresh, m.preparedEffects(&tokenRow{ID: uuid.New(), StepSeq: 2}, "n", fresh), "another token")
	assert.Empty(t, m.preparedEffects(tok, "n", nil), "a fresh evaluation that rendered no effect (it failed) takes its failure")
	assert.Equal(t, hashVars(vars), hashVars(map[string]any{"order": map[string]any{"owner": 7}}), "the hash is stable")
}
