package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/eval"
	"github.com/iota-uz/iota-sdk/pkg/bichat/eval/dataset"
	"github.com/iota-uz/iota-sdk/pkg/bichat/testharness"
)

type Report struct {
	Timestamp   string                  `json:"timestamp"`
	Summary     ReportSummary           `json:"summary"`
	RunReport   testharness.RunReport   `json:"run_report"`
	OracleFacts map[string]dataset.Fact `json:"oracle_facts,omitempty"`
	Seeded      bool                    `json:"seeded"`
	SeededSets  []string                `json:"seeded_datasets,omitempty"`
}

type ReportSummary struct {
	Total              int     `json:"total"`
	Passed             int     `json:"passed"`
	Failed             int     `json:"failed"`
	Errored            int     `json:"errored"`
	PassRate           float64 `json:"pass_rate"`
	AvgScore           float64 `json:"avg_score"`
	ScoredTurns        int     `json:"scored_turns"`
	CasesPath          string  `json:"cases_path"`
	ServerURL          string  `json:"server_url"`
	RPCEndpointPath    string  `json:"rpc_endpoint_path"`
	StreamEndpointPath string  `json:"stream_endpoint_path"`
	JudgeModel         string  `json:"judge_model"`
	ToolUseEfficiency  int     `json:"tool_use_efficiency"`
	InputTokens        int     `json:"input_tokens"`
	OutputTokens       int     `json:"output_tokens"`
	TotalTokens        int     `json:"total_tokens"`
	Cost               float64 `json:"cost"`
}

type RunOptions struct {
	CasesPath string
	Tag       string
	Category  string
	FailFast  bool

	ServerURL    string
	RPCPath      string
	StreamPath   string
	CookieName   string
	SessionToken string

	JudgeModel   string
	HITLModel    string
	OpenAIAPIKey string

	Seed         bool
	SeedDSN      string
	SeedTenantID string

	ArtifactsDir string
	Parallel     int
}

func LoadCases(path string) ([]eval.TestCase, error) {
	if strings.HasSuffix(strings.ToLower(path), ".json") {
		return eval.LoadTestCases(path)
	}
	return eval.LoadTestCasesFromDir(path)
}

func Run(ctx context.Context, opts RunOptions) (Report, error) {
	if strings.TrimSpace(opts.CasesPath) == "" {
		return Report{}, fmt.Errorf("--cases is required")
	}
	if strings.TrimSpace(opts.SessionToken) == "" {
		return Report{}, fmt.Errorf("--session-token is required")
	}
	if strings.TrimSpace(opts.SeedDSN) == "" {
		return Report{}, fmt.Errorf("--seed-dsn is required")
	}
	if strings.TrimSpace(opts.SeedTenantID) == "" {
		return Report{}, fmt.Errorf("--seed-tenant-id is required")
	}
	apiKey := strings.TrimSpace(opts.OpenAIAPIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	}
	if apiKey == "" {
		return Report{}, fmt.Errorf("openai api key is required")
	}

	tenantID, err := uuid.Parse(strings.TrimSpace(opts.SeedTenantID))
	if err != nil {
		return Report{}, fmt.Errorf("invalid --seed-tenant-id: %w", err)
	}

	registry := dataset.DefaultRegistry()
	seeder, err := dataset.NewSeeder(ctx, opts.SeedDSN, registry)
	if err != nil {
		return Report{}, fmt.Errorf("initialize dataset seeder: %w", err)
	}
	defer seeder.Close()

	harnessCfg := testharness.Config{
		ServerURL:          opts.ServerURL,
		RPCEndpointPath:    opts.RPCPath,
		StreamEndpointPath: opts.StreamPath,
		CookieName:         opts.CookieName,
		SessionToken:       opts.SessionToken,
		JudgeModel:         opts.JudgeModel,
		HITLModel:          opts.HITLModel,
		OpenAIAPIKey:       apiKey,
		Parallel:           opts.Parallel,
		FailFast:           opts.FailFast,
		ArtifactsDir:       opts.ArtifactsDir,
	}

	pipeline := eval.Pipeline{
		CaseSource:      eval.PathCaseSource{Path: opts.CasesPath},
		DatasetPreparer: eval.SeededDatasetPreparer{Seeder: seeder},
		RunnerFactory:   eval.TestHarnessRunnerFactory{},
	}
	execResult, err := pipeline.Execute(ctx, eval.ExecuteRequest{
		Tag:           opts.Tag,
		Category:      opts.Category,
		SeedTenantID:  tenantID,
		Seed:          opts.Seed,
		HarnessConfig: harnessCfg,
	})
	if err != nil {
		return Report{}, err
	}

	return buildReport(opts, execResult.DatasetIDs, execResult.OracleFacts, execResult.RunReport), nil
}

func WriteReport(outPath string, rep Report) error {
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}

	if strings.TrimSpace(outPath) == "" {
		_, err = os.Stdout.Write(append(data, '\n'))
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outPath, data, 0o644)
}

func buildReport(opts RunOptions, datasetIDs []string, oracleFacts map[string]dataset.Fact, runReport testharness.RunReport) Report {
	total := runReport.Summary.Total
	passed := runReport.Summary.Passed
	failed := runReport.Summary.Failed
	errored := runReport.Summary.Errored

	passRate := 0.0
	if total > 0 {
		passRate = float64(passed) / float64(total)
	}

	scoredTurns := 0
	totalScore := 0.0
	for _, tr := range runReport.Tests {
		for _, turn := range tr.Turns {
			if turn.Verdict == nil {
				continue
			}
			totalScore += turn.Verdict.Score
			scoredTurns++
		}
	}
	avgScore := 0.0
	if scoredTurns > 0 {
		avgScore = totalScore / float64(scoredTurns)
	}

	return Report{
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		RunReport:   runReport,
		OracleFacts: oracleFacts,
		Seeded:      opts.Seed,
		SeededSets:  datasetIDs,
		Summary: ReportSummary{
			Total:              total,
			Passed:             passed,
			Failed:             failed,
			Errored:            errored,
			PassRate:           passRate,
			AvgScore:           avgScore,
			ScoredTurns:        scoredTurns,
			CasesPath:          opts.CasesPath,
			ServerURL:          opts.ServerURL,
			RPCEndpointPath:    opts.RPCPath,
			StreamEndpointPath: opts.StreamPath,
			JudgeModel:         opts.JudgeModel,
			ToolUseEfficiency:  runReport.Summary.ToolUseEfficiency,
			InputTokens:        runReport.Summary.InputTokens,
			OutputTokens:       runReport.Summary.OutputTokens,
			TotalTokens:        runReport.Summary.TotalTokens,
			Cost:               runReport.Summary.Cost,
		},
	}
}
