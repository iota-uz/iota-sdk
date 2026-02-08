package testharness

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseJudgeVerdict_Strict(t *testing.T) {
	t.Parallel()

	_, err := parseJudgeVerdict([]byte(`{"passed":true,"score":0.9,"reason":"ok","missed_facts":[],"incorrect_claims":[],"extra":1}`))
	require.Error(t, err)

	_, err = parseJudgeVerdict([]byte(`{"passed":true,"score":1.2,"reason":"ok","missed_facts":[],"incorrect_claims":[]}`))
	require.Error(t, err)

	_, err = parseJudgeVerdict([]byte(`{"passed":true,"score":0.9,"reason":"","missed_facts":[],"incorrect_claims":[]}`))
	require.Error(t, err)

	v, err := parseJudgeVerdict([]byte(`{"passed":true,"score":0.95,"reason":"oracle aligned","missed_facts":[],"incorrect_claims":[]}`))
	require.NoError(t, err)
	require.True(t, v.Passed)
	require.InEpsilon(t, 0.95, v.Score, 1e-9)
}

func TestParseJudgeVerdict_ExtractsWrappedJSON(t *testing.T) {
	t.Parallel()

	v, err := parseJudgeVerdict([]byte("```json\n{\"passed\":false,\"score\":0.4,\"reason\":\"missing key fact\",\"missed_facts\":[\"x\"],\"incorrect_claims\":[]}\n```"))
	require.NoError(t, err)
	require.False(t, v.Passed)
	require.InEpsilon(t, 0.4, v.Score, 1e-9)
	require.Equal(t, "missing key fact", v.Reason)
}

func TestBuildJudgeUserPrompt_WithOracleFacts(t *testing.T) {
	t.Parallel()

	prompt := buildJudgeUserPrompt(JudgeTurnInput{
		UserPrompt:  "What is Q1 revenue?",
		FinalAnswer: "Q1 revenue is 600000.",
		OracleFacts: []OracleFact{
			{
				Key:           "analytics_baseline_v1.q1_total_income_minor",
				ExpectedValue: "600000",
				ValueType:     "number",
				Tolerance:     0.1,
				Normalization: "currency_minor_units",
			},
		},
	})

	require.Contains(t, prompt, "Oracle facts (authoritative)")
	require.Contains(t, prompt, "analytics_baseline_v1.q1_total_income_minor")
	require.Contains(t, prompt, "normalization=currency_minor_units")
}

func TestBuildJudgeUsage_EstimatesCost(t *testing.T) {
	t.Parallel()

	usage := buildJudgeUsage("gpt-5-mini", 1_000, 500, 1_500)
	require.NotNil(t, usage)
	require.Equal(t, 1_000, usage.PromptTokens)
	require.Equal(t, 500, usage.CompletionTokens)
	require.Equal(t, 1_500, usage.TotalTokens)
	require.Equal(t, "USD", usage.Currency)
	require.True(t, usage.EstimatedCost)
	require.Greater(t, usage.Cost, 0.0)
}
