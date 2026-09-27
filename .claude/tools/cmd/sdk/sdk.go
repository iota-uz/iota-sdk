package sdk

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	core "github.com/iota-uz/iota-sdk/sdk-tools/internal/sdk"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	command := &cobra.Command{Use: "sdk", Short: "Preview, request and consume verified SDK releases"}
	runner := core.ExecRunner{}
	upstream := core.GitHub{Runner: runner, Repo: core.Repository}
	var goDir, webDir string
	use := &cobra.Command{Use: "use <SDK PR number>", Args: cobra.ExactArgs(1), Short: "Declare an SDK dependency and prepare an ignored local preview", RunE: func(cmd *cobra.Command, args []string) error {
		number, err := strconv.Atoi(args[0])
		if err != nil {
			return err
		}
		root, err := repositoryRoot(cmd, runner)
		if err != nil {
			return err
		}
		d := core.Dependency{PR: number, Channel: "preview", GoDir: goDir, WebDir: webDir}
		if err = core.WriteDependency(root, d); err != nil {
			return err
		}
		if err = upstream.Preview(cmd.Context(), root, d); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Preview ready. Commit .sdk/dependency.json with your consumer changes.")
		return nil
	}}
	use.Flags().StringVar(&goDir, "go-dir", ".", "Go module directory relative to the consumer root")
	use.Flags().StringVar(&webDir, "web-dir", "", "Optional standalone pnpm consumer directory")
	command.AddCommand(use)
	command.AddCommand(&cobra.Command{Use: "preview", Args: cobra.NoArgs, Short: "Prepare the preview declared in .sdk/dependency.json", RunE: func(cmd *cobra.Command, _ []string) error {
		root, err := repositoryRoot(cmd, runner)
		if err != nil {
			return err
		}
		d, err := core.ReadDependency(root)
		if err != nil {
			return err
		}
		return upstream.Preview(cmd.Context(), root, d)
	}})
	var promoteRetry bool
	promote := &cobra.Command{Use: "promote", Args: cobra.NoArgs, Short: "Wait for a verified SDK release and update local production dependencies", RunE: func(cmd *cobra.Command, _ []string) error {
		root, err := repositoryRoot(cmd, runner)
		if err != nil {
			return err
		}
		if err = upstream.Promote(cmd.Context(), root, promoteRetry, cmd.OutOrStdout()); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Verified release pinned locally. Review, commit and push the dependency changes.")
		return nil
	}}
	promote.Flags().BoolVar(&promoteRetry, "retry", false, "Retry failed verification after fixing infrastructure")
	command.AddCommand(promote)
	command.AddCommand(&cobra.Command{Use: "status", Args: cobra.NoArgs, Short: "Print durable SDK release state as JSON", RunE: func(cmd *cobra.Command, _ []string) error {
		state, _, err := upstream.ReadState(cmd.Context())
		if err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(state)
	}})
	command.AddCommand(&cobra.Command{Use: "finalize", Args: cobra.NoArgs, Short: "Update local production locks when the requested release is ready", RunE: func(cmd *cobra.Command, _ []string) error {
		root, err := repositoryRoot(cmd, runner)
		if err != nil {
			return err
		}
		changed, err := upstream.Finalize(cmd.Context(), root)
		if err != nil {
			return err
		}
		return output(cmd, "changed", strconv.FormatBool(changed))
	}})
	command.AddCommand(&cobra.Command{Use: "verify", Args: cobra.NoArgs, Short: "Verify committed production SDK dependencies without changing pins", RunE: func(cmd *cobra.Command, _ []string) error {
		root, err := repositoryRoot(cmd, runner)
		if err != nil {
			return err
		}
		d, err := core.ReadDependency(root)
		if err != nil {
			return err
		}
		if d.Channel != "release" || d.Version == "" || d.SHA == "" {
			return fmt.Errorf("SDK preview is not production-ready; run sdkctl promote")
		}
		ready, err := upstream.ResolveDependency(cmd.Context(), d)
		if err != nil {
			return err
		}
		if ready == nil {
			return fmt.Errorf("SDK release is not ready")
		}
		return core.VerifyLocks(cmd.Context(), runner, root, d)
	}})
	var base string
	check := &cobra.Command{Use: "check-changes", Args: cobra.NoArgs, Short: "Validate additive release declarations in a PR", RunE: func(cmd *cobra.Command, _ []string) error {
		root, err := repositoryRoot(cmd, runner)
		if err != nil {
			return err
		}
		return core.CheckChanges(cmd.Context(), runner, root, base)
	}}
	check.Flags().StringVar(&base, "base", "origin/main", "PR base commit")
	command.AddCommand(check)
	controller := &cobra.Command{Use: "controller", Hidden: true}
	controller.AddCommand(&cobra.Command{Use: "register <SDK PR number>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		number, err := strconv.Atoi(args[0])
		if err != nil {
			return err
		}
		return upstream.Register(cmd.Context(), number)
	}})
	var retry bool
	prepare := &cobra.Command{Use: "prepare", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		candidate, err := upstream.Prepare(cmd.Context(), os.Getenv("GITHUB_RUN_ID"), retry)
		if err != nil {
			return err
		}
		if err = output(cmd, "needed", strconv.FormatBool(candidate != nil)); err != nil || candidate == nil {
			return err
		}
		if err = output(cmd, "sha", candidate.SHA); err != nil {
			return err
		}
		if err = output(cmd, "publish_only", strconv.FormatBool(candidate.Phase == "publishing")); err != nil {
			return err
		}
		if err = output(cmd, "artifact_run", candidate.RunID); err != nil {
			return err
		}
		return output(cmd, "version", candidate.Version)
	}}
	prepare.Flags().BoolVar(&retry, "retry", false, "Explicitly retry a failed candidate without code changes")
	controller.AddCommand(prepare)
	controller.AddCommand(&cobra.Command{Use: "phase <sha> <failed|publishing|ready>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return upstream.SetPhase(cmd.Context(), args[0], args[1])
	}})
	command.AddCommand(controller)
	return command
}

func repositoryRoot(cmd *cobra.Command, runner core.Runner) (string, error) {
	out, err := runner.Run(cmd.Context(), "", nil, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.Abs(strings.TrimSpace(string(out)))
}

func output(cmd *cobra.Command, key, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("invalid workflow output")
	}
	line := key + "=" + value + "\n"
	fmt.Fprint(cmd.OutOrStdout(), line)
	if path := os.Getenv("GITHUB_OUTPUT"); path != "" {
		file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = file.WriteString(line)
		return err
	}
	return nil
}
