package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// StringMatchChecker checks if expected strings appear in the response.
type StringMatchChecker struct{}

// NewStringMatchChecker creates a new StringMatchChecker.
func NewStringMatchChecker() *StringMatchChecker {
	return &StringMatchChecker{}
}

// Name returns the checker name.
func (c *StringMatchChecker) Name() string {
	return "string_match"
}

// Check verifies that all expected strings appear in the response.
func (c *StringMatchChecker) Check(ctx context.Context, tc TestCase, response string) (CheckResult, error) {
	// Skip if no expected content
	if len(tc.ExpectedContent) == 0 {
		return CheckResult{
			Name:    c.Name(),
			Passed:  true,
			Score:   1.0,
			Details: "No expected content to check (skipped)",
		}, nil
	}

	responseLower := strings.ToLower(response)
	found := 0
	missing := []string{}

	for _, expected := range tc.ExpectedContent {
		if strings.Contains(responseLower, strings.ToLower(expected)) {
			found++
		} else {
			missing = append(missing, expected)
		}
	}

	score := float64(found) / float64(len(tc.ExpectedContent))
	passed := found == len(tc.ExpectedContent)

	details := fmt.Sprintf("Found %d/%d expected strings", found, len(tc.ExpectedContent))
	if len(missing) > 0 {
		details += fmt.Sprintf(". Missing: %v", missing)
	}

	return CheckResult{
		Name:    c.Name(),
		Passed:  passed,
		Score:   score,
		Details: details,
	}, nil
}

// LLMGradeChecker uses an LLM to grade the response.
type LLMGradeChecker struct {
	model agents.Model
}

// NewLLMGradeChecker creates a new LLMGradeChecker.
func NewLLMGradeChecker(model agents.Model) *LLMGradeChecker {
	return &LLMGradeChecker{
		model: model,
	}
}

// Name returns the checker name.
func (c *LLMGradeChecker) Name() string {
	return "llm_grade"
}

// gradeResponse holds the LLM grading result.
type gradeResponse struct {
	Score     float64 `json:"score"`
	Passed    bool    `json:"passed"`
	Reasoning string  `json:"reasoning"`
}

// Check uses an LLM to evaluate the response quality.
func (c *LLMGradeChecker) Check(ctx context.Context, tc TestCase, response string) (CheckResult, error) {
	// Skip if no golden answer
	if tc.GoldenAnswer == "" {
		return CheckResult{
			Name:    c.Name(),
			Passed:  true,
			Score:   1.0,
			Details: "No golden answer to check (skipped)",
		}, nil
	}

	// Build grading prompt
	systemPrompt := `You are an expert evaluator for AI assistant responses. Your task is to grade how well an actual response matches an expected answer.

Evaluate based on:
1. Correctness - Does it provide accurate information?
2. Completeness - Does it cover all key points from the expected answer?
3. Relevance - Does it stay on topic and answer the question?

Return a JSON object with:
- "score": A number between 0.0 and 1.0 (0.0 = completely wrong, 1.0 = perfect match)
- "passed": true if score >= 0.7, false otherwise
- "reasoning": Brief explanation of the grade

Example output:
{"score": 0.85, "passed": true, "reasoning": "Response correctly identifies the main trend and provides accurate numbers, but misses one minor detail about seasonality."}`

	userPrompt := fmt.Sprintf(`Question: %s

Expected Answer:
%s

Actual Response:
%s

Please evaluate the actual response and return a JSON grading.`, tc.Question, tc.GoldenAnswer, response)

	// Create request
	req := agents.Request{
		Messages: []types.Message{
			types.SystemMessage(systemPrompt),
			types.UserMessage(userPrompt),
		},
	}

	// Generate evaluation
	var opts []agents.GenerateOption

	// Use JSON mode if available
	if c.model.HasCapability(agents.CapabilityJSONMode) {
		opts = append(opts, agents.WithJSONMode())
	}

	// Lower temperature for more deterministic grading
	temp := 0.3
	opts = append(opts, agents.WithTemperature(temp))

	resp, err := c.model.Generate(ctx, req, opts...)
	if err != nil {
		return CheckResult{
			Name:    c.Name(),
			Passed:  false,
			Score:   0.0,
			Details: fmt.Sprintf("LLM grading failed: %v", err),
		}, err
	}

	// Parse LLM response
	var grade gradeResponse
	content := resp.Message.Content()

	// Try to extract JSON if wrapped in markdown code blocks
	if strings.Contains(content, "```") {
		re := regexp.MustCompile("```(?:json)?\n([^`]+)\n```")
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 {
			content = matches[1]
		}
	}

	if err := json.Unmarshal([]byte(content), &grade); err != nil {
		return CheckResult{
			Name:    c.Name(),
			Passed:  false,
			Score:   0.0,
			Details: fmt.Sprintf("Failed to parse LLM grading response: %v. Response: %s", err, content),
		}, err
	}

	// Clamp score to valid range
	if grade.Score < 0.0 {
		grade.Score = 0.0
	}
	if grade.Score > 1.0 {
		grade.Score = 1.0
	}

	return CheckResult{
		Name:    c.Name(),
		Passed:  grade.Passed,
		Score:   grade.Score,
		Details: grade.Reasoning,
	}, nil
}

// SQLResultChecker compares SQL queries using basic normalization.
type SQLResultChecker struct{}

// NewSQLResultChecker creates a new SQLResultChecker.
func NewSQLResultChecker() *SQLResultChecker {
	return &SQLResultChecker{}
}

// Name returns the checker name.
func (c *SQLResultChecker) Name() string {
	return "sql_result"
}

// Check extracts and compares SQL queries.
func (c *SQLResultChecker) Check(ctx context.Context, tc TestCase, response string) (CheckResult, error) {
	// Skip if no expected SQL
	if tc.ExpectedSQL == "" {
		return CheckResult{
			Name:    c.Name(),
			Passed:  true,
			Score:   1.0,
			Details: "No expected SQL to check (skipped)",
		}, nil
	}

	// Extract SQL from response
	actualSQL := extractSQL(response)
	if actualSQL == "" {
		return CheckResult{
			Name:    c.Name(),
			Passed:  false,
			Score:   0.0,
			Details: "No SQL found in response",
		}, nil
	}

	// Normalize both SQL queries
	expectedNorm := normalizeSQL(tc.ExpectedSQL)
	actualNorm := normalizeSQL(actualSQL)

	// Calculate similarity
	score := calculateSQLSimilarity(expectedNorm, actualNorm)
	passed := score >= 0.8 // 80% similarity threshold

	details := fmt.Sprintf("SQL similarity: %.2f. Expected:\n%s\n\nActual:\n%s", score, expectedNorm, actualNorm)

	return CheckResult{
		Name:    c.Name(),
		Passed:  passed,
		Score:   score,
		Details: details,
	}, nil
}

// extractSQL extracts SQL code from response (looks for SELECT, INSERT, UPDATE, DELETE, WITH).
func extractSQL(response string) string {
	// Try to find SQL in code blocks first
	re := regexp.MustCompile("```(?:sql)?\n([^`]+)\n```")
	matches := re.FindStringSubmatch(response)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Look for SQL keywords
	sqlKeywords := []string{"SELECT", "INSERT", "UPDATE", "DELETE", "WITH"}
	responseLower := strings.ToLower(response)

	for _, keyword := range sqlKeywords {
		idx := strings.Index(responseLower, strings.ToLower(keyword))
		if idx >= 0 {
			// Extract from keyword to end (or to next non-SQL looking part)
			sql := response[idx:]
			// Try to find the end of SQL statement
			if endIdx := strings.Index(sql, ";"); endIdx > 0 {
				return strings.TrimSpace(sql[:endIdx+1])
			}
			// If no semicolon, try to extract reasonable length
			if len(sql) > 500 {
				sql = sql[:500]
			}
			return strings.TrimSpace(sql)
		}
	}

	return ""
}

// normalizeSQL normalizes SQL for comparison.
func normalizeSQL(sql string) string {
	// Convert to lowercase
	sql = strings.ToLower(sql)

	// Remove extra whitespace
	sql = regexp.MustCompile(`\s+`).ReplaceAllString(sql, " ")

	// Remove comments
	sql = regexp.MustCompile(`--[^\n]*`).ReplaceAllString(sql, "")
	sql = regexp.MustCompile(`/\*.*?\*/`).ReplaceAllString(sql, "")

	// Trim
	sql = strings.TrimSpace(sql)

	return sql
}

// calculateSQLSimilarity calculates basic string similarity between SQL queries.
// Uses a simple token-based approach (not semantic SQL parsing).
func calculateSQLSimilarity(expected, actual string) float64 {
	// Exact match
	if expected == actual {
		return 1.0
	}

	// Tokenize both queries
	expectedTokens := tokenizeSQL(expected)
	actualTokens := tokenizeSQL(actual)

	if len(expectedTokens) == 0 && len(actualTokens) == 0 {
		return 1.0
	}
	if len(expectedTokens) == 0 || len(actualTokens) == 0 {
		return 0.0
	}

	// Count matching tokens
	expectedMap := make(map[string]int)
	for _, token := range expectedTokens {
		expectedMap[token]++
	}

	actualMap := make(map[string]int)
	for _, token := range actualTokens {
		actualMap[token]++
	}

	// Calculate intersection
	matches := 0
	for token, expectedCount := range expectedMap {
		if actualCount, ok := actualMap[token]; ok {
			if expectedCount <= actualCount {
				matches += expectedCount
			} else {
				matches += actualCount
			}
		}
	}

	// Use Jaccard-like similarity
	// Score = (2 * matches) / (total expected + total actual)
	total := len(expectedTokens) + len(actualTokens)
	if total == 0 {
		return 0.0
	}

	return (2.0 * float64(matches)) / float64(total)
}

// tokenizeSQL splits SQL into tokens.
func tokenizeSQL(sql string) []string {
	// Split on whitespace and common SQL delimiters
	sql = regexp.MustCompile(`[(),;]`).ReplaceAllString(sql, " ")
	tokens := strings.Fields(sql)

	// Filter out very short tokens (like single chars)
	filtered := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if len(token) > 0 {
			filtered = append(filtered, token)
		}
	}

	return filtered
}
