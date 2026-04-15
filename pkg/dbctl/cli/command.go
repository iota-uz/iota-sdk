package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/execution"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/ops"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/policy"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/spf13/cobra"
)

// resolveRunOptions populates the config-derived fields of RunOptions from the
// legacy configuration singleton. This is the single config-resolution site
// in the dbctl CLI layer; runner.go no longer reads it.
func resolveRunOptions(base execution.RunOptions) execution.RunOptions {
	legacyConf := configuration.Use()
	cfg := dbconfig.FromLegacy(legacyConf)
	base.DBConfig = &cfg
	base.AppEnvironment = legacyConf.GoAppEnvironment
	base.Logger = legacyConf.Logger()
	base.LegacyConf = legacyConf
	return base
}

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
	var dryRun bool
	var policyPath string
	cmd := &cobra.Command{
		Use:   "plan <operation>",
		Short: "Evaluate policy and print execution plan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			out := cmd.OutOrStdout()
			plan, err := execution.Plan(ctx, resolveRunOptions(execution.RunOptions{
				Operation:  args[0],
				Mode:       ops.ExecutionModePlan,
				Yes:        yes,
				DryRun:     dryRun,
				JSONOutput: jsonOutput,
				PolicyPath: policyPath,
			}))
			if err != nil {
				return err
			}
			if jsonOutput {
				execution.Emit(out, true, execution.Event{Type: "summary", Operation: plan.Spec.Name, Status: "planned", Payload: plan})
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
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm destructive intent when required")
	cmd.Flags().StringVar(&policyPath, "policy-path", "", "Path to an optional dbctl policy file")
	return cmd
}

func newApplyCommand() *cobra.Command {
	var jsonOutput bool
	var yes bool
	var dryRun bool
	var actor string
	var policyPath string
	cmd := &cobra.Command{
		Use:   "apply <operation>",
		Short: "Execute operation through policy-checked runner",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()
			return execution.Apply(ctx, resolveRunOptions(execution.RunOptions{
				Operation:  args[0],
				Mode:       ops.ExecutionModeApply,
				Yes:        yes,
				DryRun:     dryRun,
				JSONOutput: jsonOutput,
				PolicyPath: policyPath,
				Actor:      actor,
				Out:        cmd.OutOrStdout(),
			}))
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit JSON events")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview actions without executing")
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm destructive intent when required")
	cmd.Flags().StringVar(&actor, "actor", "", "Actor identifier for audit logs")
	cmd.Flags().StringVar(&policyPath, "policy-path", "", "Path to an optional dbctl policy file")
	return cmd
}

func newDoctorCommand() *cobra.Command {
	var policyPath string
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
			targetPlan, err := execution.Plan(cmd.Context(), resolveRunOptions(execution.RunOptions{
				Operation:  "seed.main",
				Mode:       ops.ExecutionModePlan,
				PolicyPath: policyPath,
			}))
			if err != nil {
				return serrors.E(op, err)
			}
			_, _ = fmt.Fprintf(out, "policy hash: %s\n", policy.HashPolicy(payload))
			_, _ = fmt.Fprintf(out, "policy envs: %d\n", len(cfg.Environments))
			_, _ = fmt.Fprintf(out, "resolved target: env=%s host=%s:%s user=%s db=%s\n",
				targetPlan.RunContext.Target.Environment,
				targetPlan.RunContext.Target.Host,
				targetPlan.RunContext.Target.Port,
				targetPlan.RunContext.Target.User,
				targetPlan.RunContext.Target.Name,
			)
			return nil
		},
	}
	cmd.Flags().StringVar(&policyPath, "policy-path", "", "Path to an optional dbctl policy file")
	return cmd
}
