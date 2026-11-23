# Python worker sandbox (pykernel)

`pkg/pykernel` runs model-authored Python for the built-in chat and for MCP
`execute`. Every kernel is confined by the OS before the first instruction of
user code. This page is the contract a consumer relies on and the steps to
verify it on its own hosts.

## What a kernel can and cannot do

| Area | Guarantee | Mechanism |
|---|---|---|
| Files | Writes only its session workdir; reads only the Python runtime (stdlib, site-packages, interpreter), shared libraries, `/etc/ld.so.cache`, `/etc/localtime`, zoneinfo, `/dev/null`, `/dev/zero`, `/dev/urandom` and the host's `Profile.ReadPaths`. No `/proc`, `$HOME`, `/data`, `/tmp`, `/etc/passwd`, application binary or config. Nothing written to the workdir can be executed. | Landlock (ABI ≥ 6) |
| Network | No socket of any family can be created (TCP, UDP, raw, packet, netlink, unix). TCP bind/connect is also denied by Landlock; abstract unix sockets are scoped. The only channel to the host is the inherited bridge fd. | seccomp + Landlock |
| Environment | Explicit allowlist (`Config.EnvAllowlist` + `ExtraEnv`); no DSN, S3 key, OIDC secret. Only fds 0–2 and the bridge are open. Data comes only through capabilities, which run in the host with the Session's tenant (and `CallContextBinder` identity) on every call. | host |
| Privileges | `no_new_privs`, empty capability set, never uid 0. Refused: `ptrace`, `process_vm_*`, `mount`, `unshare`/`setns`/namespace `clone`, `clone3`, `bpf`, `keyctl`/`add_key`, `perf_event_open`, `io_uring_*`, `setsid`/`setpgid`, kexec/modules/swap/reboot/clock/hostname. Signals cannot leave the sandbox. A foreign syscall ABI (x32, 32-bit) kills the process. | seccomp + Landlock scope |
| Resources | `RLIMIT_AS`, `CPU`, `FSIZE`, `NPROC`, `NOFILE` set soft == hard by the kernel itself before it reads any code (no `Start`→`prlimit` window); wall-clock timeout per exec with a server cap; workdir quota; output cap with explicit truncation flags. | shim + host |
| Cancel | Cancelling an exec, its timeout or a quota stop cancel the context of running capability calls (nested SQL/tools) at once, interrupt the code, and free the slot. | host |

## Fail closed

`manager.New` confines by default (`isolation.Profile{}` = `ModeRequired`). It
starts a real child with the configured interpreter and limits, which applies
the sandbox and probes it (`isolation.Check`). If any boundary in
`isolation.RequiredBoundaries` is missing, `New` returns an error wrapping
`isolation.ErrIsolationUnavailable` with the list. Every spawned kernel repeats
the report in its `kernel.ready` handshake and is refused when it differs.

```go
m, err := manager.New(manager.Config{
    Policy:       lifecycle.WarmPool(15*time.Minute, 8),
    EnvAllowlist: []string{"PATH", "LANG"},
    Sandbox: isolation.Profile{
        RunAs:     &isolation.Credential{UID: 10001, GID: 10001}, // required when the app runs as root
        ReadPaths: []string{"/usr/share/fonts"},
    },
    Limits: isolation.ResourceLimits{AddressSpaceBytes: 2 << 30, WorkdirBytes: 512 << 20},
})
if err != nil {
    // keep execute disabled (dependency_unavailable); err lists what is missing
}
report := m.Isolation() // report.Err() == nil, report.Verified == true
```

`isolation.Profile{Mode: isolation.ModeInsecureDisabled}` turns the sandbox
off for development on macOS or old kernels. Never use it for real input.

Requirements: Linux with Landlock ABI ≥ 6 (kernel 6.12+) in the LSM list,
seccomp, CPython with `ctypes`. Namespaces, `bwrap` and `nsjail` are not used,
so it works in containers that forbid them (Railway).

### Running as root

`RLIMIT_NPROC` is not enforced for uid 0, so a root host must set
`Profile.RunAs`. The Manager hands every session workdir to that uid before a
lease and makes its runtime directory readable. The workdir's parent
directories must be traversable (`o+x`) by it. When the application is PID 1 of
a container, the Manager reaps the processes of a disposed kernel that were
reparented to it.

## Terminal events

Every exec stream ends with exactly one `EventDone` whose `DonePayload` says:

- `Reason`: `completed`, `error`, `cancelled`, `timeout`, `quota_exceeded`,
  `kernel_exited`;
- `StdoutTruncated`, `StderrTruncated`, `EventsDropped` (`Truncated()`): output
  lost to the cap or to a consumer that stopped reading — the bridge never
  blocks on a consumer, so the slot is freed and the next exec runs;
- `NamespaceReset`, `FilesPreserved`: the namespace of an earlier kernel of the
  session is gone (respawn, eviction, host restart, `FreshNamespace` reuse)
  while its workdir files are intact. Nothing is re-executed to restore it.

`ErrKernelBusy` and `ErrPoolExhausted` are returned before any code runs.

## Conformance suite

`pkg/pykernel/conformance` probes every boundary from inside a real kernel and
checks host-side listeners: reading `/proc/1/environ`, `$HOME`, the app binary
and `/etc`; writing outside the workdir; executing from it; TCP and UDP to the
internet, to the database and to a host listener; raw/netlink and abstract or
pathname unix sockets; `ptrace`, `process_vm_readv`, signals; namespaces,
mount, bpf, keyrings, perf, io_uring; capabilities and `no_new_privs`; rlimits;
memory exhaustion; a fork bomb (and that every forked process is reaped); the
workdir quota; cancellation and timeouts reaching a capability; truncation and
a stalled consumer.

Run it with the Manager configuration used in production:

```go
rep := conformance.Run(ctx, conformance.Options{Manager: m, DatabaseAddrs: []string{dbHostPort}})
if err := rep.Err(); err != nil { /* not conformant */ }
```

or on a copy of the environment with the command:

```sh
go run github.com/iota-uz/iota-sdk/cmd/pykernel-conformance@<sdk-version> \
  -python python3 -db "$DB_HOST:$DB_PORT" [-run-as-uid 10001]
```

It prints the isolation report and every probe as JSON and exits non-zero on
any failure. SDK CI runs it on GitHub runners and as root in a
`python:3.11-slim` container (`PYKERNEL_CONFORMANCE=required`).

Enable `execute` and MCP writes only on hosts where the suite passes.
