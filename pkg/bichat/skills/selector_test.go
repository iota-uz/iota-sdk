package skills

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelector_MentionOverridePriority(t *testing.T) {
	t.Parallel()

	catalog := testCatalog()
	selector := NewSelector(catalog)

	result, err := selector.Select(context.Background(), SelectionRequest{
		Message: "please use @finance/month-end to fix this dashboard",
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.Selected)
	assert.Equal(t, "finance/month-end", result.Selected[0].Skill.Slug)
	assert.Equal(t, 1, result.Selected[0].Priority)
	require.NotEmpty(t, result.Reference)
	assert.Contains(t, result.Reference, "@finance/month-end")
}

func TestSelector_AutoRanking(t *testing.T) {
	t.Parallel()

	catalog := testCatalog()
	selector := NewSelector(catalog)

	result, err := selector.Select(context.Background(), SelectionRequest{
		Message: "I have sql error and missing column in query",
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.Selected)
	assert.Equal(t, "analytics/sql-debug", result.Selected[0].Skill.Slug)
}

func TestSelector_UnknownMentionIgnored(t *testing.T) {
	t.Parallel()

	catalog := testCatalog()
	selector := NewSelector(catalog)

	result, err := selector.Select(context.Background(), SelectionRequest{
		Message: "@not/exist explain kpi mismatch",
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.Selected)
	assert.NotContains(t, result.Mentioned, "not/exist")
	assert.Equal(t, "analytics/kpi-review", result.Selected[0].Skill.Slug)
}

func TestSelector_TopNCap(t *testing.T) {
	t.Parallel()

	catalog := testCatalog()
	selector := NewSelector(catalog, WithSelectionLimit(1))

	result, err := selector.Select(context.Background(), SelectionRequest{
		Message: "sql error kpi validation month close",
	})
	require.NoError(t, err)
	require.Len(t, result.Selected, 1)
}

func TestSelector_MaxCharsDeterministicExclusion(t *testing.T) {
	t.Parallel()

	catalog := testCatalog()
	selector := NewSelector(catalog,
		WithSelectionLimit(3),
		WithMaxChars(220),
	)

	result, err := selector.Select(context.Background(), SelectionRequest{
		Message: "@analytics/sql-debug @analytics/kpi-review @finance/month-end",
	})
	require.NoError(t, err)

	assert.NotEmpty(t, result.Selected)
	require.NotEmpty(t, result.Reference)
	assert.LessOrEqual(t, len(result.Reference), 220)
	assert.Contains(t, result.Reference, "@analytics/kpi-review")
	assert.NotContains(t, result.Reference, "@finance/month-end")
}

func testCatalog() *Catalog {
	skills := []Skill{
		{
			Slug:       "analytics/sql-debug",
			ParentSlug: "analytics",
			Metadata: SkillMetadata{
				Name:        "SQL Debugging",
				Description: "Recover quickly from SQL query failures",
				WhenToUse:   []string{"sql error", "missing column"},
				Tags:        []string{"sql", "debugging"},
			},
			Body: "Inspect schema, fix query, retry safely.",
		},
		{
			Slug:       "analytics/kpi-review",
			ParentSlug: "analytics",
			Metadata: SkillMetadata{
				Name:        "KPI Review",
				Description: "Validate KPI definitions and calculations",
				WhenToUse:   []string{"kpi mismatch", "metric validation"},
				Tags:        []string{"analytics", "metrics"},
			},
			Body: "Check grain, aggregation, and business definitions.",
		},
		{
			Slug:       "finance/month-end",
			ParentSlug: "finance",
			Metadata: SkillMetadata{
				Name:        "Month End",
				Description: "Close month books and verify reconciliations",
				WhenToUse:   []string{"month close", "reconciliation"},
				Tags:        []string{"finance", "closing"},
			},
			Body: "Run period checks and reconcile balances before close.",
		},
	}
	bySlug := map[string]Skill{}
	children := map[string][]string{}
	for _, skill := range skills {
		bySlug[skill.Slug] = skill
		children[skill.ParentSlug] = append(children[skill.ParentSlug], skill.Slug)
	}
	return &Catalog{
		Root:     "/tmp/skills",
		Skills:   skills,
		BySlug:   bySlug,
		Children: children,
	}
}
