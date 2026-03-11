package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/commands/common"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/credentials"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/execution"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/ops"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/persistence"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/policy"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dbctl",
		Short: "Policy-driven DB operations engine",
	}
	cmd.AddCommand(newPlanCommand())
	cmd.AddCommand(newApplyCommand())
	cmd.AddCommand(newDoctorCommand())
	cmd.AddCommand(newHistoryCommand())
	cmd.AddCommand(newCredentialsCommand())
	return cmd
}

func newPlanCommand() *cobra.Command {
	var jsonOutput bool
	var yes bool
	var ticket string
	var policyPath string
	cmd := &cobra.Command{
		Use:   "plan <operation>",
		Short: "Evaluate policy and print execution plan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			plan, err := execution.Plan(ctx, execution.RunOptions{
				Operation:     args[0],
				Mode:          ops.ExecutionModePlan,
				Yes:           yes,
				ApproveTicket: ticket,
				JSONOutput:    jsonOutput,
				PolicyPath:    policyPath,
			})
			if err != nil {
				return err
			}
			if jsonOutput {
				execution.Emit(os.Stdout, true, execution.Event{Type: "summary", Operation: plan.Spec.Name, Status: "planned", Payload: plan})
				return nil
			}
			fmt.Printf("Operation: %s\n", plan.Spec.Name)
			fmt.Printf("Kind: %s\n", plan.Spec.Kind)
			fmt.Printf("Policy hash: %s\n", plan.PolicyHash)
			for _, step := range plan.Spec.Steps {
				fmt.Printf("- %s: %s [%s]\n", step.ID, step.Description, step.TxMode)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit JSON events")
	cmd.Flags().BoolVar(&yes, "yes", false, "Acknowledge confirmation requirements")
	cmd.Flags().StringVar(&ticket, "approve-ticket", "", "Change request ticket required by policy")
	cmd.Flags().StringVar(&policyPath, "policy", "", "Path to policy file (default .dbctl/policy.yaml)")
	return cmd
}

func newApplyCommand() *cobra.Command {
	var jsonOutput bool
	var yes bool
	var ticket string
	var policyPath string
	var actor string
	cmd := &cobra.Command{
		Use:   "apply <operation>",
		Short: "Execute operation through policy-checked runner",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return execution.Apply(cmd.Context(), execution.RunOptions{
				Operation:     args[0],
				Mode:          ops.ExecutionModeApply,
				Yes:           yes,
				ApproveTicket: ticket,
				JSONOutput:    jsonOutput,
				PolicyPath:    policyPath,
				Actor:         actor,
				Out:           os.Stdout,
			})
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit JSON events")
	cmd.Flags().BoolVar(&yes, "yes", false, "Acknowledge confirmation requirements")
	cmd.Flags().StringVar(&ticket, "approve-ticket", "", "Change request ticket required by policy")
	cmd.Flags().StringVar(&policyPath, "policy", "", "Path to policy file (default .dbctl/policy.yaml)")
	cmd.Flags().StringVar(&actor, "actor", "", "Actor identifier for audit logs")
	return cmd
}

func newDoctorCommand() *cobra.Command {
	var policyPath string
	var yes bool
	var ticket string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Validate dbctl policy and target resolution",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, payload, err := policy.Load(policyPath)
			if err != nil {
				return err
			}
			targetPlan, err := execution.Plan(cmd.Context(), execution.RunOptions{
				Operation:     "seed.main",
				Mode:          ops.ExecutionModePlan,
				Yes:           yes,
				ApproveTicket: ticket,
				PolicyPath:    policyPath,
			})
			if err != nil {
				return fmt.Errorf("doctor failed: %w", err)
			}
			fmt.Printf("policy hash: %s\n", policy.HashPolicy(payload))
			fmt.Printf("policy envs: %d\n", len(cfg.Environments))
			fmt.Printf("resolved target: env=%s host=%s db=%s\n", targetPlan.RunContext.Target.Environment, targetPlan.RunContext.Target.Host, targetPlan.RunContext.Target.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&policyPath, "policy", "", "Path to policy file (default .dbctl/policy.yaml)")
	cmd.Flags().BoolVar(&yes, "yes", false, "Acknowledge confirmation requirements when policy requires it")
	cmd.Flags().StringVar(&ticket, "approve-ticket", "", "Change request ticket required by policy")
	return cmd
}

func newHistoryCommand() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show recent dbctl run history",
		RunE: func(cmd *cobra.Command, args []string) error {
			pool, err := common.GetDefaultDatabasePool()
			if err != nil {
				return err
			}
			defer pool.Close()
			repo := persistence.New(pool)
			if err := repo.EnsureTables(cmd.Context()); err != nil {
				return err
			}
			runs, err := repo.ListRuns(cmd.Context(), limit)
			if err != nil {
				return err
			}
			for _, run := range runs {
				fmt.Printf("%s %s %s %s\n", run.StartedAt.Format(time.RFC3339), run.Operation, run.Status, run.ID)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of runs to display")
	return cmd
}

func newCredentialsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "credentials", Short: "Credential bootstrap artifacts"}
	cmd.AddCommand(newCredentialShowCommand())
	return cmd
}

func newCredentialShowCommand() *cobra.Command {
	var reveal bool
	var policyPath string
	cmd := &cobra.Command{
		Use:   "show <run-id>",
		Short: "Show credential bootstrap artifact for a run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pool, err := common.GetDefaultDatabasePool()
			if err != nil {
				return err
			}
			defer pool.Close()
			repo := persistence.New(pool)
			artifact, err := repo.LatestArtifact(cmd.Context(), args[0], "credential_bootstrap")
			if err != nil {
				return err
			}
			if artifact == nil {
				return fmt.Errorf("no credential artifact found")
			}
			var bootstrap credentials.BootstrapArtifact
			if err := json.Unmarshal([]byte(artifact.PayloadJSON), &bootstrap); err != nil {
				return fmt.Errorf("parse bootstrap artifact payload: %w", err)
			}

			fmt.Printf("run_id: %s\n", artifact.RunID)
			fmt.Printf("token_id: %s\n", bootstrap.TokenID)
			fmt.Printf("subject: %s\n", bootstrap.Subject)
			fmt.Printf("expires_at: %s\n", bootstrap.ExpiresAt.Format(time.RFC3339))
			if !reveal {
				fmt.Println("secret: [hidden] use --reveal when policy allows")
				return nil
			}

			cfg, _, err := policy.Load(policyPath)
			if err != nil {
				return err
			}
			if !allowsReveal(cfg.Credentials.Emission) {
				return fmt.Errorf("credential reveal is disabled by policy (emission=%s)", cfg.Credentials.Emission)
			}
			if strings.TrimSpace(bootstrap.Secret) == "" {
				fmt.Println("secret: [not available in artifact]")
				return nil
			}
			fmt.Printf("secret: %s\n", bootstrap.Secret)
			return nil
		},
	}
	cmd.Flags().BoolVar(&reveal, "reveal", false, "Reveal secret fields when policy allows")
	cmd.Flags().StringVar(&policyPath, "policy", "", "Path to policy file (default .dbctl/policy.yaml)")
	return cmd
}

func allowsReveal(emission string) bool {
	switch strings.TrimSpace(strings.ToLower(emission)) {
	case "masked":
		return true
	default:
		return false
	}
}
