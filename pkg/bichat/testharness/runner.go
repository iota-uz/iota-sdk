package testharness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type TestStatus string

const (
	TestStatusPassed TestStatus = "PASSED"
	TestStatusFailed TestStatus = "FAILED"
	TestStatusError  TestStatus = "ERROR"
)

type TurnReport struct {
	Prompt         string        `json:"prompt"`
	StreamedAnswer string        `json:"streamed_answer,omitempty"`
	FinalAnswer    string        `json:"final_answer,omitempty"`
	SSEErrorJSON   string        `json:"sse_error_json,omitempty"`
	ToolCalls      []ToolCall    `json:"tool_calls,omitempty"`
	Verdict        *JudgeVerdict `json:"verdict,omitempty"`
	DurationMS     int64         `json:"duration_ms"`
}

type TestReport struct {
	ID          string       `json:"id"`
	Description string       `json:"description,omitempty"`
	Status      TestStatus   `json:"status"`
	Error       string       `json:"error,omitempty"`
	DurationMS  int64        `json:"duration_ms"`
	Turns       []TurnReport `json:"turns"`
}

type RunReport struct {
	GeneratedAt time.Time    `json:"generated_at"`
	Cached      bool         `json:"cached"`
	CacheKey    string       `json:"cache_key,omitempty"`
	Tests       []TestReport `json:"tests"`
	Summary     Summary      `json:"summary"`
}

type Summary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Errored int `json:"errored"`
}

type Runner struct {
	cfg   Config
	gql   *GraphQLClient
	sse   *SSEClient
	judge *OpenAIJudge
	cache *Cache
}

func NewRunner(cfg Config) (*Runner, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	r := &Runner{
		cfg:   cfg,
		gql:   NewGraphQLClient(cfg),
		sse:   NewSSEClient(cfg),
		cache: NewCache(cfg),
	}
	if !cfg.DisableJudge {
		r.judge = NewOpenAIJudge(cfg)
	}
	return r, nil
}

func LoadSuiteJSON(data []byte) (TestSuite, error) {
	var s TestSuite
	if err := json.Unmarshal(data, &s); err != nil {
		return TestSuite{}, err
	}
	return s, nil
}

func (r *Runner) Run(ctx context.Context, suite TestSuite) (RunReport, int, error) {
	if r == nil {
		return RunReport{}, 2, errors.New("runner is nil")
	}

	report := RunReport{
		GeneratedAt: time.Now(),
		Tests:       make([]TestReport, len(suite.Tests)),
	}

	if r.cache.Enabled() {
		key, err := r.cache.Key(suite, r.cfg)
		if err != nil {
			return RunReport{}, 2, err
		}
		if cached, ok, err := r.cache.LoadReport(key); err == nil && ok && cached != nil {
			cached.Cached = true
			cached.CacheKey = key
			return *cached, exitCode(*cached), nil
		}
		report.CacheKey = key
	}

	parallelism := r.cfg.EffectiveParallelism()
	if parallelism < 1 {
		parallelism = 1
	}

	type job struct {
		idx int
		tc  TestCase
	}
	type jobResult struct {
		idx int
		tr  TestReport
	}

	jobs := make(chan job, len(suite.Tests))
	results := make(chan jobResult, len(suite.Tests))

	var wg sync.WaitGroup
	for i := 0; i < parallelism; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				results <- jobResult{idx: j.idx, tr: r.runOneTest(ctx, j.tc)}
			}
		}()
	}

	for i, tc := range suite.Tests {
		jobs <- job{idx: i, tc: tc}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	for res := range results {
		report.Tests[res.idx] = res.tr
	}

	report.Summary = summarize(report.Tests)

	if r.cache.Enabled() && report.CacheKey != "" {
		_ = r.cache.SaveReport(report.CacheKey, report)
	}

	return report, exitCode(report), nil
}

func (r *Runner) runOneTest(parent context.Context, tc TestCase) TestReport {
	start := time.Now()

	tr := TestReport{
		ID:          tc.ID,
		Description: tc.Description,
		Status:      TestStatusPassed,
		Turns:       make([]TurnReport, 0, len(tc.Turns)),
	}

	timeoutSeconds := tc.MaxDurationSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	sessionID, err := r.gql.CreateSession(ctx, "")
	if err != nil {
		tr.Status = TestStatusError
		tr.Error = fmt.Sprintf("create session: %v", err)
		tr.DurationMS = time.Since(start).Milliseconds()
		return tr
	}

	for _, turn := range tc.Turns {
		turnStart := time.Now()

		sseRes, err := r.sse.StreamMessage(ctx, sessionID, turn.Prompt)
		if err != nil {
			tr.Status = TestStatusError
			tr.Error = fmt.Sprintf("sse stream: %v", err)
			break
		}

		finalAnswer, toolCalls, err := r.fetchFinalAnswer(ctx, sessionID)
		if err != nil {
			tr.Status = TestStatusError
			tr.Error = fmt.Sprintf("fetch final answer: %v", err)
			break
		}
		if strings.TrimSpace(finalAnswer) == "" {
			finalAnswer = sseRes.StreamedContent
		}

		var sseErrorJSON string
		if sseRes.ErrorPayload != nil {
			if b, err := json.Marshal(sseRes.ErrorPayload); err == nil {
				sseErrorJSON = string(b)
			}
		}

		turnReport := TurnReport{
			Prompt:         turn.Prompt,
			StreamedAnswer: sseRes.StreamedContent,
			FinalAnswer:    finalAnswer,
			SSEErrorJSON:   sseErrorJSON,
			ToolCalls:      toolCalls,
			DurationMS:     time.Since(turnStart).Milliseconds(),
		}

		if r.judge != nil {
			verdict, err := r.judge.Evaluate(ctx, JudgeTurnInput{
				UserPrompt:        turn.Prompt,
				FinalAnswer:       finalAnswer,
				StreamedAnswer:    sseRes.StreamedContent,
				SSEError:          sseErrorJSON,
				ExpectedChecklist: turn.ExpectedChecklist,
				JudgeInstructions: turn.JudgeInstructions,
				ToolCalls:         toolCalls,
			})
			if err != nil {
				tr.Status = TestStatusError
				tr.Error = fmt.Sprintf("judge: %v", err)
				turnReport.Verdict = nil
				tr.Turns = append(tr.Turns, turnReport)
				break
			}
			turnReport.Verdict = verdict
			if !verdict.Passed && tr.Status == TestStatusPassed {
				tr.Status = TestStatusFailed
			}
		}

		tr.Turns = append(tr.Turns, turnReport)
		if tr.Status == TestStatusError {
			break
		}
	}

	tr.DurationMS = time.Since(start).Milliseconds()
	return tr
}

func (r *Runner) fetchFinalAnswer(ctx context.Context, sessionID uuid.UUID) (string, []ToolCall, error) {
	msgs, err := r.gql.Messages(ctx, sessionID, 500, 0)
	if err != nil {
		return "", nil, err
	}
	var lastAssistant *Message
	for i := range msgs {
		if msgs[i].Role == "ASSISTANT" {
			lastAssistant = &msgs[i]
		}
	}
	if lastAssistant == nil {
		return "", nil, nil
	}
	return lastAssistant.Content, lastAssistant.ToolCalls, nil
}

func summarize(tests []TestReport) Summary {
	var s Summary
	s.Total = len(tests)
	for _, t := range tests {
		switch t.Status {
		case TestStatusPassed:
			s.Passed++
		case TestStatusFailed:
			s.Failed++
		case TestStatusError:
			s.Errored++
		}
	}
	return s
}

func exitCode(report RunReport) int {
	if report.Summary.Errored > 0 {
		return 2
	}
	if report.Summary.Failed > 0 {
		return 1
	}
	return 0
}
