package ops

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/policy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OperationKind string

const (
	OperationKindSeed        OperationKind = "seed"
	OperationKindDestructive OperationKind = "destructive"
	OperationKindMigration   OperationKind = "migration"
)

type TxMode string

const (
	TxModeNone     TxMode = "none"
	TxModeSingleTx TxMode = "single_tx"
	TxModeOwnTx    TxMode = "own_tx"
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
	ApproveTicket  string
	Yes            bool
	RunID          string
}

type ExecutionContext struct {
	Run        *RunContext
	Pool       *pgxpool.Pool
	App        application.Application
	Tx         pgx.Tx
	Policy     policy.Config
	PolicyHash string
	JSONOutput bool
	PolicyPath string
}
