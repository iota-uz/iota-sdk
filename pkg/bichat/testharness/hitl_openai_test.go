package testharness

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseHITLAnswers_StrictAndFallback(t *testing.T) {
	t.Parallel()

	questions := []HITLQuestion{
		{
			ID:   "metric",
			Text: "Choose metric",
			Type: "single_choice",
			Options: []HITLQuestionOption{
				{ID: "revenue", Label: "Revenue"},
				{ID: "profit", Label: "Profit"},
			},
		},
		{
			ID:   "period",
			Text: "Choose period",
			Type: "single_choice",
			Options: []HITLQuestionOption{
				{ID: "q1", Label: "Q1"},
				{ID: "q2", Label: "Q2"},
			},
		},
	}

	_, err := parseHITLAnswers([]byte(`{"answers":{"metric":"revenue"},"extra":"x"}`), questions)
	require.Error(t, err)

	answers, err := parseHITLAnswers([]byte(`{"answers":{"metric":"unknown","period":"q2"}}`), questions)
	require.NoError(t, err)
	require.Equal(t, "revenue", answers["metric"])
	require.Equal(t, "q2", answers["period"])

	answers, err = parseHITLAnswers([]byte(`{"answers":{}}`), questions)
	require.NoError(t, err)
	require.Equal(t, "revenue", answers["metric"])
	require.Equal(t, "q1", answers["period"])
}

func TestParseHITLAnswers_ExtractsWrappedJSON(t *testing.T) {
	t.Parallel()

	questions := []HITLQuestion{
		{
			ID:   "dimension",
			Text: "Choose dimension",
			Type: "single_choice",
			Options: []HITLQuestionOption{
				{ID: "region", Label: "Region"},
			},
		},
	}

	answers, err := parseHITLAnswers([]byte("answer:\n```json\n{\"answers\":{\"dimension\":\"region\"}}\n```"), questions)
	require.NoError(t, err)
	require.Equal(t, "region", answers["dimension"])
}
