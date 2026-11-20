package graph

import "fmt"

// DryRunInput is one input of a dry run: an event, or the timeout of the wait the run sleeps on.
type DryRunInput struct {
	Event   Event
	Timeout bool
}

// DryRunOptions adjust a dry run.
type DryRunOptions struct {
	// CallError is the product's check of a call's rendered request, the one its engine makes before
	// calling: a non-empty error sends the node down its failed outcome without reading the given answer.
	CallError func(Effect) string
}

// TraceStep is one step of a dry run's trace.
type TraceStep struct {
	Node         string         `json:"node"`
	Outcome      string         `json:"outcome"`
	ContextPatch map[string]any `json:"context_patch"`
	End          map[string]any `json:"end,omitempty"`
	Token        string         `json:"token,omitempty"`
}

// Trace outcomes besides node outcomes.
const (
	TraceWaiting        = "waiting"
	TraceEnded          = "ended"
	TraceEnd            = "end"
	TraceNotTriggered   = "not_triggered"
	TraceMissingContext = "missing_context"
	TraceFailed         = "failed"
)

const maxDryRunSteps = 500

// DryRun runs the graph over the inputs with the given context and applies nothing. The first event a
// trigger takes starts the run; later ones feed its waits. An effect is traced as its rendered params; a
// call's answer is the context's nodes.<id> (output, and outcome for a call whose answer does not name
// it). A field a step needs and the context lacks ends the trace with missing_context and the field's
// path; a run still waiting after the last input ends with waiting. An input no trigger takes leaves the
// trace with not_triggered and, per trigger of the event's kind, the condition that failed.
func (i *Interpreter) DryRun(subject Subject, ctx Context, inputs []DryRunInput, o DryRunOptions) []TraceStep {
	_ = subject // a run is about its subject; the interpreter reads it through ctx.Vars
	given := ctx.Nodes
	run := Context{Event: map[string]any{}, Nodes: map[string]NodeState{}, Vars: ctx.Vars, Now: ctx.Now, Tenant: ctx.Tenant}
	for id, st := range ctx.Nodes {
		run.Nodes[id] = st
	}
	var trace []TraceStep
	started, ended := false, false
	waiting := ""

	stop := func(node string, s Step) {
		if s.Kind == StepMissingContext {
			trace = append(trace, TraceStep{Node: node, Outcome: TraceMissingContext, ContextPatch: map[string]any{"path": s.Path}})
		} else {
			trace = append(trace, TraceStep{Node: node, Outcome: TraceFailed, ContextPatch: map[string]any{"code": s.Code, "message": s.Message}})
		}
		ended = true
	}
	patch := func(effects []Effect, output map[string]any) map[string]any {
		p := map[string]any{}
		if len(output) > 0 {
			p["output"] = output
		}
		if len(effects) > 0 {
			p["effects"] = effects
		}
		return p
	}
	takeFailed := func(node, message string) (string, bool) {
		failedOutcome := i.FailedOutcome(node)
		if failedOutcome == "" {
			return "", false
		}
		out := map[string]any{"error": message}
		run.Nodes[node] = NodeState{Output: out}
		trace = append(trace, TraceStep{Node: node, Outcome: failedOutcome, ContextPatch: map[string]any{"output": out}})
		return failedOutcome, true
	}
	advance := func(from, outcome string) {
		current, ok := i.Next(from, outcome)
		for steps := 0; ok && !ended && steps < maxDryRunSteps; steps++ {
			if current == Ignore {
				trace = append(trace, TraceStep{Node: Ignore, Outcome: TraceEnded, ContextPatch: map[string]any{}})
				ended = true
				return
			}
			node, exists := i.view.Node(current)
			if !exists {
				stop(current, failed(FailInvalidGraph, "no node %s", current))
				return
			}
			step := i.Start(node.ID, &StepContext{Ctx: run})
			if step.Kind == StepFailed {
				if fo, took := takeFailed(node.ID, step.Message); took {
					current, ok = i.Next(node.ID, fo)
					continue
				}
			}
			if step.stops() {
				stop(node.ID, step)
				return
			}
			switch step.Kind {
			case StepEnd:
				trace = append(trace, TraceStep{Node: node.ID, Outcome: TraceEnd, ContextPatch: map[string]any{}, End: step.End})
				ended = true
				return
			case StepCall:
				refused := ""
				if o.CallError != nil {
					for _, e := range step.Effects {
						if refused = o.CallError(e); refused != "" {
							break
						}
					}
				}
				if refused != "" {
					fo, took := takeFailed(node.ID, refused)
					if !took {
						stop(node.ID, failed(FailInvalidRequest, "%s", refused))
						return
					}
					current, ok = i.Next(node.ID, fo)
					continue
				}
				outcome, output, st := i.callGiven(node.ID, given)
				if st != nil {
					stop(node.ID, *st)
					return
				}
				trace = append(trace, TraceStep{Node: node.ID, Outcome: outcome, ContextPatch: patch(step.Effects, output)})
				run.Nodes[node.ID] = NodeState{Output: output}
				current, ok = i.Next(node.ID, outcome)
				continue
			case StepWait:
				out := map[string]any{"waits_for": step.Wait.Event}
				if step.Wait.Event == "" {
					out = map[string]any{"timeout_ms": step.Wait.Timeout.Milliseconds()}
				}
				trace = append(trace, TraceStep{Node: node.ID, Outcome: TraceWaiting, ContextPatch: patch(step.Effects, out)})
				waiting = node.ID
				return
			}
			trace = append(trace, TraceStep{Node: node.ID, Outcome: step.Outcome, ContextPatch: patch(step.Effects, step.Output)})
			if step.Output != nil {
				run.Nodes[node.ID] = NodeState{Output: step.Output}
			}
			current, ok = i.Next(node.ID, step.Outcome)
		}
	}

	var missed []map[string]any
	for _, input := range inputs {
		if ended {
			break
		}
		if !started {
			if input.Timeout {
				continue
			}
			for _, t := range i.view.graph.Triggers {
				evCtx := run
				evCtx.Event = input.Event
				m, st := i.TriggerMismatch(t.ID, input.Event, evCtx)
				if st != nil {
					stop(t.ID, *st)
					break
				}
				if m != "" {
					if m != MismatchEvent {
						missed = append(missed, map[string]any{"trigger": t.ID, "failed": m})
					}
					continue
				}
				run.Event = map[string]any(input.Event)
				trace = append(trace, TraceStep{Node: t.ID, Outcome: TriggerOutcome, ContextPatch: map[string]any{}})
				started = true
				advance(t.ID, TriggerOutcome)
				break
			}
			continue
		}
		if waiting == "" {
			continue
		}
		w := waiting
		if input.Timeout {
			intent, _ := i.WaitIntentOf(w)
			if intent.Timeout == 0 {
				continue
			}
			outcome, _ := i.Wake(w, nil)
			trace = append(trace, TraceStep{Node: w, Outcome: outcome, ContextPatch: map[string]any{}})
			waiting = ""
			advance(w, outcome)
			continue
		}
		matched, st := i.WaitMatches(w, input.Event, run)
		if st != nil {
			stop(w, *st)
			break
		}
		if !matched {
			continue
		}
		outcome, output := i.Wake(w, input.Event)
		run.Nodes[w] = NodeState{Output: output}
		trace = append(trace, TraceStep{Node: w, Outcome: outcome, ContextPatch: map[string]any{"output": output}})
		waiting = ""
		advance(w, outcome)
	}
	if len(trace) == 0 {
		node := "trigger"
		if len(missed) > 0 {
			node, _ = missed[0]["trigger"].(string)
		} else if len(i.view.graph.Triggers) > 0 {
			node = i.view.graph.Triggers[0].ID
		}
		p := map[string]any{}
		if len(missed) > 0 {
			p["triggers"] = missed
		}
		trace = append(trace, TraceStep{Node: node, Outcome: TraceNotTriggered, ContextPatch: p})
	}
	return trace
}

// callGiven is the answer a dry run takes for a call from the given nodes.<id>: its output and, by the
// node type's answer rule, its outcome; a node whose answer does not name its outcome (a Code node) reads
// nodes.<id>.outcome, which may be left out when the node has one outcome besides its failed one.
func (i *Interpreter) callGiven(nodeID string, given map[string]NodeState) (string, map[string]any, *Step) {
	st, ok := given[nodeID]
	if !ok || st.Output == nil {
		s := Step{Kind: StepMissingContext, Path: fmt.Sprintf("nodes.%s.output", nodeID), Message: "no answer for this call"}
		return "", nil, &s
	}
	node, _ := i.view.Node(nodeID)
	spec := i.view.catalog.Nodes[node.Type]
	if spec.Answer != nil {
		outcome, s := i.CallOutcome(nodeID, st.Output)
		if s != nil {
			return "", nil, s
		}
		return outcome, st.Output, nil
	}
	if s := i.tooLarge(st.Output, "the answer"); s != nil {
		return "", nil, s
	}
	outcomes := i.view.Outcomes(nodeID)
	if st.Outcome != "" {
		if !contains(outcomes, st.Outcome) {
			s := failed(FailUnknownOutcome, "%s has no outcome %s", nodeID, st.Outcome)
			return "", nil, &s
		}
		return st.Outcome, st.Output, nil
	}
	var taken []string
	for _, o := range outcomes {
		if o != spec.FailedOutcome {
			taken = append(taken, o)
		}
	}
	if len(taken) == 1 {
		return taken[0], st.Output, nil
	}
	s := Step{Kind: StepMissingContext, Path: fmt.Sprintf("nodes.%s.outcome", nodeID), Message: fmt.Sprintf("%s has outcomes %v: name the one taken", nodeID, taken)}
	return "", nil, &s
}
