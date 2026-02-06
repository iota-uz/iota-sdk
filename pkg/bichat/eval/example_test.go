package eval_test

import (
	"context"
	"fmt"
	"log"

	"github.com/iota-uz/iota-sdk/pkg/bichat/eval"
)

// Example demonstrates basic usage of the evaluation framework.
func Example() {
	// 1. Create an agent runner
	// This is a simple mock, but in production you'd use your actual agent
	runner := func(ctx context.Context, question string) (string, error) {
		// Your agent execution logic here
		return "The total sales for 2024 were $5.2 million", nil
	}

	// 2. Setup checkers
	checkers := []eval.Checker{
		eval.NewStringMatchChecker(),
		eval.NewSQLResultChecker(),
	}

	// 3. Create evaluator
	evaluator := eval.NewEvaluator(runner, checkers...)

	// 4. Define test cases
	testCases := []eval.TestCase{
		{
			ID:              "sales_query",
			Question:        "What were the total sales in 2024?",
			ExpectedContent: []string{"sales", "2024", "million"},
			Tags:            []string{"sales", "aggregation"},
			Category:        "financial",
		},
	}

	// 5. Run evaluation
	results, err := evaluator.Run(context.Background(), testCases)
	if err != nil {
		log.Fatal(err)
	}

	// 6. Process results
	for _, result := range results {
		fmt.Printf("Test %s: %v (score: %.2f)\n", result.TestCaseID, result.Passed, result.Score)
		for _, check := range result.Checks {
			fmt.Printf("  - %s: %v (%.2f)\n", check.Name, check.Passed, check.Score)
		}
	}
}

// ExampleLoadTestCases demonstrates loading test cases from JSON.
func ExampleLoadTestCases() {
	// Load test cases from a JSON file
	cases, err := eval.LoadTestCases("testdata/cases.json")
	if err != nil {
		log.Fatal(err)
	}

	// Filter by category
	salesCases := eval.FilterByCategory(cases, "sales")

	// Filter by tag
	basicCases := eval.FilterByTag(salesCases, "basic")

	fmt.Printf("Loaded %d test cases, filtered to %d\n", len(cases), len(basicCases))
}

// ExampleLLMGradeChecker demonstrates using LLM-based grading.
func ExampleLLMGradeChecker() {
	// In production, you'd use a real model implementation
	// model := openai.NewModel(client, openai.ModelConfig{Name: "gpt-4"})

	// For this example, we'll skip the actual model creation
	// checker := eval.NewLLMGradeChecker(model)

	fmt.Println("LLMGradeChecker evaluates responses using an LLM judge")
	fmt.Println("It compares the actual response to a golden answer")
	fmt.Println("Returns a score (0.0-1.0) and reasoning")
}

// ExampleEvaluator_RunSingle demonstrates running a single test case.
func ExampleEvaluator_RunSingle() {
	runner := func(ctx context.Context, question string) (string, error) {
		return "SELECT product_name, SUM(quantity) FROM orders GROUP BY product_name ORDER BY SUM(quantity) DESC LIMIT 10", nil
	}

	evaluator := eval.NewEvaluator(
		runner,
		eval.NewSQLResultChecker(),
	)

	testCase := eval.TestCase{
		ID:          "top_products",
		Question:    "Show me the top 10 products by sales",
		ExpectedSQL: "SELECT product_name, SUM(quantity) FROM orders GROUP BY product_name ORDER BY SUM(quantity) DESC LIMIT 10",
	}

	result, err := evaluator.RunSingle(context.Background(), testCase)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Test passed: %v, Score: %.2f, Duration: %dms\n",
		result.Passed, result.Score, result.DurationMs)
}

// ExampleStringMatchChecker demonstrates string matching.
func ExampleStringMatchChecker() {
	checker := eval.NewStringMatchChecker()

	testCase := eval.TestCase{
		ExpectedContent: []string{"revenue", "increased", "Q4"},
	}

	response := "Revenue increased by 25% in Q4"

	result, err := checker.Check(context.Background(), testCase, response)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Passed: %v, Score: %.2f\n", result.Passed, result.Score)
	// Output: Passed: true, Score: 1.00
}

// ExampleSQLResultChecker demonstrates SQL comparison.
func ExampleSQLResultChecker() {
	checker := eval.NewSQLResultChecker()

	testCase := eval.TestCase{
		ExpectedSQL: "SELECT * FROM products WHERE category = 'electronics'",
	}

	response := "```sql\nSELECT * FROM products WHERE category = 'electronics'\n```"

	result, err := checker.Check(context.Background(), testCase, response)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("SQL match passed: %v\n", result.Passed)
	// Output: SQL match passed: true
}
