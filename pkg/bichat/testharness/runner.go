package testharness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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

type FailureKind string

const (
	FailureKindNone                        FailureKind = ""
	FailureKindNotAuthenticatedRedirect    FailureKind = "NotAuthenticatedRedirect"
	FailureKindForbidden                   FailureKind = "Forbidden"
	FailureKindHTTPError                   FailureKind = "HTTPError"
	FailureKindSSEErrorPayload             FailureKind = "SSEErrorPayload"
	FailureKindPersistenceMissingAfterDone FailureKind = "PersistenceMissingAfterDone"
	FailureKindJudgeError                  FailureKind = "JudgeError"
)

type TurnReport struct {
	Prompt         string        `json:"prompt"`
	StreamedAnswer string        `json:"streamed_answer,omitempty"`
	FinalAnswer    string        `json:"final_answer,omitempty"`
	SSEErrorJSON   string        `json:"sse_error_json,omitempty"`
	ToolCalls      []ToolCall    `json:"tool_calls,omitempty"`
	Verdict        *JudgeVerdict `json:"verdict,omitempty"`
	JudgeCached    bool          `json:"judge_cached,omitempty"`
	Status         TestStatus    `json:"status,omitempty"`
	FailureKind    FailureKind   `json:"failure_kind,omitempty"`
	Error          string        `json:"error,omitempty"`
	ArtifactsDir   string        `json:"artifacts_dir,omitempty"`
	DurationMS     int64         `json:"duration_ms"`
}

type TestReport struct {
	ID           string       `json:"id"`
	Description  string       `json:"description,omitempty"`
	Status       TestStatus   `json:"status"`
	FailureKind  FailureKind  `json:"failure_kind,omitempty"`
	Error        string       `json:"error,omitempty"`
	ArtifactsDir string       `json:"artifacts_dir,omitempty"`
	DurationMS   int64        `json:"duration_ms"`
	Turns        []TurnReport `json:"turns"`
}

type RunReport struct {
	GeneratedAt  time.Time    `json:"generated_at"`
	Cached       bool         `json:"cached"`
	CacheKey     string       `json:"cache_key,omitempty"`
	RunID        string       `json:"run_id,omitempty"`
	ArtifactsDir string       `json:"artifacts_dir,omitempty"`
	Tests        []TestReport `json:"tests"`
	Summary      Summary      `json:"summary"`
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

	runID        string
	runArtifacts string
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

	r.runID = time.Now().Format("20060102-150405")
	if strings.TrimSpace(r.cfg.ArtifactsDir) != "" {
		r.runArtifacts = filepath.Join(r.cfg.ArtifactsDir, r.runID)
		if err := os.MkdirAll(r.runArtifacts, 0o755); err != nil {
			return RunReport{}, 2, fmt.Errorf("create artifacts dir: %w", err)
		}
	}

	report := RunReport{
		GeneratedAt:  time.Now(),
		Tests:        make([]TestReport, len(suite.Tests)),
		RunID:        r.runID,
		ArtifactsDir: r.runArtifacts,
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

	testArtifacts := ""
	if r.runArtifacts != "" {
		testArtifacts = filepath.Join(r.runArtifacts, sanitizePathComponent(tc.ID))
		_ = os.MkdirAll(testArtifacts, 0o755)
		tr.ArtifactsDir = testArtifacts
	}

	timeoutSeconds := tc.MaxDurationSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	sessionID, err := r.gql.CreateSession(ctx, "")
	if err != nil {
		if tc.Expect.RedirectUnauth && isNotAuthenticatedRedirect(err) {
			tr.Status = TestStatusPassed
			tr.DurationMS = time.Since(start).Milliseconds()
			return tr
		}
		if tc.Expect.Forbidden && isForbidden(err) {
			tr.Status = TestStatusPassed
			tr.DurationMS = time.Since(start).Milliseconds()
			return tr
		}
		tr.Status = TestStatusError
		tr.FailureKind = classifyFailure(err)
		tr.Error = fmt.Sprintf("create session: %v", err)
		tr.DurationMS = time.Since(start).Milliseconds()
		return tr
	}

	for turnIdx, turn := range tc.Turns {
		turnStart := time.Now()

		baselineAssistantCount := 0
		if msgs, msgErr := r.gql.Messages(ctx, sessionID, 500, 0); msgErr == nil {
			for i := range msgs {
				if strings.EqualFold(msgs[i].Role, "assistant") {
					baselineAssistantCount++
				}
			}
		}

		turnArtifacts := ""
		if testArtifacts != "" {
			turnArtifacts = filepath.Join(testArtifacts, fmt.Sprintf("turn-%d", turnIdx+1))
			_ = os.MkdirAll(turnArtifacts, 0o755)
			_ = os.WriteFile(filepath.Join(turnArtifacts, "prompt.txt"), []byte(turn.Prompt), 0o644)
		}

		var sseRawFile, sseJSONFile *os.File
		if turnArtifacts != "" {
			sseRawFile, _ = os.Create(filepath.Join(turnArtifacts, "sse.txt"))
			sseJSONFile, _ = os.Create(filepath.Join(turnArtifacts, "sse.jsonl"))
		}
		if sseRawFile != nil {
			defer func() { _ = sseRawFile.Close() }()
		}
		if sseJSONFile != nil {
			defer func() { _ = sseJSONFile.Close() }()
		}

		sseRes, err := r.sse.StreamMessage(ctx, sessionID, turn.Prompt, StreamSinks{Raw: sseRawFile, JSON: sseJSONFile})
		if err != nil {
			if tc.Expect.RedirectUnauth && isNotAuthenticatedRedirect(err) {
				tr.Status = TestStatusPassed
				break
			}
			if tc.Expect.Forbidden && isForbidden(err) {
				tr.Status = TestStatusPassed
				break
			}
			tr.Status = TestStatusError
			tr.FailureKind = classifyFailure(err)
			tr.Error = fmt.Sprintf("sse stream: %v", err)
			tr.Turns = append(tr.Turns, TurnReport{
				Prompt:       turn.Prompt,
				Status:       TestStatusError,
				FailureKind:  classifyFailure(err),
				Error:        err.Error(),
				ArtifactsDir: turnArtifacts,
				DurationMS:   time.Since(turnStart).Milliseconds(),
			})
			break
		}

		finalAnswer, toolCalls, waitErr := r.waitForAssistantMessage(ctx, sessionID, baselineAssistantCount)
		if waitErr != nil {
			tr.Status = TestStatusError
			tr.FailureKind = FailureKindPersistenceMissingAfterDone
			tr.Error = fmt.Sprintf("fetch final answer: %v", waitErr)
			tr.Turns = append(tr.Turns, TurnReport{
				Prompt:       turn.Prompt,
				Status:       TestStatusError,
				FailureKind:  FailureKindPersistenceMissingAfterDone,
				Error:        waitErr.Error(),
				ArtifactsDir: turnArtifacts,
				DurationMS:   time.Since(turnStart).Milliseconds(),
			})
			break
		}
		if strings.TrimSpace(finalAnswer) == "" && sseRes != nil {
			finalAnswer = sseRes.StreamedContent
		}

		var sseErrorJSON string
		if sseRes != nil && sseRes.ErrorPayload != nil {
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
			Status:         TestStatusPassed,
			FailureKind:    FailureKindNone,
			ArtifactsDir:   turnArtifacts,
			DurationMS:     time.Since(turnStart).Milliseconds(),
		}

		if sseErrorJSON != "" && !tc.Expect.SSEError {
			turnReport.Status = TestStatusError
			turnReport.FailureKind = FailureKindSSEErrorPayload
			turnReport.Error = "sse returned error payload"
			tr.Status = TestStatusError
			tr.FailureKind = FailureKindSSEErrorPayload
			tr.Error = "sse returned error payload"
			tr.Turns = append(tr.Turns, turnReport)
			break
		}

		if turnArtifacts != "" {
			if msgs, err := r.gql.Messages(ctx, sessionID, 500, 0); err == nil {
				if b, err := json.MarshalIndent(msgs, "", "  "); err == nil {
					_ = os.WriteFile(filepath.Join(turnArtifacts, "graphql_messages.json"), b, 0o644)
				}
			}
		}

		if r.judge != nil {
			judgeInput := JudgeTurnInput{
				UserPrompt:        turn.Prompt,
				FinalAnswer:       finalAnswer,
				StreamedAnswer:    sseRes.StreamedContent,
				SSEError:          sseErrorJSON,
				ExpectedChecklist: turn.ExpectedChecklist,
				JudgeInstructions: turn.JudgeInstructions,
				ToolCalls:         toolCalls,
			}

			judgeUserPrompt := buildJudgeUserPrompt(judgeInput)
			var verdict *JudgeVerdict
			if r.cache != nil && r.cache.Enabled() {
				key := r.cache.JudgeKey(r.cfg.JudgeModel, judgeUserPrompt)
				if cached, ok, _ := r.cache.LoadJudgeVerdict(key); ok && cached != nil {
					v := *cached
					verdict = &v
					turnReport.JudgeCached = true
				}
			}
			if verdict == nil {
				v, err := r.judge.Evaluate(ctx, judgeInput)
				if err != nil {
					tr.Status = TestStatusError
					tr.FailureKind = FailureKindJudgeError
					tr.Error = fmt.Sprintf("judge: %v", err)
					turnReport.Status = TestStatusError
					turnReport.FailureKind = FailureKindJudgeError
					turnReport.Error = err.Error()
					tr.Turns = append(tr.Turns, turnReport)
					break
				}
				verdict = v
				if r.cache != nil && r.cache.Enabled() && verdict != nil {
					key := r.cache.JudgeKey(r.cfg.JudgeModel, judgeUserPrompt)
					_ = r.cache.SaveJudgeVerdict(key, *verdict)
				}
			}

			turnReport.Verdict = verdict
			if verdict != nil && !verdict.Passed && tr.Status == TestStatusPassed {
				tr.Status = TestStatusFailed
			}

			if turnArtifacts != "" {
				if b, err := json.MarshalIndent(judgeInput, "", "  "); err == nil {
					_ = os.WriteFile(filepath.Join(turnArtifacts, "judge_input.json"), b, 0o644)
				}
				_ = os.WriteFile(filepath.Join(turnArtifacts, "judge_prompt.txt"), []byte(judgeUserPrompt), 0o644)
				if verdict != nil {
					if b, err := json.MarshalIndent(verdict, "", "  "); err == nil {
						_ = os.WriteFile(filepath.Join(turnArtifacts, "judge_verdict.json"), b, 0o644)
					}
				}
			}
		}

		tr.Turns = append(tr.Turns, turnReport)
		if tr.Status == TestStatusError {
			break
		}
	}

	tr.DurationMS = time.Since(start).Milliseconds()
	if tr.Status != TestStatusPassed {
		tr.FailureKind = firstFailureKind(tr.Turns)
	}
	if testArtifacts != "" {
		if b, err := json.MarshalIndent(tr, "", "  "); err == nil {
			_ = os.WriteFile(filepath.Join(testArtifacts, "result.json"), b, 0o644)
		}
	}
	return tr
}

func (r *Runner) waitForAssistantMessage(ctx context.Context, sessionID uuid.UUID, baselineAssistantCount int) (string, []ToolCall, error) {
	timeout := time.Duration(r.cfg.GraphQLPollTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	interval := time.Duration(r.cfg.GraphQLPollIntervalMillis) * time.Millisecond
	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	deadline := time.Now().Add(timeout)

	for {
		msgs, err := r.gql.Messages(ctx, sessionID, 500, 0)
		if err != nil {
			return "", nil, err
		}
		var assistantCount int
		var lastAssistant *Message
		for i := range msgs {
			if strings.EqualFold(msgs[i].Role, "assistant") {
				assistantCount++
				lastAssistant = &msgs[i]
			}
		}
		if lastAssistant != nil && assistantCount > baselineAssistantCount {
			return lastAssistant.Content, lastAssistant.ToolCalls, nil
		}
		if time.Now().After(deadline) {
			return "", nil, fmt.Errorf("assistant message not persisted within %s", timeout)
		}
		select {
		case <-ctx.Done():
			return "", nil, ctx.Err()
		case <-time.After(interval):
		}
	}
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

func sanitizePathComponent(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, string(os.PathSeparator), "_")
	s = strings.ReplaceAll(s, "..", "_")
	s = strings.ReplaceAll(s, " ", "_")
	if s == "" {
		return "unknown"
	}
	return s
}

func classifyFailure(err error) FailureKind {
	if err == nil {
		return FailureKindNone
	}
	var redir *NotAuthenticatedRedirectError
	if errors.As(err, &redir) {
		return FailureKindNotAuthenticatedRedirect
	}
	var httpStatus *HTTPStatusError
	if errors.As(err, &httpStatus) {
		if httpStatus.StatusCode == http.StatusForbidden {
			return FailureKindForbidden
		}
		return FailureKindHTTPError
	}
	if isForbidden(err) {
		return FailureKindForbidden
	}
	return FailureKindHTTPError
}

func isNotAuthenticatedRedirect(err error) bool {
	var redir *NotAuthenticatedRedirectError
	return errors.As(err, &redir)
}

func isForbidden(err error) bool {
	var httpStatus *HTTPStatusError
	if errors.As(err, &httpStatus) {
		return httpStatus.StatusCode == http.StatusForbidden
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "forbidden") || strings.Contains(msg, "access denied") {
		return true
	}
	return false
}

func firstFailureKind(turns []TurnReport) FailureKind {
	for _, t := range turns {
		if t.FailureKind != FailureKindNone {
			return t.FailureKind
		}
	}
	return FailureKindNone
}
