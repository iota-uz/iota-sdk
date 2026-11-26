// Package setstate is the built-in node type set_state: it writes (or deletes) one key of the automation's
// memory between runs, which the expressions of later steps and of later runs read as state.get(key) and
// state.has(key). The write is made in the transaction of the step, so it commits with the step or not at
// all. A dry run captures the node and writes nothing.
package setstate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/automation/state"
)

// Type is the node type.
const Type = "set_state"

// Outcomes of the node.
const (
	OutcomeSet    = "set"
	OutcomeFailed = "failed"
)

const paramsSchema = `{"type": "object", "properties": {
	"key": {"type": "string", "minLength": 1, "maxLength": 200, "description": "template: the key, at most 200 characters"},
	"value": {"type": "string", "minLength": 1, "maxLength": 2000, "description": "CEL: the value to keep, at most 16 KiB as JSON"},
	"ttl": {"type": "string", "pattern": "^[1-9][0-9]{0,6}(m|h|d)$", "x-max-ms": 31536000000, "description": "20m, 2h or 7d: how long the key lives, from 1 minute to 365 days; without it the key stays"},
	"delete": {"type": "boolean", "default": false, "description": "delete the key instead of writing it"}
}, "required": ["key"], "additionalProperties": false}`

// Node is the set_state node type.
type Node struct{}

var (
	_ catalog.NodeType      = Node{}
	_ engine.EffectNode     = Node{}
	_ engine.NodeValidator  = Node{}
	_ catalog.LocalizedNode = Node{}
)

// New is the set_state node type.
func New() Node { return Node{} }

func (Node) Spec() graph.NodeSpec {
	return graph.NodeSpec{
		Type: Type, Title: "Set state", Group: "data", Behavior: graph.BehaviorEffect,
		Description: "Keeps a value under a key in the memory of this automation, for the next runs: state.get(key) is the " +
			"value or null, state.has(key) tells whether the key is there. The key may expire after a ttl; delete removes it.",
		Params:        graph.MustSchemaParams(paramsSchema),
		Outcomes:      []string{OutcomeSet, OutcomeFailed},
		DoneOutcome:   OutcomeSet,
		FailedOutcome: OutcomeFailed,
		Templates:     []string{"key"},
		Values:        []string{"value"},
		Output: expr.Fields{
			{Name: "key", Field: expr.StringField},
			{Name: "deleted", Field: expr.BoolField},
			{Name: "expires_at", Field: expr.OptionalOf(expr.TimestampField)},
		},
		// A dry run captures the node: it shows the rendered key and value and writes nothing.
		DryRun: graph.DryRunCapture,
	}
}

func (Node) Requires() []catalog.ConnectorKind { return nil }

func (Node) Localize(spec graph.NodeSpec, locale string) graph.NodeSpec {
	switch locale {
	case "ru":
		spec.Title = "Запомнить значение"
		spec.Description = "Хранит значение по ключу в памяти этой автоматизации для следующих запусков: state.get(ключ) — значение или null, " +
			"state.has(ключ) — есть ли ключ. Ключ может истечь через ttl; delete удаляет его."
	case "uz":
		spec.Title = "Qiymatni eslab qolish"
		spec.Description = "Qiymatni kalit bo‘yicha ushbu avtomatlashtirish xotirasida keyingi ishga tushirishlar uchun saqlaydi: state.get(kalit) — qiymat yoki null, " +
			"state.has(kalit) — kalit bor-yo‘qligi. Kalit ttl dan keyin tugashi mumkin; delete uni o‘chiradi."
	case "uz-Cyrl":
		spec.Title = "Қийматни эслаб қолиш"
		spec.Description = "Қийматни калит бўйича ушбу автоматлаштириш хотирасида кейинги ишга туширишлар учун сақлайди: state.get(калит) — қиймат ёки null, " +
			"state.has(калит) — калит бор-йўқлиги. Калит ttl дан кейин тугаши мумкин; delete уни ўчиради."
	}
	return spec
}

// ValidateParams refuses what the schema cannot: a write needs a value, a delete has neither a value nor a ttl.
func (Node) ValidateParams(_ context.Context, params map[string]any) []graph.GraphError {
	deleting, _ := params["delete"].(bool)
	_, hasValue := params["value"]
	_, hasTTL := params["ttl"]
	var errs []graph.GraphError
	switch {
	case deleting && hasValue:
		errs = append(errs, graph.GraphError{Path: "params.value", Code: "state_value_unused", Message: "a node that deletes the key has no value"})
	case !deleting && !hasValue:
		errs = append(errs, graph.GraphError{Path: "params.value", Code: "state_value_required", Message: "a value is required unless the node deletes the key"})
	}
	if deleting && hasTTL {
		errs = append(errs, graph.GraphError{Path: "params.ttl", Code: "state_ttl_unused", Message: "a node that deletes the key has no ttl"})
	}
	return errs
}

// ApplyEffect writes or deletes the key in the step's transaction.
func (Node) ApplyEffect(ctx context.Context, tx pgx.Tx, run engine.StepContext, _ graph.Node, e graph.Effect) (map[string]any, error) {
	key, _ := e.Params["key"].(string)
	// The key is rendered text: a template may leave it empty or too long.
	if err := state.ValidateKey(key); err != nil {
		return nil, stepError(err)
	}
	if run.AutomationID == uuid.Nil {
		return nil, &engine.StepError{Code: engine.CodeInvalidInput, Message: "the node runs outside an automation"}
	}
	if deleting, _ := e.Params["delete"].(bool); deleting {
		if _, err := state.Delete(ctx, tx, run.TenantID, run.AutomationID, key); err != nil {
			return nil, stepError(err)
		}
		return map[string]any{"key": key, "deleted": true}, nil
	}
	value, present := e.Params["value"]
	if !present {
		return nil, &engine.StepError{Code: engine.CodeInvalidInput, Message: "the node has no value to keep"}
	}
	var ttl time.Duration
	if s, ok := e.Params["ttl"].(string); ok && s != "" {
		ms, ok := graph.DurationMs(s)
		if !ok {
			return nil, &engine.StepError{Code: state.CodeTTLInvalid, Message: fmt.Sprintf("%q is not a duration", s)}
		}
		ttl = state.DurationOfMs(ms)
	}
	var byRun *uuid.UUID
	if run.RunID != uuid.Nil {
		id := run.RunID
		byRun = &id
	}
	entry, err := state.Put(ctx, tx, run.TenantID, run.AutomationID, key, value, ttl, byRun)
	if err != nil {
		return nil, stepError(err)
	}
	out := map[string]any{"key": key, "deleted": false}
	if entry.ExpiresAt != nil {
		out["expires_at"] = entry.ExpiresAt.UTC().Format(time.RFC3339Nano)
	}
	return out, nil
}

// stepError is a refusal of the memory as the node's failure: its code and message. Any other error (the
// database) is returned as it is: the engine retries a platform failure and fails the node on the rest.
func stepError(err error) error {
	var se *state.Error
	if errors.As(err, &se) {
		return &engine.StepError{Code: se.Code, Message: short(se.Message)}
	}
	return err
}

func short(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 500 {
		return s[:500]
	}
	return s
}
