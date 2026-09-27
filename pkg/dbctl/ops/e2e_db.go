package ops

import (
	"context"
	"fmt"

	commande2e "github.com/iota-uz/iota-sdk/pkg/commands/e2e"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
)

// e2eConfig extracts a *dbconfig.Config from the ExecutionContext.
// Falls back to a zero-value Config if DBConfig is nil.
func e2eConfig(e *ExecutionContext) *dbconfig.Config {
	if e.DBConfig == nil {
		return &dbconfig.Config{}
	}
	return e.DBConfig
}

func E2ECreateOperation() OperationSpec {
	return OperationSpec{
		Name: "db.e2e.create",
		Kind: OperationKindDestructive,
		Steps: []StepSpec{{
			ID:          "e2e_create",
			Description: "Drop and recreate e2e database",
			TxMode:      TxModeNone,
			Handler: func(_ context.Context, e *ExecutionContext) error {
				return commande2e.CreateRaw(e2eConfig(e), e.Logger)
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
			Handler: func(_ context.Context, e *ExecutionContext) error {
				return commande2e.DropRaw(e2eConfig(e), e.Logger)
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
			Handler: func(_ context.Context, e *ExecutionContext) error {
				return commande2e.Migrate(e2eConfig(e), e.Logger)
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
				Handler: func(_ context.Context, e *ExecutionContext) error {
					return commande2e.CreateRaw(e2eConfig(e), e.Logger)
				},
			},
			{
				ID:          "e2e_migrate",
				Description: "Run e2e migrations",
				TxMode:      TxModeNone,
				Handler: func(_ context.Context, e *ExecutionContext) error {
					return commande2e.Migrate(e2eConfig(e), e.Logger)
				},
			},
			{
				ID:          "e2e_seed",
				Description: "Seed e2e data",
				TxMode:      TxModeNone,
				Handler: func(_ context.Context, e *ExecutionContext) error {
					return commande2e.SeedRaw(e2eConfig(e), e.Logger)
				},
			},
		},
		Postconditions: []Condition{{
			ID:          "ensure_seed_complete",
			Description: "Ensure the e2e database exists after reset",
			Check: func(_ context.Context, e *ExecutionContext) error {
				exists, err := commande2e.DatabaseExists(e2eConfig(e))
				if err != nil {
					return err
				}
				if !exists {
					return fmt.Errorf("e2e database does not exist after reset")
				}
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
			Check: func(_ context.Context, e *ExecutionContext) error {
				exists, err := commande2e.DatabaseExists(e2eConfig(e))
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
			Handler: func(_ context.Context, e *ExecutionContext) error {
				return commande2e.SeedRaw(e2eConfig(e), e.Logger)
			},
		}},
	}
}
