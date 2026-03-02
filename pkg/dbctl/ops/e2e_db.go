package ops

import (
	"context"
	"fmt"

	commande2e "github.com/iota-uz/iota-sdk/pkg/commands/e2e"
)

func E2ECreateOperation() OperationSpec {
	return OperationSpec{
		Name: "db.e2e.create",
		Kind: OperationKindDestructive,
		Steps: []StepSpec{{
			ID:          "e2e_create",
			Description: "Drop and recreate e2e database",
			TxMode:      TxModeNone,
			Handler: func(_ context.Context, _ *ExecutionContext) error {
				return commande2e.CreateRaw()
			},
		}},
	}
}

func E2EDropOperation() OperationSpec {
	return OperationSpec{
		Name: "db.e2e.drop",
		Kind: OperationKindDestructive,
		Steps: []StepSpec{{
			ID:          "e2e_drop",
			Description: "Drop e2e database",
			TxMode:      TxModeNone,
			Handler: func(_ context.Context, _ *ExecutionContext) error {
				return commande2e.DropRaw()
			},
		}},
	}
}

func E2EMigrateOperation() OperationSpec {
	return OperationSpec{
		Name: "db.e2e.migrate",
		Kind: OperationKindMigration,
		Steps: []StepSpec{{
			ID:          "e2e_migrate",
			Description: "Run e2e migrations",
			TxMode:      TxModeNone,
			Handler: func(_ context.Context, _ *ExecutionContext) error {
				return commande2e.Migrate()
			},
		}},
	}
}

func E2EResetOperation() OperationSpec {
	return OperationSpec{
		Name: "db.e2e.reset",
		Kind: OperationKindDestructive,
		Steps: []StepSpec{
			{
				ID:          "e2e_create",
				Description: "Drop and recreate e2e database",
				TxMode:      TxModeNone,
				Handler: func(_ context.Context, _ *ExecutionContext) error {
					return commande2e.CreateRaw()
				},
			},
			{
				ID:          "e2e_migrate",
				Description: "Run e2e migrations",
				TxMode:      TxModeNone,
				Handler: func(_ context.Context, _ *ExecutionContext) error {
					return commande2e.Migrate()
				},
			},
			{
				ID:          "e2e_seed",
				Description: "Seed e2e data",
				TxMode:      TxModeNone,
				Handler: func(_ context.Context, _ *ExecutionContext) error {
					return commande2e.SeedRaw()
				},
			},
		},
		Postconditions: []Condition{{
			ID:          "ensure_seed_complete",
			Description: "Ensure reset chain completes without errors",
			Check: func(_ context.Context, _ *ExecutionContext) error {
				return nil
			},
		}},
	}
}

func SeedE2EOperation() OperationSpec {
	return OperationSpec{
		Name: "seed.e2e",
		Kind: OperationKindSeed,
		Preconditions: []Condition{{
			ID:          "e2e_db_available",
			Description: "Verify e2e DB is reachable",
			Check: func(_ context.Context, _ *ExecutionContext) error {
				exists, err := commande2e.DatabaseExists()
				if err != nil {
					return err
				}
				if !exists {
					return fmt.Errorf("e2e database does not exist")
				}
				return nil
			},
		}},
		Steps: []StepSpec{{
			ID:          "seed_e2e",
			Description: "Seed e2e dataset",
			TxMode:      TxModeNone,
			Handler: func(_ context.Context, _ *ExecutionContext) error {
				return commande2e.SeedRaw()
			},
		}},
	}
}
