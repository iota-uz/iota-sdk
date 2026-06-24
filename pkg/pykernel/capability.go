package pykernel

import (
	"context"
	"errors"
	"fmt"
)

// Access classifies a capability's effect on data. The host — never the kernel
// — decides, at dispatch time, whether a write capability is permitted given
// the run's Mode.
type Access int

const (
	// AccessRead is safe in both plan and apply mode.
	AccessRead Access = iota
	// AccessWrite mutates data and is refused by the host in plan mode.
	AccessWrite
)

func (a Access) String() string {
	switch a {
	case AccessRead:
		return "read"
	case AccessWrite:
		return "write"
	default:
		return fmt.Sprintf("Access(%d)", int(a))
	}
}

// Mode is the dry-run dimension of a run, carried on the Session and propagated
// to every capability call. There is exactly one such dimension (mirroring the
// Python engine's RunMode); the consumer picks it when it builds the Session,
// and capabilities cannot bypass it.
type Mode int

const (
	// ModePlan is a read-only rehearsal: write capabilities are refused.
	ModePlan Mode = iota
	// ModeApply permits write capabilities.
	ModeApply
)

// IsWritable reports whether write capabilities are permitted in this mode.
func (m Mode) IsWritable() bool { return m == ModeApply }

func (m Mode) String() string {
	switch m {
	case ModePlan:
		return "plan"
	case ModeApply:
		return "apply"
	default:
		return fmt.Sprintf("Mode(%d)", int(m))
	}
}

// CallArgs is the validated, boundary-converted argument map for a capability
// invocation. Keys are the parameter names from the capability Signature.
type CallArgs map[string]any

// ParamSpec describes one capability parameter. It is advisory metadata used to
// synthesize the Python proxy signature; it is NOT enforced by a generic
// host-side validator. Capabilities that need argument validation must perform
// it themselves (e.g. in their invoke implementation).
type ParamSpec struct {
	Name     string
	Type     string // doc/validation hint, e.g. "str", "list[dict]"
	Required bool
}

// CapabilitySignature is the declared shape of a capability. The kernel shim
// turns it into a callable Python function; the Doc becomes that function's
// docstring, which the model sees.
type CapabilitySignature struct {
	Params  []ParamSpec
	Returns string // doc hint, e.g. "list[dict]"
	Doc     string
}

// Capability is one host function exposed into the kernel's Python namespace
// under Name. The host owns invocation; kernel code can only call it by name
// with JSON arguments. Implementations must be safe for concurrent use.
type Capability interface {
	// Name is the Python identifier exposed in the namespace (e.g. "sql").
	Name() string
	// Signature is the argument/return spec used to generate the proxy and to
	// validate inbound arguments.
	Signature() CapabilitySignature
	// Access reports read vs write, which drives plan-mode refusal centrally.
	Access() Access
	// Invoke runs the host function. ctx already carries the host-bound tenant
	// and request scope. The returned value is JSON-serialized at the boundary.
	Invoke(ctx context.Context, args CallArgs) (any, error)
}

// CapabilitySet is the immutable bundle of capabilities frozen onto a kernel at
// Acquire. A consumer's policy layer is mostly the choice of which capabilities
// populate this set plus the run Mode.
type CapabilitySet interface {
	// List returns the capabilities in registration order.
	List() []Capability
	// Lookup resolves a capability by name.
	Lookup(name string) (Capability, bool)
}

// Sentinel errors callers branch on.
var (
	// ErrPlanModeWrite is returned by Authorize/Dispatch when a write
	// capability is invoked during a plan run. The bridge maps it to a Python
	// exception raised at the call site inside the kernel.
	ErrPlanModeWrite = errors.New("pykernel: write capability refused in plan mode")
	// ErrCapabilityNotFound is returned when a call names an unregistered
	// capability.
	ErrCapabilityNotFound = errors.New("pykernel: capability not found")
	// ErrDuplicateCapability is returned by NewCapabilitySet on a name clash.
	ErrDuplicateCapability = errors.New("pykernel: duplicate capability name")
)

// NewCapabilitySet builds an immutable set, rejecting duplicate names and any
// capability with an empty name.
func NewCapabilitySet(caps ...Capability) (CapabilitySet, error) {
	set := &capabilitySet{
		order:  make([]Capability, 0, len(caps)),
		byName: make(map[string]Capability, len(caps)),
	}
	for _, c := range caps {
		if c == nil {
			return nil, errors.New("pykernel: nil capability")
		}
		name := c.Name()
		if name == "" {
			return nil, errors.New("pykernel: capability with empty name")
		}
		if _, dup := set.byName[name]; dup {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateCapability, name)
		}
		set.byName[name] = c
		set.order = append(set.order, c)
	}
	return set, nil
}

type capabilitySet struct {
	order  []Capability
	byName map[string]Capability
}

func (s *capabilitySet) List() []Capability {
	out := make([]Capability, len(s.order))
	copy(out, s.order)
	return out
}

func (s *capabilitySet) Lookup(name string) (Capability, bool) {
	c, ok := s.byName[name]
	return c, ok
}

// Authorize reports whether cap may run under mode. A write capability in plan
// mode is refused with ErrPlanModeWrite. This is the SINGLE enforcement point
// for the plan/apply boundary — the Go analogue of the Python engine's
// assert_writable. Keep all mode checks funneling through here.
func Authorize(mode Mode, c Capability) error {
	if c.Access() == AccessWrite && !mode.IsWritable() {
		return fmt.Errorf("%w: %s", ErrPlanModeWrite, c.Name())
	}
	return nil
}

// Dispatch resolves name in set, authorizes it against mode, then invokes it.
// The host-side bridge routes every kernel capability call through Dispatch so
// the mode check cannot be skipped. ctx must already carry the host-bound
// tenant and request scope from the Session.
func Dispatch(ctx context.Context, set CapabilitySet, mode Mode, name string, args CallArgs) (any, error) {
	c, ok := set.Lookup(name)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrCapabilityNotFound, name)
	}
	if err := Authorize(mode, c); err != nil {
		return nil, err
	}
	return c.Invoke(ctx, args)
}

// CapabilityFunc adapts a plain function to a Capability, mirroring the
// adapter-function style used elsewhere in the SDK.
func CapabilityFunc(
	name string,
	access Access,
	sig CapabilitySignature,
	fn func(context.Context, CallArgs) (any, error),
) Capability {
	return &capabilityFunc{name: name, access: access, sig: sig, fn: fn}
}

type capabilityFunc struct {
	name   string
	access Access
	sig    CapabilitySignature
	fn     func(context.Context, CallArgs) (any, error)
}

func (c *capabilityFunc) Name() string                   { return c.name }
func (c *capabilityFunc) Signature() CapabilitySignature { return c.sig }
func (c *capabilityFunc) Access() Access                 { return c.access }

func (c *capabilityFunc) Invoke(ctx context.Context, args CallArgs) (any, error) {
	if c.fn == nil {
		return nil, fmt.Errorf("pykernel: capability %q has no handler", c.name)
	}
	return c.fn(ctx, args)
}
