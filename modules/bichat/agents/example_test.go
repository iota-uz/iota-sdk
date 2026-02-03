package agents_test

import (
	"context"
	"fmt"
	"time"

	bichatagents "github.com/iota-uz/iota-sdk/modules/bichat/agents"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/tools"
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
	// Agent model: gpt-4
	// Number of tools: 6
}

// ExampleNewDefaultBIAgent_withOptions demonstrates using optional tools.
func ExampleNewDefaultBIAgent_withOptions() {
	executor := &mockQueryExecutor{}
	kbSearcher := &mockKBSearcher{}

	// Create agent with optional KB search
	agent, err := bichatagents.NewDefaultBIAgent(
		executor,
		bichatagents.WithKBSearcher(kbSearcher),
		bichatagents.WithModel("gpt-3.5-turbo"),
	)
	if err != nil {
		panic(err)
	}

	metadata := agent.Metadata()
	agentTools := agent.Tools()

	fmt.Println("Model:", metadata.Model)
	fmt.Printf("Tools count: %d\n", len(agentTools))

	// Output:
	// Model: gpt-3.5-turbo
	// Tools count: 7
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

func (m *mockKBSearcher) Search(ctx context.Context, query string, limit int) ([]tools.SearchResult, error) {
	return []tools.SearchResult{}, nil
}

func (m *mockKBSearcher) IsAvailable() bool {
	return true
}
