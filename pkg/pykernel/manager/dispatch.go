package manager

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/pykernel"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/bridge"
)

// capSpec mirrors one entry the Python shim reads from PYKERNEL_CAPABILITIES to
// synthesize a callable proxy.
type capSpec struct {
	Name    string     `json:"name"`
	Params  []capParam `json:"params"`
	Returns string     `json:"returns"`
	Doc     string     `json:"doc"`
}

type capParam struct {
	Name string `json:"name"`
}

func capSpecsJSON(set pykernel.CapabilitySet) ([]byte, error) {
	specs := make([]capSpec, 0)
	for _, c := range set.List() {
		sig := c.Signature()
		params := make([]capParam, 0, len(sig.Params))
		for _, p := range sig.Params {
			params = append(params, capParam{Name: p.Name})
		}
		specs = append(specs, capSpec{Name: c.Name(), Params: params, Returns: sig.Returns, Doc: sig.Doc})
	}
	return json.Marshal(specs)
}

// dispatcher binds a kernel's capability set, run Mode and tenant, and routes
// every kernel cap.call through the central plan/apply authorization. The
// kernel supplies only the capability name and arguments; tenant and Mode are
// bound here from the Session.
type dispatcher struct {
	caps   pykernel.CapabilitySet
	mode   pykernel.Mode
	tenant uuid.UUID
}

func (d *dispatcher) Dispatch(ctx context.Context, call bridge.Call) bridge.Reply {
	ctx = composables.WithTenantID(ctx, d.tenant)

	var args pykernel.CallArgs
	if len(call.Args) > 0 {
		if err := json.Unmarshal(call.Args, &args); err != nil {
			return bridge.Reply{Err: &bridge.CallError{Type: "ValueError", Message: "invalid arguments: " + err.Error()}}
		}
	}

	out, err := pykernel.Dispatch(ctx, d.caps, d.mode, call.Name, args)
	if err != nil {
		return bridge.Reply{Err: capError(err)}
	}

	raw, err := json.Marshal(out)
	if err != nil {
		return bridge.Reply{Err: &bridge.CallError{Type: "TypeError", Message: "result is not JSON-serializable: " + err.Error()}}
	}
	return bridge.Reply{Result: raw}
}

// capError maps a dispatch error to the Python exception type the shim raises.
func capError(err error) *bridge.CallError {
	switch {
	case errors.Is(err, pykernel.ErrPlanModeWrite):
		return &bridge.CallError{Type: "PlanModeViolation", Message: err.Error()}
	case errors.Is(err, pykernel.ErrCapabilityNotFound):
		return &bridge.CallError{Type: "CapabilityNotFound", Message: err.Error()}
	default:
		return &bridge.CallError{Type: "CapabilityError", Message: err.Error()}
	}
}

// translate converts a bridge output notification into a pykernel.ExecEvent and
// reports whether it terminates the stream.
func translate(ev bridge.RawEvent) (pykernel.ExecEvent, bool) {
	out := pykernel.ExecEvent{Timestamp: time.Now()}
	switch ev.Kind {
	case bridge.KindStdout:
		var p struct {
			Chunk string `json:"chunk"`
		}
		_ = json.Unmarshal(ev.Params, &p)
		out.Kind = pykernel.EventStdout
		out.Stdout = []byte(p.Chunk)
	case bridge.KindMetric:
		var p struct {
			Name  string            `json:"name"`
			Value float64           `json:"value"`
			Tags  map[string]string `json:"tags"`
		}
		_ = json.Unmarshal(ev.Params, &p)
		out.Kind = pykernel.EventMetric
		out.Metric = &pykernel.MetricPayload{Name: p.Name, Value: p.Value, Tags: p.Tags}
	case bridge.KindLog:
		var p struct {
			Level   string         `json:"level"`
			Message string         `json:"message"`
			Fields  map[string]any `json:"fields"`
		}
		_ = json.Unmarshal(ev.Params, &p)
		out.Kind = pykernel.EventLog
		out.Log = &pykernel.LogPayload{Level: p.Level, Message: p.Message, Fields: p.Fields}
	case bridge.KindResult:
		var p struct {
			Text      string `json:"text"`
			MIME      string `json:"mime"`
			Data      []byte `json:"data"`
			Truncated bool   `json:"truncated"`
		}
		_ = json.Unmarshal(ev.Params, &p)
		out.Kind = pykernel.EventResult
		out.Result = &pykernel.ResultPayload{Text: p.Text, MIME: p.MIME, Data: p.Data, Truncated: p.Truncated}
	case bridge.KindError:
		var p struct {
			Type      string `json:"type"`
			Message   string `json:"message"`
			Traceback string `json:"traceback"`
		}
		_ = json.Unmarshal(ev.Params, &p)
		out.Kind = pykernel.EventError
		out.Err = &pykernel.ExecError{Type: p.Type, Message: p.Message, Traceback: p.Traceback}
	case bridge.KindDone:
		out.Kind = pykernel.EventDone
		return out, true
	default:
		out.Kind = pykernel.EventLog
		out.Log = &pykernel.LogPayload{Level: "debug", Message: "unknown kernel event: " + ev.Kind}
	}
	return out, false
}
