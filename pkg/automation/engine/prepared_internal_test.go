package engine

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
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

// A preparation that no longer fits what the step reads is not handed to the node: the step prepares again, a
// few times at most, and then the step is retried later instead of being applied.
func TestStalePreparationIsNotHandedToTheNode(t *testing.T) {
	tok := &tokenRow{ID: uuid.New(), StepSeq: 2, NodeID: "n"}
	vars := map[string]any{"order": map[string]any{"owner": 7}}
	prep := &preparedStep{token: tok.ID, seq: 3, node: "n", varsHash: hashVars(vars), attempts: 1, PreparedEffect: PreparedEffect{Value: "checked"}}
	x := &execution{prepared: prep}
	m := &mover{x: x, varsHash: hashVars(vars)}

	got := m.preparedFor(tok, 3)
	require.NotNil(t, got)
	assert.Equal(t, "checked", got.Value, "the variables are what they were: the preparation fits")

	m.varsHash = hashVars(map[string]any{"order": map[string]any{"owner": 8}})
	assert.Nil(t, m.preparedFor(tok, 3), "the variables changed: the old preparation is not handed over, a new one is asked for")

	prep.attempts = maxPrepareAttempts
	got = m.preparedFor(tok, 3)
	require.NotNil(t, got)
	assert.True(t, got.Exhausted, "after the last attempt no preparation fits")
	assert.Nil(t, got.Value)

	assert.Nil(t, m.preparedFor(tok, 4), "another step")
}

// Once the preparations are exhausted the effect is not applied, whatever the node would have done without the
// check it asked for: the step fails as a platform error and the engine retries it later.
func TestExhaustedPreparationRetriesTheStepInsteadOfApplyingTheEffect(t *testing.T) {
	node := graph.Node{ID: "n", Type: "t"}
	applied := false
	en := &preparingEffect{applied: &applied}
	_, err := applyOne(context.Background(), nil, StepContext{RunID: uuid.New(), Prepared: &PreparedEffect{Exhausted: true}}, node, graph.NodeSpec{}, en, graph.Effect{})
	require.Error(t, err)
	assert.True(t, host.IsInfrastructure(err), "a retry, not the node's failure")
	assert.False(t, applied, "nothing was applied")

	_, err = applyOne(context.Background(), nil, StepContext{RunID: uuid.New(), Prepared: &PreparedEffect{Value: "ok"}}, node, graph.NodeSpec{}, en, graph.Effect{})
	require.NoError(t, err)
	assert.True(t, applied)
}

// preparingEffect is an EffectNode that asks for its preparation.
type preparingEffect struct{ applied *bool }

func (p *preparingEffect) PrepareEffect(context.Context, StepContext, graph.Node, graph.Effect) (any, error) {
	return nil, nil
}

func (p *preparingEffect) ApplyEffect(context.Context, pgx.Tx, StepContext, graph.Node, graph.Effect) (map[string]any, error) {
	*p.applied = true
	return nil, nil
}
