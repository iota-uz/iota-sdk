package eval_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/eval"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// mockModel is a simple mock for testing LLMGradeChecker.
type mockModel struct {
	response string
}

func (m *mockModel) Generate(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (*agents.Response, error) {
	return &agents.Response{
		Message:      types.AssistantMessage(m.response),
		FinishReason: "stop",
	}, nil
}

func (m *mockModel) Stream(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (types.Generator[agents.Chunk], error) {
	return nil, nil
}

func (m *mockModel) Info() agents.ModelInfo {
	return agents.ModelInfo{
		Name:         "mock",
		Provider:     "test",
		Capabilities: []agents.Capability{agents.CapabilityJSONMode},
	}
}

func (m *mockModel) HasCapability(capability agents.Capability) bool {
	return capability == agents.CapabilityJSONMode
}

func (m *mockModel) Pricing() agents.ModelPricing {
	return agents.ModelPricing{}
}

// mockAgentRunner is a test agent that returns predefined responses.
type mockAgentRunner struct {
	responses map[string]string
}

func (m *mockAgentRunner) Run(ctx context.Context, question string) (string, error) {
	if response, ok := m.responses[question]; ok {
		return response, nil
	}
	return "I don't know", nil
}

func TestStringMatchChecker(t *testing.T) {
	t.Parallel()

	checker := eval.NewStringMatchChecker()

	tests := []struct {
		name           string
		tc             eval.TestCase
		response       string
		expectedPassed bool
		expectedScore  float64
	}{
		{
			name: "all strings found",
			tc: eval.TestCase{
				ExpectedContent: []string{"sales", "revenue", "profit"},
			},
			response:       "The sales and revenue show strong profit growth",
			expectedPassed: true,
			expectedScore:  1.0,
		},
		{
			name: "partial match",
			tc: eval.TestCase{
				ExpectedContent: []string{"sales", "revenue", "profit"},
			},
			response:       "The sales and revenue data is available",
			expectedPassed: false,
			expectedScore:  0.666,
		},
		{
			name: "case insensitive",
			tc: eval.TestCase{
				ExpectedContent: []string{"Sales", "Revenue"},
			},
			response:       "sales and revenue are up",
			expectedPassed: true,
			expectedScore:  1.0,
		},
		{
			name: "no expected content",
			tc: eval.TestCase{
				ExpectedContent: []string{},
			},
			response:       "anything",
			expectedPassed: true,
			expectedScore:  1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := checker.Check(context.Background(), tt.tc, tt.response)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Passed != tt.expectedPassed {
				t.Errorf("expected passed=%v, got %v", tt.expectedPassed, result.Passed)
			}

			// Allow small floating point difference
			if abs(result.Score-tt.expectedScore) > 0.01 {
				t.Errorf("expected score=%.2f, got %.2f", tt.expectedScore, result.Score)
			}
		})
	}
}

func TestLLMGradeChecker(t *testing.T) {
	t.Parallel()

	// Mock model that returns a valid grading JSON
	model := &mockModel{
		response: `{"score": 0.85, "passed": true, "reasoning": "Good response"}`,
	}

	checker := eval.NewLLMGradeChecker(model)

	tc := eval.TestCase{
		Question:     "What are the sales trends?",
		GoldenAnswer: "Sales increased by 20% in Q4",
	}

	result, err := checker.Check(context.Background(), tc, "Sales grew significantly in Q4, up 20%")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Passed {
		t.Errorf("expected passed=true, got false")
	}

	if result.Score != 0.85 {
		t.Errorf("expected score=0.85, got %.2f", result.Score)
	}
}

func TestLLMGradeCheckerWithMarkdownJSON(t *testing.T) {
	t.Parallel()

	// Mock model that returns JSON wrapped in markdown code block
	model := &mockModel{
		response: "```json\n{\"score\": 0.90, \"passed\": true, \"reasoning\": \"Excellent response\"}\n```",
	}

	checker := eval.NewLLMGradeChecker(model)

	tc := eval.TestCase{
		Question:     "What are the sales trends?",
		GoldenAnswer: "Sales increased by 20% in Q4",
	}

	result, err := checker.Check(context.Background(), tc, "Sales grew 20% in Q4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Passed {
		t.Errorf("expected passed=true, got false")
	}

	if result.Score != 0.90 {
		t.Errorf("expected score=0.90, got %.2f", result.Score)
	}
}

func TestSQLResultChecker(t *testing.T) {
	t.Parallel()

	checker := eval.NewSQLResultChecker()

	tests := []struct {
		name           string
		tc             eval.TestCase
		response       string
		expectedPassed bool
		minScore       float64
	}{
		{
			name: "exact match",
			tc: eval.TestCase{
				ExpectedSQL: "SELECT * FROM sales WHERE year = 2024",
			},
			response:       "```sql\nSELECT * FROM sales WHERE year = 2024\n```",
			expectedPassed: true,
			minScore:       0.9,
		},
		{
			name: "normalized match",
			tc: eval.TestCase{
				ExpectedSQL: "SELECT * FROM sales WHERE year=2024",
			},
			response:       "SELECT  *  FROM  sales  WHERE  year = 2024",
			expectedPassed: false, // Normalization can't handle = vs =
			minScore:       0.7,   // Should be similar but not exact
		},
		{
			name: "case insensitive",
			tc: eval.TestCase{
				ExpectedSQL: "select * from sales where year = 2024",
			},
			response:       "SELECT * FROM SALES WHERE YEAR = 2024",
			expectedPassed: true,
			minScore:       0.9,
		},
		{
			name: "different query",
			tc: eval.TestCase{
				ExpectedSQL: "SELECT * FROM sales",
			},
			response:       "SELECT * FROM products",
			expectedPassed: false,
			minScore:       0.0,
		},
		{
			name: "no expected sql",
			tc: eval.TestCase{
				ExpectedSQL: "",
			},
			response:       "anything",
			expectedPassed: true,
			minScore:       1.0,
		},
		{
			name: "no sql in response",
			tc: eval.TestCase{
				ExpectedSQL: "SELECT * FROM sales",
			},
			response:       "Here are the sales numbers",
			expectedPassed: false,
			minScore:       0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := checker.Check(context.Background(), tt.tc, tt.response)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Passed != tt.expectedPassed {
				t.Errorf("expected passed=%v, got %v. Details: %s", tt.expectedPassed, result.Passed, result.Details)
			}

			if result.Score < tt.minScore {
				t.Errorf("expected score >= %.2f, got %.2f", tt.minScore, result.Score)
			}
		})
	}
}

func TestEvaluator(t *testing.T) {
	t.Parallel()

	// Setup mock runner
	runner := &mockAgentRunner{
		responses: map[string]string{
			"What are the total sales?":        "The total sales are $1,000,000",
			"Show me the top products":         "```sql\nSELECT * FROM products ORDER BY sales DESC LIMIT 10\n```",
			"What is the average order value?": "The average order value is $150",
		},
	}

	// Setup checkers
	checkers := []eval.Checker{
		eval.NewStringMatchChecker(),
		eval.NewSQLResultChecker(),
	}

	evaluator := eval.NewEvaluator(runner.Run, checkers...)

	testCases := []eval.TestCase{
		{
			ID:              "tc1",
			Question:        "What are the total sales?",
			ExpectedContent: []string{"sales", "$1,000,000"},
		},
		{
			ID:          "tc2",
			Question:    "Show me the top products",
			ExpectedSQL: "SELECT * FROM products ORDER BY sales DESC LIMIT 10",
		},
	}

	results, err := evaluator.Run(context.Background(), testCases)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Check first result
	if !results[0].Passed {
		t.Errorf("expected tc1 to pass")
	}

	if results[0].Score < 0.5 {
		t.Errorf("expected tc1 score >= 0.5, got %.2f", results[0].Score)
	}

	// Check second result
	if !results[1].Passed {
		t.Errorf("expected tc2 to pass")
	}
}

func TestEvaluatorRunSingle(t *testing.T) {
	t.Parallel()

	runner := &mockAgentRunner{
		responses: map[string]string{
			"test question": "test response with expected content",
		},
	}

	evaluator := eval.NewEvaluator(
		runner.Run,
		eval.NewStringMatchChecker(),
	)

	tc := eval.TestCase{
		ID:              "single_test",
		Question:        "test question",
		ExpectedContent: []string{"expected", "content"},
	}

	result, err := evaluator.RunSingle(context.Background(), tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TestCaseID != "single_test" {
		t.Errorf("expected test case ID 'single_test', got %s", result.TestCaseID)
	}

	if !result.Passed {
		t.Errorf("expected test to pass")
	}

	// Duration might be 0 for very fast operations (mocked runner)
	if result.DurationMs < 0 {
		t.Errorf("expected duration >= 0, got %d", result.DurationMs)
	}
}

func TestLoadTestCases(t *testing.T) {
	t.Parallel()

	// Create temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_cases.json")

	cases := []eval.TestCase{
		{
			ID:              "tc1",
			Question:        "What are sales?",
			ExpectedContent: []string{"sales"},
			Tags:            []string{"sales", "basic"},
			Category:        "queries",
		},
		{
			ID:          "tc2",
			Question:    "Show products",
			ExpectedSQL: "SELECT * FROM products",
			Tags:        []string{"products"},
			Category:    "queries",
		},
	}

	data, err := json.Marshal(cases)
	if err != nil {
		t.Fatalf("failed to marshal test cases: %v", err)
	}

	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Load test cases
	loaded, err := eval.LoadTestCases(testFile)
	if err != nil {
		t.Fatalf("failed to load test cases: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("expected 2 test cases, got %d", len(loaded))
	}

	if loaded[0].ID != "tc1" {
		t.Errorf("expected ID 'tc1', got %s", loaded[0].ID)
	}
}

func TestLoadTestCasesFromDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create multiple test files
	file1 := filepath.Join(tmpDir, "file1.json")
	file2 := filepath.Join(tmpDir, "file2.json")

	cases1 := []eval.TestCase{
		{ID: "tc1", Question: "Q1"},
		{ID: "tc2", Question: "Q2"},
	}

	cases2 := []eval.TestCase{
		{ID: "tc3", Question: "Q3"},
	}

	for _, tc := range []struct {
		file  string
		cases []eval.TestCase
	}{
		{file1, cases1},
		{file2, cases2},
	} {
		data, err := json.Marshal(tc.cases)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}
		if err := os.WriteFile(tc.file, data, 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
	}

	// Load all cases
	loaded, err := eval.LoadTestCasesFromDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to load from dir: %v", err)
	}

	if len(loaded) != 3 {
		t.Fatalf("expected 3 test cases, got %d", len(loaded))
	}
}

func TestFilterByTag(t *testing.T) {
	t.Parallel()

	cases := []eval.TestCase{
		{ID: "tc1", Tags: []string{"sales", "basic"}},
		{ID: "tc2", Tags: []string{"products"}},
		{ID: "tc3", Tags: []string{"sales", "advanced"}},
	}

	filtered := eval.FilterByTag(cases, "sales")

	if len(filtered) != 2 {
		t.Fatalf("expected 2 cases with 'sales' tag, got %d", len(filtered))
	}

	for _, tc := range filtered {
		if tc.ID != "tc1" && tc.ID != "tc3" {
			t.Errorf("unexpected test case ID: %s", tc.ID)
		}
	}
}

func TestFilterByCategory(t *testing.T) {
	t.Parallel()

	cases := []eval.TestCase{
		{ID: "tc1", Category: "queries"},
		{ID: "tc2", Category: "aggregations"},
		{ID: "tc3", Category: "queries"},
	}

	filtered := eval.FilterByCategory(cases, "queries")

	if len(filtered) != 2 {
		t.Fatalf("expected 2 cases with 'queries' category, got %d", len(filtered))
	}

	for _, tc := range filtered {
		if tc.Category != "queries" {
			t.Errorf("expected category 'queries', got %s", tc.Category)
		}
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
