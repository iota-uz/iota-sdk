package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/commands/common"
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
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Validate dbctl policy and target resolution",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, payload, err := policy.Load(policyPath)
			if err != nil {
				return err
			}
			targetPlan, err := execution.Plan(cmd.Context(), execution.RunOptions{Operation: "seed.main", Mode: ops.ExecutionModePlan, Yes: true, PolicyPath: policyPath})
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
			if !reveal {
				fmt.Printf("artifact: %s\n", artifact.PayloadJSON)
				return nil
			}
			fmt.Printf("artifact (revealed): %s\n", artifact.PayloadJSON)
			return nil
		},
	}
	cmd.Flags().BoolVar(&reveal, "reveal", false, "Reveal secret fields when policy allows")
	return cmd
}
