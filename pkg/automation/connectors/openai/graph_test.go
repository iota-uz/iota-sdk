package openai_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/internal/aitest"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/openai"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

func codes(errs []graph.GraphError) []string {
	out := make([]string, len(errs))
	for i, e := range errs {
		out[i] = e.Path + ":" + e.Code
	}
	return out
}

func llmGraph(params string) string {
	return aitest.Graph("ai.llm", params, []string{"done", "failed"})
}

func TestGraphValidation(t *testing.T) {
	v := aitest.View(t, openai.New())
	require.Contains(t, v.Catalog.Nodes, "ai.llm")

	assert.Empty(t, aitest.Validate(t, v, llmGraph(`{"prompt":"Summarize {{ event.text }}"}`)))
	assert.Empty(t, aitest.Validate(t, v, llmGraph(`{"prompt":"x","model":"gpt-4o-2024-08-06"}`)))

	// An alias is refused on save.
	errs := aitest.Validate(t, v, llmGraph(`{"prompt":"x","model":"gpt-4o-latest"}`))
	require.Len(t, errs, 1)
	assert.Equal(t, []string{"nodes[0].params.model:params"}, codes(errs))
	assert.Contains(t, errs[0].Message, "alias")

	// A template that reads what the event does not have is refused.
	assert.NotEmpty(t, aitest.Validate(t, v, llmGraph(`{"prompt":"{{ event.nope }}"}`)))
	// No secrets in a prompt.
	assert.NotEmpty(t, aitest.Validate(t, v, llmGraph(`{"prompt":"{{ secrets.key }}"}`)))
}

func TestGraphImages(t *testing.T) {
	v := aitest.View(t, openai.New())
	assert.Empty(t, aitest.Validate(t, v, llmGraph(`{"prompt":"what is it","model":"gpt-4o-2024-08-06","images":"event.upload_ids"}`)))

	errs := aitest.Validate(t, v, llmGraph(`{"prompt":"what is it","model":"gpt-3.5-turbo-0125","images":"event.upload_ids"}`))
	require.Len(t, errs, 1)
	assert.Equal(t, "nodes[0].params.images", errs[0].Path)
	assert.Equal(t, "model_no_vision", errs[0].Code)

	// The images expression is a list.
	assert.NotEmpty(t, aitest.Validate(t, v, llmGraph(`{"prompt":"x","model":"gpt-4o-2024-08-06","images":"event.title"}`)))
	assert.NotEmpty(t, aitest.Validate(t, v, llmGraph(`{"prompt":"x","model":"gpt-4o-2024-08-06","images":"event.nope"}`)))
}

func TestGraphJSONOutputIsTypedBySchema(t *testing.T) {
	v := aitest.View(t, openai.New())
	params := `{"prompt":"x","format":"json","schema":` + answerSchema + `}`
	next := func(expression string) string {
		return aitest.Doc(
			`{"id":"n1","type":"ai.llm","params":`+params+`},{"id":"check","type":"branch","params":{"expression":"`+expression+`"}}`,
			`{"from":{"node":"t","outcome":"fired"},"to":"n1"},`+
				`{"from":{"node":"n1","outcome":"done"},"to":"check"},{"from":{"node":"n1","outcome":"failed"},"to":"ignore"},`+
				`{"from":{"node":"check","outcome":"true"},"to":"ignore"},{"from":{"node":"check","outcome":"false"},"to":"ignore"}`)
	}
	assert.Empty(t, aitest.Validate(t, v, next(`nodes.n1.output.json.score > 3`)))
	assert.Empty(t, aitest.Validate(t, v, next(`nodes.n1.output.usage.input_tokens > 100`)))
	assert.Empty(t, aitest.Validate(t, v, next(`nodes.n1.output.model == 'gpt-4o'`)))
	assert.Empty(t, aitest.Validate(t, v, next(`nodes.n1.output.latency_ms < 5000`)))
	// The fields come from the schema: a field it does not have, or a field of the wrong type, is refused.
	assert.NotEmpty(t, aitest.Validate(t, v, next(`nodes.n1.output.json.missing > 3`)))
	assert.NotEmpty(t, aitest.Validate(t, v, next(`nodes.n1.output.json.summary > 3`)))
	assert.NotEmpty(t, aitest.Validate(t, v, next(`nodes.n1.output.usage.nope > 3`)))

	// A schema the node cannot type is refused on save.
	errs := aitest.Validate(t, v, llmGraph(`{"prompt":"x","format":"json","schema":{"type":"object","properties":{"a":{"type":"string","pattern":"x"}}}}`))
	require.NotEmpty(t, errs)
	assert.True(t, strings.HasPrefix(errs[0].Path, "nodes[0].params"), errs[0].Path)
}

func TestCatalogViewCarriesTheNodeSpec(t *testing.T) {
	v := aitest.View(t, openai.New())
	cv := v.JSON(nil)
	var found bool
	for _, n := range cv.Nodes {
		if n.Type != "ai.llm" {
			continue
		}
		found = true
		assert.Equal(t, []string{"openai"}, n.Requires)
		assert.Equal(t, graph.DryRunExecute, n.DryRun)
		require.NotNil(t, n.OutputFrom)
		assert.Equal(t, "schema", n.OutputFrom.Param)
		assert.Equal(t, "json", n.OutputFrom.Field)
		assert.NotEmpty(t, n.Output)
	}
	assert.True(t, found)
}
