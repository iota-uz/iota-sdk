// Package flow is the built-in node types that start another graph of the tenant: start_automation
// starts its run and continues, run_automation starts it and waits for automation.run.finished.v1 of that
// run (done, failed, or the node's timeout). Both start the graph's enabled version through its manual
// trigger in the step's transaction (engine.RunStarter); the started run's depth counts toward the
// cascade limit. Cancelling a run cancels the run its run_automation started; a run started by
// start_automation lives on its own.
package flow

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// Node types.
const (
	TypeStart = "start_automation"
	TypeRun   = "run_automation"
)

// Outcomes.
const (
	OutcomeStarted = "started"
	OutcomeDone    = "done"
	OutcomeFailed  = "failed"
	OutcomeTimeout = "timeout"
)

const uuidPattern = `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`

const commonProps = `
	"automation_id": {"type": "string", "pattern": "` + uuidPattern + `", "description": "the graph to start; its enabled version runs"},
	"input": {"type": "object", "maxProperties": 50, "default": {},
		"propertyNames": {"pattern": "^[a-z_][a-zA-Z0-9_]{0,63}$"},
		"additionalProperties": {"type": "string", "minLength": 1, "maxLength": 2000, "description": "CEL"},
		"description": "event.input of the started run: name → CEL"},
	"subject": {"type": "string", "minLength": 1, "maxLength": 2000, "description": "CEL: {entity, key} the run is about; default: this run's subject"}`

const startSchema = `{"type": "object", "properties": {` + commonProps + `
}, "required": ["automation_id"], "additionalProperties": false}`

const runSchema = `{"type": "object", "properties": {` + commonProps + `,
	"timeout": {"type": "string", "pattern": "^[1-9][0-9]{0,5}(s|m|h|d)$", "x-max-ms": 2592000000, "default": "1h", "description": "at most 30d"}
}, "required": ["automation_id"], "additionalProperties": false}`

// Starter is the engine's RunStarter, resolved on first use (the engine is built after the catalog).
type Starter func() (engine.RunStarter, error)

// Static is a Starter of a RunStarter at hand.
func Static(s engine.RunStarter) Starter { return func() (engine.RunStarter, error) { return s, nil } }

type starter struct {
	resolve Starter
	once    sync.Once
	s       engine.RunStarter
	err     error
}

func (st *starter) get() (engine.RunStarter, error) {
	st.once.Do(func() {
		if st.resolve == nil {
			st.err = errors.New("this process starts no runs from steps")
			return
		}
		st.s, st.err = st.resolve()
	})
	return st.s, st.err
}

// start starts the run of the effect's params.
func (st *starter) start(ctx context.Context, tx pgx.Tx, run engine.StepContext, e graph.Effect, attached bool) (map[string]any, error) {
	s, err := st.get()
	if err != nil {
		return nil, &engine.StepError{Code: engine.CodeNotEnabled, Message: err.Error()}
	}
	raw, _ := e.Params["automation_id"].(string)
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, &engine.StepError{Code: engine.CodeInvalidInput, Message: "automation_id is not a UUID"}
	}
	in := engine.StepStart{AutomationID: id, Attached: attached}
	if m, ok := e.Params["input"].(map[string]any); ok {
		in.Input = m
	}
	if v, ok := e.Params["subject"]; ok && v != nil {
		subj, err := subjectOf(v)
		if err != nil {
			return nil, &engine.StepError{Code: engine.CodeInvalidInput, Message: err.Error()}
		}
		in.Subject = &subj
	}
	ref, err := s.StartFromStep(ctx, tx, run, in) // a refusal is a *StepError: the node's failed outcome
	if err != nil {
		return nil, err
	}
	return map[string]any{"run_id": ref.RunID.String()}, nil
}

func subjectOf(v any) (engine.SubjectRef, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return engine.SubjectRef{}, errors.New("subject is an object {entity, key}")
	}
	entity, _ := m["entity"].(string)
	if entity == "" {
		return engine.SubjectRef{}, errors.New("subject has no entity")
	}
	key := fmt.Sprint(m["key"])
	if m["key"] == nil {
		key = ""
	}
	return engine.SubjectRef{Entity: entity, Key: key}, nil
}

// StartNode is start_automation.
type StartNode struct{ st *starter }

// RunNode is run_automation.
type RunNode struct{ st *starter }

var (
	_ catalog.NodeType       = (*StartNode)(nil)
	_ engine.EffectNode      = (*StartNode)(nil)
	_ catalog.LocalizedNode  = (*StartNode)(nil)
	_ catalog.NodeType       = (*RunNode)(nil)
	_ engine.EffectNode      = (*RunNode)(nil)
	_ engine.EffectWaitKeyer = (*RunNode)(nil)
	_ engine.WakeResolver    = (*RunNode)(nil)
	_ catalog.LocalizedNode  = (*RunNode)(nil)
)

// New is start_automation and run_automation over one starter.
func New(s Starter) (*StartNode, *RunNode) {
	st := &starter{resolve: s}
	return &StartNode{st: st}, &RunNode{st: st}
}

func (n *StartNode) Spec() graph.NodeSpec {
	return graph.NodeSpec{
		Type: TypeStart, Title: "Start automation", Group: "automation", Behavior: graph.BehaviorEffect,
		Description: "Starts another automation of the tenant (its enabled version, through its manual trigger) and continues at once.",
		Params:      graph.MustSchemaParams(startSchema),
		Outcomes:    []string{OutcomeStarted, OutcomeFailed}, DoneOutcome: OutcomeStarted, FailedOutcome: OutcomeFailed,
		Values: []string{"input.*", "subject"},
		Output: expr.Fields{{Name: "run_id", Field: expr.StringField}},
		DryRun: graph.DryRunCapture,
	}
}

func (n *StartNode) Requires() []catalog.ConnectorKind { return nil }

func (n *StartNode) Localize(spec graph.NodeSpec, locale string) graph.NodeSpec {
	switch locale {
	case "ru":
		spec.Title, spec.Description = "Запустить автоматизацию", "Запускает другую автоматизацию тенанта (её включённую версию через ручной триггер) и сразу продолжает."
	case "uz":
		spec.Title, spec.Description = "Avtomatlashtirishni ishga tushirish", "Tenantning boshqa avtomatlashtirishini (yoqilgan versiyasini qo‘lda trigger orqali) ishga tushiradi va darhol davom etadi."
	case "uz-Cyrl":
		spec.Title, spec.Description = "Автоматлаштиришни ишга тушириш", "Тенантнинг бошқа автоматлаштиришини (ёқилган версиясини қўлда триггер орқали) ишга туширади ва дарҳол давом этади."
	}
	return spec
}

func (n *StartNode) ApplyEffect(ctx context.Context, tx pgx.Tx, run engine.StepContext, _ graph.Node, e graph.Effect) (map[string]any, error) {
	return n.st.start(ctx, tx, run, e, false)
}

func (n *RunNode) Spec() graph.NodeSpec {
	return graph.NodeSpec{
		Type: TypeRun, Title: "Run automation", Group: "automation", Behavior: graph.BehaviorWait,
		Description: "Starts another automation of the tenant and waits until its run finishes: done, failed or timeout.",
		Params:      graph.MustSchemaParams(runSchema),
		Outcomes:    []string{OutcomeDone, OutcomeFailed}, FailedOutcome: OutcomeFailed,
		Values: []string{"input.*", "subject"},
		Wait: &graph.WaitSpec{
			Event: graph.ValueOrParam{Value: engine.EventRunFinished}, TimeoutParam: "timeout",
			Matched: OutcomeDone, TimeoutOutcome: OutcomeTimeout, Effect: true,
		},
		Output: expr.Fields{
			{Name: "run_id", Field: expr.StringField},
			{Name: "status", Field: expr.StringField},
			{Name: "output", Field: expr.DynField},
		},
		DryRun: graph.DryRunCapture,
	}
}

func (n *RunNode) Requires() []catalog.ConnectorKind { return nil }

func (n *RunNode) Localize(spec graph.NodeSpec, locale string) graph.NodeSpec {
	switch locale {
	case "ru":
		spec.Title, spec.Description = "Выполнить автоматизацию", "Запускает другую автоматизацию тенанта и ждёт завершения её запуска: done, failed или timeout."
	case "uz":
		spec.Title, spec.Description = "Avtomatlashtirishni bajarish", "Tenantning boshqa avtomatlashtirishini ishga tushiradi va u tugashini kutadi: done, failed yoki timeout."
	case "uz-Cyrl":
		spec.Title, spec.Description = "Автоматлаштиришни бажариш", "Тенантнинг бошқа автоматлаштиришини ишга туширади ва у тугашини кутади: done, failed ёки timeout."
	}
	return spec
}

func (n *RunNode) ApplyEffect(ctx context.Context, tx pgx.Tx, run engine.StepContext, _ graph.Node, e graph.Effect) (map[string]any, error) {
	return n.st.start(ctx, tx, run, e, true)
}

// EffectWaitSubject: the wait listens for the finish of the run the effect started.
func (n *RunNode) EffectWaitSubject(_, output map[string]any) (engine.SubjectRef, bool) {
	id, _ := output["run_id"].(string)
	if id == "" {
		return engine.SubjectRef{}, false
	}
	return engine.SubjectRef{Entity: engine.SubjectRun, Key: id}, true
}

// ResolveWake: done when the run finished done, failed otherwise (failed, cancelled), with its output.
func (n *RunNode) ResolveWake(ev graph.Event) (string, map[string]any) {
	status, _ := ev["status"].(string)
	out := map[string]any{"run_id": ev["runId"], "status": status, "output": ev["output"]}
	if status == string(engine.RunDone) {
		return OutcomeDone, out
	}
	return OutcomeFailed, out
}
