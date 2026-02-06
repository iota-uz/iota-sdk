package eval

import (
	"context"
	"fmt"
	"time"
)

// TestCase defines a single evaluation case.
type TestCase struct {
	ID              string   `json:"id"`
	Question        string   `json:"question"`
	ExpectedSQL     string   `json:"expected_sql,omitempty"`
	ExpectedContent []string `json:"expected_content,omitempty"`
	GoldenAnswer    string   `json:"golden_answer,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	Category        string   `json:"category,omitempty"`
}

// CheckResult holds the outcome of a single check.
type CheckResult struct {
	Name    string  `json:"name"`
	Passed  bool    `json:"passed"`
	Score   float64 `json:"score"`
	Details string  `json:"details,omitempty"`
}

// EvalResult holds the outcome of evaluating a single test case.
type EvalResult struct {
	TestCaseID   string        `json:"test_case_id"`
	Question     string        `json:"question"`
	Passed       bool          `json:"passed"`
	Score        float64       `json:"score"`
	ActualAnswer string        `json:"actual_answer"`
	Checks       []CheckResult `json:"checks"`
	DurationMs   int64         `json:"duration_ms"`
	Error        string        `json:"error,omitempty"`
}

// Checker is a strategy for evaluating a response.
type Checker interface {
	Name() string
	Check(ctx context.Context, tc TestCase, response string) (CheckResult, error)
}

// AgentRunner is the function that runs the agent and returns its response.
// This abstraction allows the evaluator to work with any agent setup.
type AgentRunner func(ctx context.Context, question string) (string, error)

// Evaluator runs test cases against an agent.
type Evaluator struct {
	runner   AgentRunner
	checkers []Checker
}

// NewEvaluator creates an evaluator with the given runner and checkers.
func NewEvaluator(runner AgentRunner, checkers ...Checker) *Evaluator {
	return &Evaluator{
		runner:   runner,
		checkers: checkers,
	}
}

// Run executes all test cases and returns results.
func (e *Evaluator) Run(ctx context.Context, cases []TestCase) ([]EvalResult, error) {
	results := make([]EvalResult, 0, len(cases))

	for _, tc := range cases {
		result, err := e.RunSingle(ctx, tc)
		if err != nil {
			return nil, fmt.Errorf("failed to run test case %s: %w", tc.ID, err)
		}
		results = append(results, *result)
	}

	return results, nil
}

// RunSingle executes a single test case and returns the result.
func (e *Evaluator) RunSingle(ctx context.Context, tc TestCase) (*EvalResult, error) {
	result := &EvalResult{
		TestCaseID: tc.ID,
		Question:   tc.Question,
		Checks:     make([]CheckResult, 0, len(e.checkers)),
	}

	// Measure execution time
	start := time.Now()
	response, err := e.runner(ctx, tc.Question)
	result.DurationMs = time.Since(start).Milliseconds()

	// If runner failed, mark as error
	if err != nil {
		result.Error = err.Error()
		result.Passed = false
		result.Score = 0.0
		return result, nil
	}

	result.ActualAnswer = response

	// Run all checkers
	var totalScore float64
	allPassed := true

	for _, checker := range e.checkers {
		checkResult, err := checker.Check(ctx, tc, response)
		if err != nil {
			// If checker fails, log but continue with other checkers
			checkResult = CheckResult{
				Name:    checker.Name(),
				Passed:  false,
				Score:   0.0,
				Details: fmt.Sprintf("Checker error: %v", err),
			}
		}

		result.Checks = append(result.Checks, checkResult)
		totalScore += checkResult.Score

		if !checkResult.Passed {
			allPassed = false
		}
	}

	// Calculate aggregate results
	if len(e.checkers) > 0 {
		result.Score = totalScore / float64(len(e.checkers))
	} else {
		result.Score = 0.0
	}
	result.Passed = allPassed

	return result, nil
}
