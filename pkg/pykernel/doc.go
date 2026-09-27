// Package pykernel runs untrusted, AI-agent-authored Python inside a sandboxed
// subprocess and exposes a small, host-controlled capability surface to it.
//
// It is a reusable primitive shared by two consumers:
//
//   - the Ali analyst REPL — a warm, per-session kernel with a read-only sql()
//     capability;
//   - the data-migration engine — an ephemeral, per-run kernel with
//     write-capable connector capabilities gated by a plan/apply mode.
//
// One primitive serves both; the only difference is the capability set, the
// run Mode, and the lifecycle policy (see the lifecycle subpackage).
//
// # The host-binds-context invariant
//
// Everything security-relevant is decided on the HOST and is never supplied by
// the kernel. A Session carries the tenant id, the capability set, the run
// Mode, and the workdir; the Manager freezes these onto a kernel at Acquire and
// they cannot be changed or escalated for the life of the lease. When kernel
// code calls a capability, the wire frame carries only the capability name and
// its arguments — the dispatcher injects tenant and Mode from the Session.
// A kernel therefore cannot forge a tenant, widen its permissions, or turn a
// plan run into a writing run.
//
// # Plan/apply enforcement is centralized
//
// Each Capability declares an Access (read or write). The plan/apply boundary
// is enforced in exactly one place — Authorize / Dispatch — which refuses a
// write capability while the run is in plan mode (ErrPlanModeWrite). This is the
// Go analogue of the Python engine's assert_writable: a single, greppable choke
// point rather than per-capability checks scattered through the code. A refused
// call surfaces inside the kernel as a raised Python exception, so an agent that
// tries to write during a dry run fails loudly instead of silently no-op-ing.
//
// # Defense in depth
//
// Authorization is the policy boundary, not the only one. The kernel runs with
// no database/credentials in its environment and no network egress, so the only
// path to data is a capability call over the bridge; rlimits and a wall-clock
// timeout bound resource use; the workdir is jailed. Those mechanisms live in
// the isolation and bridge subpackages.
//
// # Error convention
//
// pykernel deliberately uses the stdlib errors/fmt.Errorf rather than the SDK
// pkg/serrors convention because it is a low-level primitive with no tenant or
// operation context to carry; callers wrap pykernel errors with serrors at the
// call site. Wrap with fmt.Errorf("...: %w", err) and expose sentinel errors
// for conditions callers branch on.
package pykernel
