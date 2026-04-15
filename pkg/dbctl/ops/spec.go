package ops

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/policy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

type OperationKind string

const (
	OperationKindSeed        OperationKind = "seed"
	OperationKindDestructive OperationKind = "destructive"
	OperationKindMigration   OperationKind = "migration"
)

type TxMode string

const (
	TxModeNone  TxMode = "none"
	TxModeOwnTx TxMode = "own_tx"
)

type ExecutionMode string

const (
	ExecutionModePlan  ExecutionMode = "plan"
	ExecutionModeApply ExecutionMode = "apply"
)

type Condition struct {
	ID          string
	Description string
	Check       func(ctx context.Context, e *ExecutionContext) error
}

type StepSpec struct {
	ID             string
	Description    string
	TxMode         TxMode
	IdempotencyKey string
	Handler        func(ctx context.Context, e *ExecutionContext) error
}

type OperationSpec struct {
	Name           string
	Kind           OperationKind
	Steps          []StepSpec
	Preconditions  []Condition
	Postconditions []Condition
}

type RunContext struct {
	Operation      string
	ExecutionMode  ExecutionMode
	Target         policy.Target
	PolicyDecision policy.Decision
	Yes            bool
}

type ExecutionContext struct {
	Run        *RunContext
	Pool       *pgxpool.Pool
	App        application.Application
	Tx         pgx.Tx
	JSONOutput bool
	PolicyPath string
	// Logger is resolved from the typed telemetryconfig at runner time.
	// W5.1 will replace this with a proper typed logger injection.
	Logger *logrus.Logger
	// LegacyConf provides the full legacy configuration for operations that
	// still need it during the W4 transition. W5.1 will remove this field.
	LegacyConf *configuration.Configuration
}
