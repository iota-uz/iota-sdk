package agents_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	bichatagents "github.com/iota-uz/iota-sdk/modules/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
)

// ExampleNewDefaultBIAgent demonstrates basic usage of the default BI agent.
func ExampleNewDefaultBIAgent() {
	// Create a query executor (in real usage, this would use your database pool)
	executor := &mockQueryExecutor{}

	// Create the default BI agent with core SQL tools
	agent, err := bichatagents.NewDefaultBIAgent(executor)
	if err != nil {
		panic(err)
	}

	// Access agent metadata
	metadata := agent.Metadata()
	fmt.Println("Agent name:", metadata.Name)
	fmt.Println("Agent model:", metadata.Model)

	// List available tools
	agentTools := agent.Tools()
	fmt.Printf("Number of tools: %d\n", len(agentTools))

	// Output:
	// Agent name: bi_agent
	// Agent model: gpt-5.2
	// Number of tools: 9
}

// ExampleNewDefaultBIAgent_withOptions demonstrates using optional tools.
func ExampleNewDefaultBIAgent_withOptions() {
	executor := &mockQueryExecutor{}
	kbSearcher := &mockKBSearcher{}

	// Create agent with optional KB search
	agent, err := bichatagents.NewDefaultBIAgent(
		executor,
		bichatagents.WithKBSearcher(kbSearcher),
		bichatagents.WithModel("gpt-5-mini"),
	)
	if err != nil {
		panic(err)
	}

	metadata := agent.Metadata()
	agentTools := agent.Tools()

	fmt.Println("Model:", metadata.Model)
	fmt.Printf("Tools count: %d\n", len(agentTools))

	// Output:
	// Model: gpt-5-mini
	// Tools count: 10
}

// ExampleNewDefaultBIAgent_withInsightPrompting demonstrates insight-focused response prompting.
func ExampleNewDefaultBIAgent_withInsightPrompting() {
	executor := &mockQueryExecutor{}

	// Create agent with "standard" insight depth
	// This configures the agent to provide structured analysis with key findings,
	// trends, anomalies, and comparisons after presenting data
	agent, err := bichatagents.NewDefaultBIAgent(
		executor,
		bichatagents.WithInsightPrompting("standard"),
		bichatagents.WithModel("gpt-5.2"),
	)
	if err != nil {
		panic(err)
	}

	metadata := agent.Metadata()
	fmt.Println("Model:", metadata.Model)
	fmt.Println("Agent configured for insight-focused responses")

	// The agent's system prompt now includes instructions to provide:
	// - KEY FINDINGS: 2-3 most important observations
	// - TRENDS: Notable patterns over time or across categories
	// - ANOMALIES: Unexpected values or outliers worth investigating
	// - COMPARISONS: How results compare to baselines, targets, or prior periods

	// Output:
	// Model: gpt-5.2
	// Agent configured for insight-focused responses
}

// Mock implementations for examples

type mockQueryExecutor struct{}

func (m *mockQueryExecutor) ExecuteQuery(ctx context.Context, sql string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error) {
	return &bichatsql.QueryResult{
		Columns:  []string{"id", "name"},
		Rows:     [][]any{{1, "test"}},
		RowCount: 1,
	}, nil
}

type mockKBSearcher struct{}

func (m *mockKBSearcher) Search(ctx context.Context, query string, opts kb.SearchOptions) ([]kb.SearchResult, error) {
	return []kb.SearchResult{}, nil
}

var errExampleDocNotFound = errors.New("document not found")

func (m *mockKBSearcher) GetDocument(ctx context.Context, id string) (*kb.Document, error) {
	return nil, errExampleDocNotFound
}

func (m *mockKBSearcher) IsAvailable() bool {
	return true
}
