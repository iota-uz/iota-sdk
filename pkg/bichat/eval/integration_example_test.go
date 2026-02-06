package eval_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/eval"
)

// Example_integration demonstrates a complete evaluation workflow.
// This is what you'd use in CI/CD to validate agent quality.
func Example_integration() {
	ctx := context.Background()

	// 1. Setup agent runner
	// In production, this would be your actual BiChat agent
	runner := func(ctx context.Context, question string) (string, error) {
		// Mock responses for demonstration
		responses := map[string]string{
			"What were the total sales in 2024?":    "The total sales in 2024 were $5.2 million, representing a 25% increase.",
			"Show me the top 5 products by revenue": "```sql\nSELECT product_name, SUM(revenue) FROM sales GROUP BY product_name ORDER BY SUM(revenue) DESC LIMIT 5\n```\n\nTop products:\n1. Widget A - $1.2M\n2. Gadget B - $980K\n3. Product C - $750K",
		}

		if response, ok := responses[question]; ok {
			return response, nil
		}
		return "I don't have information about that.", nil
	}

	// 2. Setup checkers
	checkers := []eval.Checker{
		eval.NewStringMatchChecker(),
		eval.NewSQLResultChecker(),
		// eval.NewLLMGradeChecker(llmModel), // Add when you have a model
	}

	// 3. Create evaluator
	evaluator := eval.NewEvaluator(runner, checkers...)

	// 4. Load test cases
	cases, err := eval.LoadTestCases("testdata/sample_cases.json")
	if err != nil {
		fmt.Printf("Error loading test cases: %v\n", err)
		return
	}

	// Filter to specific category for this example
	financialCases := eval.FilterByCategory(cases, "financial")
	fmt.Printf("Running %d financial test cases...\n", len(financialCases))

	// 5. Run evaluation
	results, err := evaluator.Run(ctx, financialCases)
	if err != nil {
		fmt.Printf("Error running evaluation: %v\n", err)
		return
	}

	// 6. Analyze results
	passed := 0
	totalScore := 0.0

	for _, result := range results {
		if result.Passed {
			passed++
		}
		totalScore += result.Score

		fmt.Printf("\nTest: %s\n", result.TestCaseID)
		fmt.Printf("  Question: %s\n", result.Question)
		fmt.Printf("  Status: %v (Score: %.2f)\n", result.Passed, result.Score)
		fmt.Printf("  Duration: %dms\n", result.DurationMs)

		for _, check := range result.Checks {
			status := "✓"
			if !check.Passed {
				status = "✗"
			}
			fmt.Printf("  %s %s: %.2f - %s\n", status, check.Name, check.Score, check.Details)
		}
	}

	// 7. Summary
	passRate := float64(passed) / float64(len(results)) * 100
	avgScore := totalScore / float64(len(results))

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Tests: %d\n", len(results))
	fmt.Printf("Passed: %d (%.1f%%)\n", passed, passRate)
	fmt.Printf("Average Score: %.2f\n", avgScore)
}

// TestEvaluationWorkflow demonstrates how to integrate evaluation into tests.
func TestEvaluationWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Setup mock agent with comprehensive responses
	runner := func(ctx context.Context, question string) (string, error) {
		// Provide mock responses that match expected content
		mockResponses := map[string]string{
			"What were the total sales in 2024?":                      "The total sales in 2024 were $5.2 million",
			"Show me the top 5 products by revenue":                   "Here are the top products by revenue",
			"What is the customer growth rate compared to last year?": "Customer growth rate is 18% year-over-year",
			"Show monthly revenue trend for Q4 2024":                  "Monthly revenue trend for Q4 2024 shows growth",
			"Which products have low inventory levels?":               "Products with low inventory levels need attention",
			"What is the average order value?":                        "The average order value is $142.50",
			"What is our customer retention rate?":                    "Our customer retention rate is 87%",
			"Break down sales by region":                              "Sales breakdown by region: North 40%, South 30%, East 20%, West 10%",
		}

		if response, ok := mockResponses[question]; ok {
			return response, nil
		}
		// Default response for unmapped questions
		return "Sample response with basic information", nil
	}

	evaluator := eval.NewEvaluator(
		runner,
		eval.NewStringMatchChecker(),
	)

	// Load test cases
	cases, err := eval.LoadTestCases("testdata/sample_cases.json")
	if err != nil {
		t.Fatalf("Failed to load test cases: %v", err)
	}

	// Run evaluation
	results, err := evaluator.Run(ctx, cases)
	if err != nil {
		t.Fatalf("Evaluation failed: %v", err)
	}

	// Assert quality thresholds
	failedTests := []string{}
	for _, result := range results {
		if !result.Passed {
			failedTests = append(failedTests, fmt.Sprintf("%s (score: %.2f)", result.TestCaseID, result.Score))
		}
	}

	if len(failedTests) > 0 {
		t.Logf("Some tests failed (expected with mock data):\n  - %v", failedTests)
		// Note: In real tests with a proper agent, you'd assert stricter thresholds
	}

	// Calculate metrics
	totalScore := 0.0
	for _, r := range results {
		totalScore += r.Score
	}
	avgScore := totalScore / float64(len(results))

	// For this mock test, just verify we got results
	if len(results) != len(cases) {
		t.Errorf("Expected %d results, got %d", len(cases), len(results))
	}

	t.Logf("Evaluation complete: %d tests, average score: %.2f", len(results), avgScore)
}

// Example_generateReport demonstrates generating evaluation reports.
func Example_generateReport() {
	ctx := context.Background()

	// Run evaluation (simplified)
	runner := func(ctx context.Context, q string) (string, error) {
		return "Sample response", nil
	}

	evaluator := eval.NewEvaluator(runner, eval.NewStringMatchChecker())
	cases := []eval.TestCase{
		{ID: "tc1", Question: "Q1", ExpectedContent: []string{"Sample"}},
	}

	results, _ := evaluator.Run(ctx, cases)

	// Generate JSON report
	report := map[string]interface{}{
		"timestamp": "2024-01-15T10:30:00Z",
		"summary": map[string]interface{}{
			"total":     len(results),
			"passed":    1,
			"failed":    0,
			"avg_score": 1.0,
		},
		"results": results,
	}

	data, _ := json.MarshalIndent(report, "", "  ")
	_ = os.WriteFile("eval_report.json", data, 0644)

	fmt.Println("Report generated: eval_report.json")
}

// Example_batchEvaluation demonstrates running multiple test suites.
func Example_batchEvaluation() {
	ctx := context.Background()

	runner := func(ctx context.Context, q string) (string, error) {
		return "Sample response", nil
	}

	// Run different test suites
	suites := []struct {
		name     string
		category string
	}{
		{"Financial Tests", "financial"},
		{"Analytics Tests", "analytics"},
		{"Operations Tests", "operations"},
	}

	for _, suite := range suites {
		cases, err := eval.LoadTestCases("testdata/sample_cases.json")
		if err != nil {
			continue
		}

		filtered := eval.FilterByCategory(cases, suite.category)
		if len(filtered) == 0 {
			continue
		}

		evaluator := eval.NewEvaluator(runner, eval.NewStringMatchChecker())
		results, err := evaluator.Run(ctx, filtered)
		if err != nil {
			fmt.Printf("Suite %s failed: %v\n", suite.name, err)
			continue
		}

		passed := 0
		for _, r := range results {
			if r.Passed {
				passed++
			}
		}

		fmt.Printf("%s: %d/%d passed\n", suite.name, passed, len(results))
	}
}

// Example_progressiveEvaluation demonstrates running tests with increasing complexity.
func Example_progressiveEvaluation() {
	ctx := context.Background()

	runner := func(ctx context.Context, q string) (string, error) {
		return "Sample response", nil
	}

	cases, _ := eval.LoadTestCases("testdata/sample_cases.json")

	// Phase 1: Fast checkers only
	fmt.Println("Phase 1: Basic validation")
	evaluator1 := eval.NewEvaluator(runner, eval.NewStringMatchChecker())
	results1, _ := evaluator1.Run(ctx, cases)
	fmt.Printf("  Basic checks: %d passed\n", countPassed(results1))

	// Phase 2: Add SQL validation for passing tests
	fmt.Println("Phase 2: SQL validation")
	sqlCases := filterPassed(cases, results1)
	evaluator2 := eval.NewEvaluator(runner, eval.NewSQLResultChecker())
	results2, _ := evaluator2.Run(ctx, sqlCases)
	fmt.Printf("  SQL checks: %d passed\n", countPassed(results2))

	// Phase 3: LLM grading for critical tests (would add when LLM available)
	fmt.Println("Phase 3: LLM grading (skipped - no model)")
}

// Helper functions for examples

func countPassed(results []eval.EvalResult) int {
	count := 0
	for _, r := range results {
		if r.Passed {
			count++
		}
	}
	return count
}

func filterPassed(cases []eval.TestCase, results []eval.EvalResult) []eval.TestCase {
	passedIDs := make(map[string]bool)
	for _, r := range results {
		if r.Passed {
			passedIDs[r.TestCaseID] = true
		}
	}

	filtered := []eval.TestCase{}
	for _, tc := range cases {
		if passedIDs[tc.ID] {
			filtered = append(filtered, tc)
		}
	}
	return filtered
}
