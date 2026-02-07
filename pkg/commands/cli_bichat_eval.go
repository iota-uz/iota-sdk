package commands

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iota-uz/iota-sdk/pkg/bichat/eval"
	evalcli "github.com/iota-uz/iota-sdk/pkg/bichat/eval/cli"
	"github.com/iota-uz/iota-sdk/pkg/cli/exitcode"
)

func NewBiChatEvalCommand() *cobra.Command {
	evalCmd := &cobra.Command{
		Use:   "eval",
		Short: "BiChat analytics evaluation tooling",
		Long:  "Run and inspect analytics-oriented BiChat evaluation test cases with mandatory LLM judging.",
	}

	evalCmd.AddCommand(newBiChatEvalRunCmd())
	evalCmd.AddCommand(newBiChatEvalListCmd())

	return evalCmd
}

func newBiChatEvalRunCmd() *cobra.Command {
	var (
		casesPath    string
		tag          string
		category     string
		reportOut    string
		failFast     bool
		minPass      float64
		minAvg       float64
		serverURL    string
		rpcPath      string
		streamPath   string
		cookieName   string
		sessionToken string
		judgeModel   string
		hitlModel    string
		openAIAPIKey string
		seed         bool
		seedDSN      string
		seedTenantID string
		artifactsDir string
		parallel     int
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run analytics eval cases and emit a JSON report",
		Long:  "Runs analytics-oriented BiChat eval test cases over live HTTP/SSE endpoints and exits non-zero on quality regression.",
		Args:  cobra.NoArgs,
		Example: `  # Live analytics eval with seeded deterministic dataset
  command bichat eval run \
    --cases ./pkg/bichat/eval/testdata/analytics/suite.json \
    --server-url http://127.0.0.1:3200 \
    --rpc-path /bi-chat/rpc \
    --stream-path /bi-chat/stream \
    --session-token '<granite_sid>' \
    --openai-api-key "$OPENAI_API_KEY" \
    --seed-dsn 'postgres://postgres:postgres@localhost:5432/iota?sslmode=disable' \
    --seed-tenant-id '00000000-0000-0000-0000-000000000001' \
    --report ./coverage/bichat_eval_report.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(casesPath) == "" {
				return exitcode.InvalidUsage(fmt.Errorf("--cases is required"))
			}
			if strings.TrimSpace(sessionToken) == "" {
				return exitcode.InvalidUsage(fmt.Errorf("--session-token is required"))
			}
			if strings.TrimSpace(seedDSN) == "" {
				return exitcode.InvalidUsage(fmt.Errorf("--seed-dsn is required"))
			}
			if strings.TrimSpace(seedTenantID) == "" {
				return exitcode.InvalidUsage(fmt.Errorf("--seed-tenant-id is required"))
			}
			if strings.TrimSpace(openAIAPIKey) == "" {
				return exitcode.InvalidUsage(fmt.Errorf("--openai-api-key (or OPENAI_API_KEY) is required"))
			}
			if minPass < 0 || minPass > 1 {
				return exitcode.InvalidUsage(fmt.Errorf("--min-pass-rate must be between 0.0 and 1.0"))
			}
			if minAvg < 0 || minAvg > 1 {
				return exitcode.InvalidUsage(fmt.Errorf("--min-avg-score must be between 0.0 and 1.0"))
			}
			if parallel < 0 {
				return exitcode.InvalidUsage(fmt.Errorf("--parallel must be >= 0"))
			}

			rep, err := evalcli.Run(cmd.Context(), evalcli.RunOptions{
				CasesPath:    casesPath,
				Tag:          tag,
				Category:     category,
				FailFast:     failFast,
				ServerURL:    strings.TrimSpace(serverURL),
				RPCPath:      strings.TrimSpace(rpcPath),
				StreamPath:   strings.TrimSpace(streamPath),
				CookieName:   strings.TrimSpace(cookieName),
				SessionToken: strings.TrimSpace(sessionToken),
				JudgeModel:   strings.TrimSpace(judgeModel),
				HITLModel:    strings.TrimSpace(hitlModel),
				OpenAIAPIKey: strings.TrimSpace(openAIAPIKey),
				Seed:         seed,
				SeedDSN:      strings.TrimSpace(seedDSN),
				SeedTenantID: strings.TrimSpace(seedTenantID),
				ArtifactsDir: strings.TrimSpace(artifactsDir),
				Parallel:     parallel,
			})
			if err != nil {
				return exitcode.New(exitcode.InvalidUsageCode, err)
			}

			if err := evalcli.WriteReport(reportOut, rep); err != nil {
				return exitcode.New(exitcode.InvalidUsageCode, err)
			}

			if rep.Summary.PassRate < minPass || rep.Summary.AvgScore < minAvg || rep.Summary.Failed > 0 || rep.Summary.Errored > 0 {
				return exitcode.SilentCode(exitcode.QualityRegressionCode)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&casesPath, "cases", "pkg/bichat/eval/testdata/analytics/suite.json", "Path to analytics suite file (.json) or directory")
	cmd.Flags().StringVar(&tag, "tag", "", "Filter cases by tag")
	cmd.Flags().StringVar(&category, "category", "", "Filter cases by category")
	cmd.Flags().StringVar(&serverURL, "server-url", "http://127.0.0.1:3200", "BiChat server base URL")
	cmd.Flags().StringVar(&rpcPath, "rpc-path", "/bi-chat/rpc", "Applet HTTP RPC endpoint path")
	cmd.Flags().StringVar(&streamPath, "stream-path", "/bi-chat/stream", "SSE stream endpoint path")
	cmd.Flags().StringVar(&cookieName, "cookie-name", "granite_sid", "Session cookie name")
	cmd.Flags().StringVar(&sessionToken, "session-token", "", "Authenticated session token (cookie value)")
	cmd.Flags().StringVar(&judgeModel, "judge-model", "gpt-5-mini", "OpenAI judge model")
	cmd.Flags().StringVar(&hitlModel, "hitl-model", "gpt-4o-mini", "OpenAI model used to answer HITL clarification questions")
	cmd.Flags().StringVar(&openAIAPIKey, "openai-api-key", "", "OpenAI API key (required; falls back to OPENAI_API_KEY)")
	cmd.Flags().BoolVar(&seed, "seed", true, "Seed deterministic analytics data before running evals")
	cmd.Flags().StringVar(&seedDSN, "seed-dsn", "", "PostgreSQL DSN for seeding and oracle computation")
	cmd.Flags().StringVar(&seedTenantID, "seed-tenant-id", "", "Tenant UUID for seeded analytics data")
	cmd.Flags().StringVar(&artifactsDir, "artifacts-dir", "", "Directory for per-run and per-turn artifacts")
	cmd.Flags().IntVar(&parallel, "parallel", 0, "Parallel workers (0 uses default)")
	cmd.Flags().StringVar(&reportOut, "report", "", "Write JSON report to this path (prints to stdout if empty)")
	cmd.Flags().BoolVar(&failFast, "fail-fast", false, "Stop on first failing/errored test case")
	cmd.Flags().Float64Var(&minPass, "min-pass-rate", 1.0, "Minimum pass rate required (0.0-1.0)")
	cmd.Flags().Float64Var(&minAvg, "min-avg-score", 0.8, "Minimum average LLM judge score required (0.0-1.0)")

	return cmd
}

func newBiChatEvalListCmd() *cobra.Command {
	var (
		casesPath string
		tag       string
		category  string
		format    string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available analytics eval cases",
		Long:  "Loads analytics eval test cases from a file/directory and prints a case inventory.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cases, err := evalcli.LoadCases(casesPath)
			if err != nil {
				return exitcode.New(exitcode.InvalidUsageCode, err)
			}
			if tag != "" {
				cases = eval.FilterByTag(cases, tag)
			}
			if category != "" {
				cases = eval.FilterByCategory(cases, category)
			}
			if len(cases) == 0 {
				return exitcode.New(exitcode.InvalidUsageCode, fmt.Errorf("no test cases to list after filtering"))
			}

			switch strings.ToLower(strings.TrimSpace(format)) {
			case "", "text":
				infos := make([]caseInfo, 0, len(cases))
				for _, tc := range cases {
					infos = append(infos, caseInfoFrom(tc))
				}
				sort.Slice(infos, func(i, j int) bool { return infos[i].ID < infos[j].ID })

				fmt.Fprintf(cmd.OutOrStdout(), "id\tdataset\tcategory\ttags\tfirst_prompt\n")
				for _, ci := range infos {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\t%s\n", ci.ID, ci.DatasetID, ci.Category, strings.Join(ci.Tags, ","), ci.FirstPrompt)
				}
				return nil

			case "json":
				infos := make([]caseInfo, 0, len(cases))
				for _, tc := range cases {
					infos = append(infos, caseInfoFrom(tc))
				}
				sort.Slice(infos, func(i, j int) bool { return infos[i].ID < infos[j].ID })

				data, err := json.MarshalIndent(infos, "", "  ")
				if err != nil {
					return exitcode.New(exitcode.InvalidUsageCode, err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil

			default:
				return exitcode.InvalidUsage(fmt.Errorf("unknown --format: %s (expected text|json)", format))
			}
		},
	}

	cmd.Flags().StringVar(&casesPath, "cases", "pkg/bichat/eval/testdata/analytics/suite.json", "Path to analytics suite file (.json) or directory")
	cmd.Flags().StringVar(&tag, "tag", "", "Filter cases by tag")
	cmd.Flags().StringVar(&category, "category", "", "Filter cases by category")
	cmd.Flags().StringVar(&format, "format", "text", "Output format: text|json")

	return cmd
}

type caseInfo struct {
	ID          string   `json:"id"`
	DatasetID   string   `json:"dataset_id"`
	Category    string   `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	FirstPrompt string   `json:"first_prompt,omitempty"`
}

func caseInfoFrom(tc eval.TestCase) caseInfo {
	firstPrompt := ""
	if len(tc.Turns) > 0 {
		firstPrompt = tc.Turns[0].Prompt
	}

	return caseInfo{
		ID:          tc.ID,
		DatasetID:   tc.DatasetID,
		Category:    tc.Category,
		Tags:        tc.Tags,
		FirstPrompt: firstPrompt,
	}
}
