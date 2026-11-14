package native

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
)

// The contract rules below are the single implementation of what the
// invocation host (pkg/extruntime/host) and the Direct dispatcher both
// enforce on a contract invocation: input/output validation, permissions,
// the call shape of tx.call / ctx.call / query, and the nesting limit. The
// host applies them to every contract (package or native); Direct, which only
// runs Go defaults, applies the same functions, so the two cannot disagree
// on an error code.

// MaxNesting bounds nested invocations (tx.call, tx.query, ctx.call, guards)
// under one root.
const MaxNesting = 32

// Error codes of the contract rules.
const (
	CodeInvalidValue        = "runtime.invalidValue"
	CodeInvalidOutput       = "runtime.invalidOutput"
	CodePermissionDenied    = "runtime.permissionDenied"
	CodeOrdinaryCommandInTx = "runtime.ordinaryCommandInTx"
	CodeNotCommand          = "runtime.notCommand"
	CodeNotQuery            = "runtime.notQuery"
	CodeNestingTooDeep      = "runtime.nestingTooDeep"
	CodeNestedTransaction   = "runtime.nestedTransaction"
	CodeIntentInTransaction = "runtime.intentInTransaction"
)

// Via is how a nested invocation is requested.
type Via string

const (
	// ViaTxCall joins the caller's transaction: operations and atomic
	// commands only.
	ViaTxCall Via = "tx.call"
	// ViaCtxCall is a new invocation from an ordinary command: commands only.
	ViaCtxCall Via = "ctx.call"
	// ViaQuery reads in the caller's snapshot or transaction: queries only.
	ViaQuery Via = "query"
)

// CheckCall enforces the call shape of via for contract c.
func CheckCall(via Via, c manifest.Resource) error {
	switch via {
	case ViaTxCall:
		if c.Kind != manifest.ResOperation && (c.Kind != manifest.ResCommand || !c.Atomic) {
			return pool.Internal(CodeOrdinaryCommandInTx, fmt.Sprintf("tx.call accepts only operations and atomic commands; %s is not", c.ID))
		}
	case ViaCtxCall:
		if c.Kind != manifest.ResCommand {
			return pool.Internal(CodeNotCommand, fmt.Sprintf("ctx.call accepts commands; %s is a %s", c.ID, c.Kind))
		}
	case ViaQuery:
		if c.Kind != manifest.ResQuery {
			return pool.Internal(CodeNotQuery, fmt.Sprintf("query accepts queries; %s is a %s", c.ID, c.Kind))
		}
	}
	return nil
}

// Permissions lists the permissions an invocation of c needs: the published
// contract's and, for a native contract, the RBAC permission of its Go
// definition (def may be nil). Both apply whichever implementation is bound.
func Permissions(c manifest.Resource, def Definition) []string {
	var out []string
	if c.Permission != "" {
		out = append(out, c.Permission)
	}
	if def != nil && def.Permission() != "" {
		out = append(out, def.Permission())
	}
	return out
}

// CheckPermissions fails with runtime.permissionDenied unless has accepts
// every permission of Permissions(c, def).
func CheckPermissions(has func(permission string) bool, c manifest.Resource, def Definition) error {
	for _, p := range Permissions(c, def) {
		if !has(p) {
			return &pool.WireError{Kind: pool.ErrKindDenied, Code: CodePermissionDenied, Message: "permission " + p + " is required for " + c.ID}
		}
	}
	return nil
}

// CheckInput validates an invocation input against the contract: a
// validation error runtime.invalidValue with one issue at the failing path.
func CheckInput(c manifest.Resource, input json.RawMessage) error {
	if c.Input == nil {
		return nil
	}
	if err := manifest.ValidateValue(*c.Input, input); err != nil {
		return invalidValue("input", err)
	}
	return nil
}

// CheckOutput validates an implementation's output: an internal error
// runtime.invalidOutput (the implementation broke its contract).
func CheckOutput(c manifest.Resource, output json.RawMessage) error {
	if c.Output == nil {
		return nil
	}
	if err := manifest.ValidateValue(*c.Output, output); err != nil {
		return invalidValue("output", err)
	}
	return nil
}

// NestingTooDeep is the error of an invocation nested deeper than
// MaxNesting.
func NestingTooDeep(contract string) error {
	return pool.Internal(CodeNestingTooDeep, fmt.Sprintf("more than %d nested invocations under one root (recursive call of %s?)", MaxNesting, contract))
}

func invalidValue(phase string, err error) *pool.WireError {
	if phase == "output" {
		return pool.Internal(CodeInvalidOutput, err.Error())
	}
	we := &pool.WireError{Kind: pool.ErrKindValidation, Code: CodeInvalidValue, Message: err.Error()}
	var pe *manifest.PathError
	if errors.As(err, &pe) {
		path, pathErr := json.Marshal(pe.Path)
		params, paramsErr := json.Marshal(map[string]string{"message": pe.Message})
		// Without an encodable path the error keeps its message and no
		// per-field issue.
		if pathErr == nil && paramsErr == nil {
			we.Issues = []pool.Issue{{Code: CodeInvalidValue, Phase: phase, Path: path, Params: params}}
		}
	}
	return we
}
