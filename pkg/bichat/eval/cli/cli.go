package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/llmproviders"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/eval"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

type Report struct {
	Timestamp string            `json:"timestamp"`
	Summary   ReportSummary     `json:"summary"`
	Results   []eval.EvalResult `json:"results"`
}

type ReportSummary struct {
	Total     int     `json:"total"`
	Passed    int     `json:"passed"`
	Failed    int     `json:"failed"`
	PassRate  float64 `json:"pass_rate"`
	AvgScore  float64 `json:"avg_score"`
	Runner    string  `json:"runner"`
	Judge     string  `json:"judge"`
	CasesPath string  `json:"cases_path"`
}

type RunOptions struct {
	CasesPath string
	Tag       string
	Category  string
	Runner    string
	Judge     string
	FailFast  bool
}

func LoadCases(path string) ([]eval.TestCase, error) {
	if strings.HasSuffix(strings.ToLower(path), ".json") {
		return eval.LoadTestCases(path)
	}
	return eval.LoadTestCasesFromDir(path)
}

func BuildRunnerAndJudge(
	ctx context.Context,
	runnerMode string,
	judgeMode string,
	cases []eval.TestCase,
) (eval.AgentRunner, agents.Model, error) {
	var judgeModel agents.Model

	switch judgeMode {
	case "none":
		// no-op
	case "openai":
		m, err := llmproviders.NewOpenAIModel()
		if err != nil {
			return nil, nil, err
		}
		judgeModel = m
	default:
		return nil, nil, fmt.Errorf("unknown judge mode: %s", judgeMode)
	}

	switch runnerMode {
	case "fixture":
		byQuestion := make(map[string]eval.TestCase, len(cases))
		for _, tc := range cases {
			byQuestion[tc.Question] = tc
		}
		return func(ctx context.Context, question string) (string, error) {
			tc, ok := byQuestion[question]
			if !ok {
				return "I don't know.", nil
			}
			if strings.TrimSpace(tc.GoldenAnswer) != "" {
				return tc.GoldenAnswer, nil
			}
			if strings.TrimSpace(tc.ExpectedSQL) != "" {
				return fmt.Sprintf("```sql\n%s\n```", tc.ExpectedSQL), nil
			}
			if len(tc.ExpectedContent) > 0 {
				return strings.Join(tc.ExpectedContent, " "), nil
			}
			return "OK", nil
		}, judgeModel, nil

	case "openai":
		m, err := llmproviders.NewOpenAIModel()
		if err != nil {
			return nil, nil, err
		}
		return func(ctx context.Context, question string) (string, error) {
			system := "You are a BI assistant under evaluation. Answer the question directly.\n\n" +
				"If SQL is required, output a single SQL query in a fenced ```sql code block.\n" +
				"Avoid explanations unless asked."
			req := agents.Request{
				Messages: []types.Message{
					types.SystemMessage(system),
					types.UserMessage(question),
				},
			}
			temp := 0.2
			resp, err := m.Generate(ctx, req, agents.WithTemperature(temp))
			if err != nil {
				return "", err
			}
			return resp.Message.Content(), nil
		}, judgeModel, nil

	default:
		return nil, nil, fmt.Errorf("unknown runner mode: %s", runnerMode)
	}
}

func Run(ctx context.Context, opts RunOptions) (Report, error) {
	cases, err := LoadCases(opts.CasesPath)
	if err != nil {
		return Report{}, err
	}
	if opts.Tag != "" {
		cases = eval.FilterByTag(cases, opts.Tag)
	}
	if opts.Category != "" {
		cases = eval.FilterByCategory(cases, opts.Category)
	}
	if len(cases) == 0 {
		return Report{}, fmt.Errorf("no test cases to run after filtering")
	}

	runnerFn, judgeModel, err := BuildRunnerAndJudge(ctx, opts.Runner, opts.Judge, cases)
	if err != nil {
		return Report{}, err
	}

	checkers := []eval.Checker{
		eval.NewStringMatchChecker(),
		eval.NewSQLResultChecker(),
	}
	if opts.Judge != "none" && judgeModel != nil {
		checkers = append(checkers, eval.NewLLMGradeChecker(judgeModel))
	}

	evaluator := eval.NewEvaluator(runnerFn, checkers...)

	results := make([]eval.EvalResult, 0, len(cases))
	for _, tc := range cases {
		res, runErr := evaluator.RunSingle(ctx, tc)
		if runErr != nil {
			return Report{}, runErr
		}
		results = append(results, *res)
		if opts.FailFast && !res.Passed {
			break
		}
	}

	return buildReport(opts.CasesPath, opts.Runner, opts.Judge, results), nil
}

func WriteReport(outPath string, rep Report) error {
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}

	if strings.TrimSpace(outPath) == "" {
		fmt.Println(string(data))
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outPath, data, 0o644)
}

func buildReport(casesPath, runner, judge string, results []eval.EvalResult) Report {
	passed := 0
	totalScore := 0.0
	for _, r := range results {
		if r.Passed {
			passed++
		}
		totalScore += r.Score
	}
	total := len(results)
	failed := total - passed
	passRate := 0.0
	avg := 0.0
	if total > 0 {
		passRate = float64(passed) / float64(total)
		avg = totalScore / float64(total)
	}

	return Report{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Summary: ReportSummary{
			Total:     total,
			Passed:    passed,
			Failed:    failed,
			PassRate:  passRate,
			AvgScore:  avg,
			Runner:    runner,
			Judge:     judge,
			CasesPath: casesPath,
		},
		Results: results,
	}
}
