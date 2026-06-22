// Package bridge implements the host side of the pykernel host↔kernel control
// channel: a length-prefixed JSON-RPC 2.0 protocol carried over a single
// full-duplex connection (in production a per-kernel AF_UNIX socket; in tests
// any io.ReadWriteCloser, e.g. net.Pipe).
//
// The channel multiplexes two logical directions over one connection:
//
//   - host→kernel: exec.submit (run code) and exec.cancel (cooperative cancel),
//     sent as notifications.
//   - kernel→host: cap.call (a blocking request — the kernel's capability proxy
//     waits for the host's response), plus out.stdout / out.metric / out.log /
//     exec.result / exec.error / exec.done notifications that stream an exec's
//     output back.
//
// The security-critical property lives in the cap.call frame: it carries only
// {exec_id, name, args}. It does NOT carry a tenant, permissions, or the run
// Mode. The host's CallDispatcher injects those from the Session, so a kernel
// cannot forge identity or escalate privileges over the wire. A capability
// refusal (e.g. a write attempted during a plan run) comes back as a JSON-RPC
// error whose data names the Python exception type to raise at the call site.
//
// This package is standalone — it does not import pykernel. The Manager adapts
// a pykernel.CapabilitySet plus a run Mode into a CallDispatcher.
package bridge
