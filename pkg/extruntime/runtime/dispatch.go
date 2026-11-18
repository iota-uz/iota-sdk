package runtime

import (
	"context"
	"encoding/json"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// HostDispatcher is the native.Dispatcher of an application with the
// extension runtime: Go callers (controllers, services) invoke native
// commands, operations and queries as root invocations of the host, as the
// acting session user. The host selects the active implementation (a package
// replacement bound to the contract, otherwise the Go default), validates,
// authorizes, runs the guards of packages and records the invocation.
type HostDispatcher struct {
	Host *host.Host
	// Subject resolves who acts; default SubjectFromContext (the session
	// user of the request).
	Subject func(ctx context.Context) (spi.Subject, error)
	// Trigger is recorded on the invocations; default TriggerUI.
	Trigger host.TriggerKind
}

var _ native.Dispatcher = (*HostDispatcher)(nil)

// Dispatch implements native.Dispatcher. Errors keep their Go chain: a
// structured runtime error (validation, denied, domain) is a *pool.WireError
// (native.AsError), a Go default's own error is returned as it was raised.
func (d *HostDispatcher) Dispatch(ctx context.Context, contract string, input json.RawMessage) (json.RawMessage, error) {
	subject := d.Subject
	if subject == nil {
		subject = SubjectFromContext
	}
	sub, err := subject(ctx)
	if err != nil {
		return nil, err
	}
	trig := d.Trigger
	if trig == "" {
		trig = host.TriggerUI
	}
	res, err := d.Host.Invoke(ctx, host.Request{
		Contract: contract, Input: input, Subject: sub,
		Trigger: host.Trigger{Kind: trig, Ref: contract},
	})
	if err != nil {
		return nil, err
	}
	return res.Output, nil
}

// CommandDispatcher is the dispatcher Go code of this application uses for
// native contracts.
func (rt *Runtime) CommandDispatcher() *HostDispatcher {
	return &HostDispatcher{Host: rt.Host}
}
