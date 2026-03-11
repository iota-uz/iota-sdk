package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/dbctl/execution"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/ops"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/policy"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
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
	return cmd
}

func newPlanCommand() *cobra.Command {
	var jsonOutput bool
	var yes bool
	var force bool
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "plan <operation>",
		Short: "Evaluate policy and print execution plan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			out := cmd.OutOrStdout()
			plan, err := execution.Plan(ctx, execution.RunOptions{
				Operation:  args[0],
				Mode:       ops.ExecutionModePlan,
				Yes:        yes,
				Force:      force,
				DryRun:     dryRun,
				JSONOutput: jsonOutput,
			})
			if err != nil {
				return err
			}
			if jsonOutput {
				execution.Emit(os.Stdout, true, execution.Event{Type: "summary", Operation: plan.Spec.Name, Status: "planned", Payload: plan})
				return nil
			}
			_, _ = fmt.Fprintf(out, "Operation: %s\n", plan.Spec.Name)
			_, _ = fmt.Fprintf(out, "Kind: %s\n", plan.Spec.Kind)
			_, _ = fmt.Fprintf(out, "Policy hash: %s\n", plan.PolicyHash)
			for _, step := range plan.Spec.Steps {
				_, _ = fmt.Fprintf(out, "- %s: %s [%s]\n", step.ID, step.Description, step.TxMode)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit JSON events")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview actions without executing")
	cmd.Flags().BoolVar(&yes, "yes", false, "Acknowledge confirmation requirements")
	cmd.Flags().BoolVar(&force, "force", false, "Confirm destructive intent")
	return cmd
}

func newApplyCommand() *cobra.Command {
	var jsonOutput bool
	var yes bool
	var force bool
	var dryRun bool
	var actor string
	cmd := &cobra.Command{
		Use:   "apply <operation>",
		Short: "Execute operation through policy-checked runner",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return execution.Apply(cmd.Context(), execution.RunOptions{
				Operation:  args[0],
				Mode:       ops.ExecutionModeApply,
				Yes:        yes,
				Force:      force,
				DryRun:     dryRun,
				JSONOutput: jsonOutput,
				Actor:      actor,
				Out:        os.Stdout,
			})
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit JSON events")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview actions without executing")
	cmd.Flags().BoolVar(&yes, "yes", false, "Acknowledge confirmation requirements")
	cmd.Flags().BoolVar(&force, "force", false, "Confirm destructive intent")
	cmd.Flags().StringVar(&actor, "actor", "", "Actor identifier for audit logs")
	return cmd
}

func newDoctorCommand() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Validate dbctl policy and target resolution",
		RunE: func(cmd *cobra.Command, args []string) error {
			const op serrors.Op = "dbctl.cli.doctor"
			out := cmd.OutOrStdout()
			cfg, payload, err := policy.Load("")
			if err != nil {
				return err
			}
			targetPlan, err := execution.Plan(cmd.Context(), execution.RunOptions{
				Operation: "seed.main",
				Mode:      ops.ExecutionModePlan,
				Yes:       yes,
			})
			if err != nil {
				return serrors.E(op, err)
			}
			_, _ = fmt.Fprintf(out, "policy hash: %s\n", policy.HashPolicy(payload))
			_, _ = fmt.Fprintf(out, "policy envs: %d\n", len(cfg.Environments))
			_, _ = fmt.Fprintf(out, "resolved target: env=%s host=%s db=%s\n", targetPlan.RunContext.Target.Environment, targetPlan.RunContext.Target.Host, targetPlan.RunContext.Target.Name)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Acknowledge confirmation requirements when policy requires it")
	return cmd
}
