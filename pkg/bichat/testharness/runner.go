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
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
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

type EvalMetrics struct {
	// ToolUseEfficiency is the number of tool calls used to produce an answer.
	ToolUseEfficiency int `json:"tool_use_efficiency"`
	UniqueToolsUsed   int `json:"unique_tools_used"`

	// InputTokens/OutputTokens are prompt/completion tokens for the eval.
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`

	// Cost is total estimated/actual USD cost for the eval.
	Cost float64 `json:"cost"`

	AssistantInputTokens  int     `json:"assistant_input_tokens,omitempty"`
	AssistantOutputTokens int     `json:"assistant_output_tokens,omitempty"`
	AssistantTotalTokens  int     `json:"assistant_total_tokens,omitempty"`
	AssistantCost         float64 `json:"assistant_cost,omitempty"`

	JudgeInputTokens  int     `json:"judge_input_tokens,omitempty"`
	JudgeOutputTokens int     `json:"judge_output_tokens,omitempty"`
	JudgeTotalTokens  int     `json:"judge_total_tokens,omitempty"`
	JudgeCost         float64 `json:"judge_cost,omitempty"`
}

type TurnReport struct {
	Prompt         string        `json:"prompt"`
	StreamedAnswer string        `json:"streamed_answer,omitempty"`
	FinalAnswer    string        `json:"final_answer,omitempty"`
	SSEErrorJSON   string        `json:"sse_error_json,omitempty"`
	ToolCalls      []ToolCall    `json:"tool_calls,omitempty"`
	Metrics        EvalMetrics   `json:"metrics"`
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
	Metrics      EvalMetrics  `json:"metrics"`
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

	ToolUseEfficiency int     `json:"tool_use_efficiency"`
	InputTokens       int     `json:"input_tokens"`
	OutputTokens      int     `json:"output_tokens"`
	TotalTokens       int     `json:"total_tokens"`
	Cost              float64 `json:"cost"`
}

type Runner struct {
	cfg   Config
	rpc   *RPCClient
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
		rpc:   NewRPCClient(cfg),
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

	if r.cfg.FailFast {
		for i, tc := range suite.Tests {
			tr := r.runOneTest(ctx, tc)
			report.Tests[i] = tr
			if tr.Status != TestStatusPassed {
				report.Tests = report.Tests[:i+1]
				break
			}
		}
		report.Summary = summarize(report.Tests)
		return report, exitCode(report), nil
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

	sessionID, err := r.rpc.CreateSession(ctx, "")
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
		if session, msgErr := r.rpc.GetSession(ctx, sessionID); msgErr == nil && session != nil {
			baselineAssistantCount = countAssistantTurns(session.Turns)
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

		sseRes, err := r.sse.StreamMessage(ctx, sessionID, turn.Prompt, StreamSinks{Raw: sseRawFile, JSON: sseJSONFile})
		// Close turn files immediately after streaming completes
		closeTurnFiles(sseRawFile, sseJSONFile)
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

		finalAnswer, toolCalls, rpcUsage, sessionSnapshot, waitErr := r.waitForAssistantMessage(ctx, sessionID, baselineAssistantCount)
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
		effectiveStreamRes := withFallbackUsage(sseRes, rpcUsage)
		streamedAnswer := ""
		if sseRes != nil {
			streamedAnswer = sseRes.StreamedContent
		}

		var sseErrorJSON string
		if sseRes != nil && sseRes.ErrorPayload != nil {
			if b, err := json.Marshal(sseRes.ErrorPayload); err == nil {
				sseErrorJSON = string(b)
			}
		}

		turnReport := TurnReport{
			Prompt:         turn.Prompt,
			StreamedAnswer: streamedAnswer,
			FinalAnswer:    finalAnswer,
			SSEErrorJSON:   sseErrorJSON,
			ToolCalls:      toolCalls,
			Metrics:        buildEvalMetrics(toolCalls, effectiveStreamRes, nil),
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

		if turnArtifacts != "" && sessionSnapshot != nil {
			if b, err := json.MarshalIndent(sessionSnapshot, "", "  "); err == nil {
				_ = os.WriteFile(filepath.Join(turnArtifacts, "rpc_session.json"), b, 0o644)
			}
		}

		if r.judge != nil {
			oracleRefs := append([]string{}, tc.OracleRefs...)
			oracleRefs = append(oracleRefs, turn.OracleRefs...)
			oracleFacts, oracleErr := r.resolveOracleFacts(oracleRefs)
			if oracleErr != nil {
				tr.Status = TestStatusError
				tr.FailureKind = FailureKindJudgeError
				tr.Error = fmt.Sprintf("resolve oracle refs: %v", oracleErr)
				turnReport.Status = TestStatusError
				turnReport.FailureKind = FailureKindJudgeError
				turnReport.Error = oracleErr.Error()
				tr.Turns = append(tr.Turns, turnReport)
				break
			}

			judgeInput := JudgeTurnInput{
				UserPrompt:        turn.Prompt,
				FinalAnswer:       finalAnswer,
				StreamedAnswer:    streamedAnswer,
				SSEError:          sseErrorJSON,
				ExpectedChecklist: turn.ExpectedChecklist,
				JudgeInstructions: turn.JudgeInstructions,
				ToolCalls:         toolCalls,
				OracleFacts:       oracleFacts,
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
			turnReport.Metrics = buildEvalMetrics(toolCalls, effectiveStreamRes, verdict)
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
	tr.Metrics = aggregateEvalMetricsFromTurns(tr.Turns)
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

func (r *Runner) waitForAssistantMessage(ctx context.Context, sessionID uuid.UUID, baselineAssistantCount int) (string, []ToolCall, *types.DebugUsage, *RPCSession, error) {
	timeout := time.Duration(r.cfg.RPCPollTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	interval := time.Duration(r.cfg.RPCPollIntervalMillis) * time.Millisecond
	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	deadline := time.Now().Add(timeout)

	for {
		session, err := r.rpc.GetSession(ctx, sessionID)
		if err != nil {
			return "", nil, nil, nil, err
		}
		assistantCount, lastAssistant := latestAssistantTurn(session.Turns)
		if lastAssistant != nil && assistantCount > baselineAssistantCount {
			var usage *types.DebugUsage
			if lastAssistant.Debug != nil {
				usage = lastAssistant.Debug.Usage.ToDebugUsage()
			}
			return lastAssistant.Content, lastAssistant.ToolCalls, usage, session, nil
		}
		if time.Now().After(deadline) {
			return "", nil, nil, session, fmt.Errorf("assistant message not persisted within %s", timeout)
		}
		select {
		case <-ctx.Done():
			return "", nil, nil, session, ctx.Err()
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
		s.ToolUseEfficiency += t.Metrics.ToolUseEfficiency
		s.InputTokens += t.Metrics.InputTokens
		s.OutputTokens += t.Metrics.OutputTokens
		s.TotalTokens += t.Metrics.TotalTokens
		s.Cost += t.Metrics.Cost
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
	// Replace both path separators for cross-platform safety
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
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
	var rpcErr *RPCMethodError
	if errors.As(err, &rpcErr) {
		if strings.EqualFold(strings.TrimSpace(rpcErr.Code), "forbidden") {
			return FailureKindForbidden
		}
		return FailureKindHTTPError
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
	var rpcErr *RPCMethodError
	if errors.As(err, &rpcErr) {
		return strings.EqualFold(strings.TrimSpace(rpcErr.Code), "forbidden")
	}
	var httpStatus *HTTPStatusError
	if errors.As(err, &httpStatus) {
		return httpStatus.StatusCode == http.StatusForbidden
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "forbidden") || strings.Contains(msg, "access denied") || strings.Contains(msg, "permission denied") {
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

func withFallbackUsage(sseRes *StreamResult, usage *types.DebugUsage) *StreamResult {
	if usage == nil {
		return sseRes
	}
	if sseRes == nil {
		return &StreamResult{Usage: usage}
	}
	if sseRes.Usage != nil {
		return sseRes
	}
	copied := *sseRes
	copied.Usage = usage
	return &copied
}

func countAssistantTurns(turns []RPCConversationTurn) int {
	count := 0
	for i := range turns {
		if isAssistantTurn(turns[i].AssistantTurn) {
			count++
		}
	}
	return count
}

func latestAssistantTurn(turns []RPCConversationTurn) (int, *RPCAssistantTurn) {
	count := 0
	var last *RPCAssistantTurn
	for i := range turns {
		assistant := turns[i].AssistantTurn
		if !isAssistantTurn(assistant) {
			continue
		}
		count++
		last = assistant
	}
	return count, last
}

func isAssistantTurn(turn *RPCAssistantTurn) bool {
	if turn == nil {
		return false
	}
	role := strings.ToLower(strings.TrimSpace(turn.Role))
	return role == "" || role == "assistant"
}

func (r *Runner) resolveOracleFacts(refs []string) ([]OracleFact, error) {
	if len(refs) == 0 {
		return nil, nil
	}

	facts := make([]OracleFact, 0, len(refs))
	seen := make(map[string]struct{}, len(refs))

	for _, ref := range refs {
		key := strings.TrimSpace(ref)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		fact, ok := r.cfg.OracleFacts[key]
		if !ok {
			return nil, fmt.Errorf("oracle reference %q not found", key)
		}
		facts = append(facts, fact)
	}

	return facts, nil
}

func buildEvalMetrics(toolCalls []ToolCall, sseRes *StreamResult, verdict *JudgeVerdict) EvalMetrics {
	metrics := EvalMetrics{
		ToolUseEfficiency: len(toolCalls),
		UniqueToolsUsed:   countUniqueTools(toolCalls),
	}

	if sseRes != nil && sseRes.Usage != nil {
		metrics.AssistantInputTokens = sseRes.Usage.PromptTokens
		metrics.AssistantOutputTokens = sseRes.Usage.CompletionTokens
		metrics.AssistantTotalTokens = sseRes.Usage.TotalTokens
		metrics.AssistantCost = sseRes.Usage.Cost
	}

	if verdict != nil && verdict.Usage != nil {
		metrics.JudgeInputTokens = verdict.Usage.PromptTokens
		metrics.JudgeOutputTokens = verdict.Usage.CompletionTokens
		metrics.JudgeTotalTokens = verdict.Usage.TotalTokens
		metrics.JudgeCost = verdict.Usage.Cost
	}

	metrics.InputTokens = metrics.AssistantInputTokens + metrics.JudgeInputTokens
	metrics.OutputTokens = metrics.AssistantOutputTokens + metrics.JudgeOutputTokens
	metrics.TotalTokens = metrics.AssistantTotalTokens + metrics.JudgeTotalTokens
	metrics.Cost = metrics.AssistantCost + metrics.JudgeCost

	return metrics
}

func aggregateEvalMetricsFromTurns(turns []TurnReport) EvalMetrics {
	agg := EvalMetrics{}
	for _, turn := range turns {
		agg.ToolUseEfficiency += turn.Metrics.ToolUseEfficiency
		agg.UniqueToolsUsed += turn.Metrics.UniqueToolsUsed
		agg.InputTokens += turn.Metrics.InputTokens
		agg.OutputTokens += turn.Metrics.OutputTokens
		agg.TotalTokens += turn.Metrics.TotalTokens
		agg.Cost += turn.Metrics.Cost

		agg.AssistantInputTokens += turn.Metrics.AssistantInputTokens
		agg.AssistantOutputTokens += turn.Metrics.AssistantOutputTokens
		agg.AssistantTotalTokens += turn.Metrics.AssistantTotalTokens
		agg.AssistantCost += turn.Metrics.AssistantCost

		agg.JudgeInputTokens += turn.Metrics.JudgeInputTokens
		agg.JudgeOutputTokens += turn.Metrics.JudgeOutputTokens
		agg.JudgeTotalTokens += turn.Metrics.JudgeTotalTokens
		agg.JudgeCost += turn.Metrics.JudgeCost
	}
	return agg
}

func countUniqueTools(toolCalls []ToolCall) int {
	if len(toolCalls) == 0 {
		return 0
	}
	seen := make(map[string]struct{}, len(toolCalls))
	for _, tc := range toolCalls {
		key := strings.TrimSpace(tc.Name)
		if key == "" {
			key = strings.TrimSpace(tc.Arguments)
		}
		if key == "" {
			continue
		}
		seen[key] = struct{}{}
	}
	return len(seen)
}

// closeTurnFiles closes the SSE artifact files for a turn, ignoring any errors.
func closeTurnFiles(sseRawFile, sseJSONFile *os.File) {
	if sseRawFile != nil {
		_ = sseRawFile.Close()
	}
	if sseJSONFile != nil {
		_ = sseJSONFile.Close()
	}
}
