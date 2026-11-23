# Python worker sandbox (pykernel)

`pkg/pykernel` runs model-authored Python for the built-in chat and for MCP
`execute`. Every kernel is confined by the OS before the first instruction of
user code. This page is the contract a consumer relies on and the steps to
verify it on its own hosts.

## What a kernel can and cannot do

| Area | Guarantee | Mechanism |
|---|---|---|
| Files | Writes only its session workdir; reads only the Python runtime (stdlib, site-packages, interpreter), shared libraries, `/etc/ld.so.cache`, `/etc/localtime`, zoneinfo, `/dev/null`, `/dev/zero`, `/dev/urandom` and the host's `Profile.ReadPaths`. No `/proc`, `$HOME`, `/data`, `/tmp`, `/etc/passwd`, application binary or config. No other session's workdir is reachable, and a read rule that contains the workdir is refused. `execve` of a file written to the workdir is refused; native code loaded with `ctypes` is not a boundary (Python itself can already make any syscall) — it runs under the same Landlock and seccomp. | Landlock (ABI ≥ 6) |
| Network | No socket of any family can be created (TCP, UDP, raw, packet, netlink, unix). Only an in-process `socketpair` is possible, and every address-bearing socket call is refused (`connect`, `bind`, `listen`, `accept`, `sendmsg`, `sendto` with an address), so no pathname or abstract unix endpoint is reachable. TCP bind/connect is also denied by Landlock; abstract unix sockets are scoped. The only channel to the host is the inherited bridge fd. | seccomp + Landlock |
| Environment | Explicit allowlist (`Config.EnvAllowlist` + `ExtraEnv`); no DSN, S3 key, OIDC secret. Only fds 0–2 and the bridge are open. Data comes only through capabilities, which run in the host with the Session's tenant (and `CallContextBinder` identity) on every call. | host |
| Privileges | `no_new_privs`, empty capability set, never uid 0. Refused: `ptrace`, `process_vm_*`, `mount`, `unshare`/`setns`/namespace `clone`, `clone3`, `bpf`, `keyctl`/`add_key`, `perf_event_open`, `io_uring_*`, `setsid`/`setpgid`, host IPC (SysV shared memory, message queues, semaphores; POSIX message queues), kexec/modules/swap/reboot/clock/hostname. `prlimit64`, `setpriority`, `sched_set*`, `ioprio_set`, `migrate_pages`/`move_pages` act only on the caller itself, never on a host process of the same uid. Signals cannot leave the sandbox. A foreign syscall ABI (x32, 32-bit) kills the process. | seccomp + Landlock scope |
| Resources | `RLIMIT_AS`, `CPU`, `FSIZE`, `NPROC`, `NOFILE` set soft == hard by the kernel itself before it reads any code (no `Start`→`prlimit` window); wall-clock timeout per exec with a server cap, armed before the code is sent; workdir quota enforced at allocation time: every write-family call (`write`, `pwrite`, `writev`, `pwritev[2]`, `ftruncate`), whatever the fd, is performed by the host supervisor on a duplicate of the caller's open file description (`pidfd_getfd`), at an offset fixed once from state only the supervisor changes, with data copied out of the caller in chunks — so what is checked is exactly what is written. Usage is charged in 4 KiB blocks plus one block per entry (files, directories, symlinks); entry creation, including `O_TMPFILE` and `memfd_create`, is counted before it runs, and unnamed inodes the kernel holds — by an fd or only by a mapping — stay charged (an entry and their bytes) until their last reference closes; sizes the kernel produced stay charged per inode while any fd or mapping holds it (unlinked or mapped files included). Writes go through the host's own description of the file, so fd flags the kernel changes (`O_APPEND`) cannot move them. Past the quota a write is cut short and then fails with `EDQUOT` (`EFBIG` past `FileSizeBytes`). The filter waits killably once the host has the call (`WAIT_KILLABLE_RECV`), so a signal never repeats a performed write; stream writes (pipes, sockets) are non-blocking on the host and bounded per kernel. `fallocate`, path `truncate`, AIO, `copy_file_range`, `sendfile`, `splice`, `setxattr` and every `ioctl` but terminal queries are refused (`shutil` falls back to plain writes). A reconcile over the quota, or a lost supervisor, kills the kernel. While no exec runs the kernel can start no thread or process, and CPU it keeps using then (after a worker-reported done) is charged against a 1 s budget that kills it. `isolation.Check` proves the host can act for a confined process (`pidfd_getfd`, memory reads with the kernel's credentials); `manager.New` additionally proves three enforcement paths with a test kernel. | shim + host supervisor |
| Exec lifetime | The worker's own "done" is not trusted: the host ends an exec only when it sees the kernel at rest — processes the exec started are killed (through a pidfd, after their group membership is verified) and no kernel thread is running — and then stops the whole kernel (`SIGSTOP`, verified) before the consumer is told. Between execs the kernel is frozen: a thread left sleeping or a timer cannot resume until a later exec, whose watchdog, quota and capability context then bound it. Code that claims to be done and keeps running ends with `ReasonTimeout`. | host |
| Cancel | Cancelling an exec, its timeout or a quota stop cancel the context of running capability calls (nested SQL/tools) at once, interrupt the code, and free the slot. A capability call that does not belong to the running exec (a thread that outlived it) is refused. | host |

## Fail closed

`manager.New` confines by default (`isolation.Profile{}` = `ModeRequired`). It
starts a real child with the configured interpreter and limits, which applies
the sandbox and probes it (`isolation.Check`). If any boundary in
`isolation.RequiredBoundaries` is missing, `New` returns an error wrapping
`isolation.ErrIsolationUnavailable` with the list; an unknown `Mode` value is
an error too. Notifications from the interpreter (output, `exec.done`) are
untrusted: they never stop the host's timers, quota supervisor or idle guard. Every spawned kernel repeats
the report in its `kernel.ready` handshake and is refused unless every
required boundary is in force.

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
`execute.NewRunner` refuses a Manager whose `Isolation()` report is not
complete and verified unless `InsecureAllowUnconfinedRuntime` is set.

Requirements: Linux with Landlock ABI ≥ 6 (kernel 6.12+) in the LSM list,
seccomp with user notification and `WAIT_KILLABLE_RECV` (5.19+), `pidfd_getfd`,
CPython with `ctypes`. Namespaces, `bwrap` and `nsjail` are not used,
so it works in containers that forbid them (Railway).

### Running as root

`RLIMIT_NPROC` is not enforced for uid 0, so a root host must set
`Profile.RunAs`. The Manager hands every session workdir to that uid before a
lease and makes its runtime directory readable. The workdir's parent
directories must be traversable (`o+x`) by it. When the application is PID 1 of
a container, the Manager reaps the processes of a disposed kernel that were
reparented to it.

The quota supervisor acts for the kernel with `pidfd_getfd`. Docker's default
seccomp profile allows it only with `CAP_SYS_PTRACE`, so a Docker deployment
grants the application that capability (`--cap-add SYS_PTRACE`); kernels drop
every capability and their own filter refuses `ptrace`. Without it
`manager.New` fails closed and says so.

### Known limits

- `RLIMIT_NPROC` counts every task of the kernel uid. Kernels sharing one
  `RunAs` uid share that budget: a fork bomb in one kernel is contained (it
  cannot hurt the host and is killed with its kernel), but until then other
  kernels of the uid may fail to start threads.
- The supervisor acts for the kernel with the kernel's own credentials
  (`pidfd_getfd`, `process_vm_readv`, `/proc`), which needs Linux ≥ 5.6 and
  Yama `ptrace_scope` ≤ 1. Where it cannot, the call fails (`EPERM`) instead of
  running unmetered. An emulated stream write moves at most 16 MiB per call
  (a short write, as POSIX allows), and at most 16 can wait per kernel; more
  fail with `EAGAIN`.
- An exec ends with the processes it started: children still running when the code returns are killed.
- The restart record lives next to the workdir (`.<name>.pykernel`); remove it
  with the workdir.
- `FreshNamespace` clears the user's names and reinstalls capabilities; process
  state (threads, imported and patched modules, `os.environ`, children) stays
  with the process. Use the ephemeral policy when no state may carry over.

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
