package commands

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/llmproviders"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/eval"
	evalcli "github.com/iota-uz/iota-sdk/pkg/bichat/eval/cli"
	"github.com/iota-uz/iota-sdk/pkg/cli/exitcode"
)

func NewBiChatEvalCommand() *cobra.Command {
	evalCmd := &cobra.Command{
		Use:   "eval",
		Short: "BiChat evaluation tooling",
		Long:  "Run and inspect BiChat evaluation test cases and quality gates.",
	}

	evalCmd.AddCommand(newBiChatEvalRunCmd())
	evalCmd.AddCommand(newBiChatEvalListCmd())

	return evalCmd
}

func newBiChatEvalRunCmd() *cobra.Command {
	var (
		casesPath string
		tag       string
		category  string
		runner    string
		judge     string
		reportOut string
		failFast  bool
		minPass   float64
		minAvg    float64
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run eval cases and emit a JSON report",
		Long:  "Runs BiChat eval test cases, writes a JSON report, and exits non-zero on quality regression.",
		Args:  cobra.NoArgs,
		Example: `  # Offline smoke evals (fixture runner)
  command bichat eval run --cases ./pkg/bichat/eval/testdata/smoke --runner fixture --judge none --report ./coverage/bichat_eval_report.json

  # OpenAI runner + OpenAI judge (requires OPENAI_API_KEY)
  command bichat eval run --cases ./pkg/bichat/eval/testdata/smoke --runner openai --judge openai --report ./coverage/bichat_eval_report.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(casesPath) == "" {
				return exitcode.InvalidUsage(fmt.Errorf("--cases is required"))
			}
			if minPass < 0 || minPass > 1 {
				return exitcode.InvalidUsage(fmt.Errorf("--min-pass-rate must be between 0.0 and 1.0"))
			}
			if minAvg < 0 || minAvg > 1 {
				return exitcode.InvalidUsage(fmt.Errorf("--min-avg-score must be between 0.0 and 1.0"))
			}
			switch strings.ToLower(strings.TrimSpace(runner)) {
			case "fixture", "openai":
				// ok
			default:
				return exitcode.InvalidUsage(fmt.Errorf("--runner must be one of: fixture|openai"))
			}
			switch strings.ToLower(strings.TrimSpace(judge)) {
			case "none", "openai":
				// ok
			default:
				return exitcode.InvalidUsage(fmt.Errorf("--judge must be one of: none|openai"))
			}

			rep, err := evalcli.Run(cmd.Context(), evalcli.RunOptions{
				CasesPath: casesPath,
				Tag:       tag,
				Category:  category,
				Runner:    strings.ToLower(strings.TrimSpace(runner)),
				Judge:     strings.ToLower(strings.TrimSpace(judge)),
				FailFast:  failFast,
				NewOpenAIModel: func() (agents.Model, error) { return llmproviders.NewOpenAIModel() },
			})
			if err != nil {
				return exitcode.New(exitcode.InvalidUsageCode, err)
			}

			if err := evalcli.WriteReport(reportOut, rep); err != nil {
				return exitcode.New(exitcode.InvalidUsageCode, err)
			}

			// Exit non-zero on quality regression.
			if rep.Summary.PassRate < minPass || rep.Summary.AvgScore < minAvg || rep.Summary.Failed > 0 {
				return exitcode.SilentCode(exitcode.QualityRegressionCode)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&casesPath, "cases", "pkg/bichat/eval/testdata/smoke", "Path to test cases file (.json) or directory")
	cmd.Flags().StringVar(&tag, "tag", "", "Filter cases by tag")
	cmd.Flags().StringVar(&category, "category", "", "Filter cases by category")
	cmd.Flags().StringVar(&runner, "runner", "fixture", "Runner mode: fixture|openai")
	cmd.Flags().StringVar(&judge, "judge", "none", "Judge mode: none|openai (uses LLMGradeChecker)")
	cmd.Flags().StringVar(&reportOut, "report", "", "Write JSON report to this path (prints to stdout if empty)")
	cmd.Flags().BoolVar(&failFast, "fail-fast", false, "Stop on first failing test case")
	cmd.Flags().Float64Var(&minPass, "min-pass-rate", 1.0, "Minimum pass rate required (0.0-1.0)")
	cmd.Flags().Float64Var(&minAvg, "min-avg-score", 1.0, "Minimum average score required (0.0-1.0)")

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
		Short: "List available eval cases",
		Long:  "Loads eval test cases from a file/directory and prints a case inventory.",
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

				fmt.Fprintf(cmd.OutOrStdout(), "id\tcategory\ttags\tquestion\n")
				for _, ci := range infos {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", ci.ID, ci.Category, strings.Join(ci.Tags, ","), ci.Question)
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

	cmd.Flags().StringVar(&casesPath, "cases", "pkg/bichat/eval/testdata/smoke", "Path to test cases file (.json) or directory")
	cmd.Flags().StringVar(&tag, "tag", "", "Filter cases by tag")
	cmd.Flags().StringVar(&category, "category", "", "Filter cases by category")
	cmd.Flags().StringVar(&format, "format", "text", "Output format: text|json")

	return cmd
}

type caseInfo struct {
	ID       string   `json:"id"`
	Category string   `json:"category,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Question string   `json:"question,omitempty"`
}

func caseInfoFrom(tc eval.TestCase) caseInfo {
	return caseInfo{
		ID:       tc.ID,
		Category: tc.Category,
		Tags:     tc.Tags,
		Question: tc.Question,
	}
}
