# shell-worker

The isolated runtime behind the SDK `shell` capability. An application talks to
it only through [`pkg/shell`](../) (standard library only); this directory is a
separate Go module (Go 1.26) so the SDK and applications stay on their own
toolchain and never import gbash.

## How a run works

```
application ── pkg/shell.Runner ── HTTPExecutor ──► supervisor (POST /v1/runs, bearer token)
                 │  validate, resolve file_ref                │ bounded queue, 1 active run
                 │  publish /output → file_ref                ▼
                 ◄──────────── result + /output files ── child process (fresh per run)
                                                              gbash, in-memory VFS
```

- **Supervisor** (`shell-worker`): authenticated internal API, bounded queue
  (one active run per replica, `SHELL_WORKER_QUEUE_SIZE` waiting), retryable
  `busy` with `retry_after_ms` on overflow. Every run is a new child process with
  an empty environment, its own process group and a hard deadline
  (`timeout_ms` + 3 s on the child process, which leaves the child room to
  decode its request and encode its result at their size limits); the group is
  SIGKILLed and reaped on deadline or when the client goes away. Reading the
  HTTP request and writing the response happen outside this deadline. gbash's
  own JSON-RPC server is not used.
- **Child** (`shell-worker child`): applies its limits before reading the
  request (`oom_score_adj=1000`, `RLIMIT_FSIZE=0`, `RLIMIT_CORE=0`,
  `RLIMIT_NOFILE=64`), a Go heap budget with a watchdog
  (`SHELL_WORKER_CHILD_MEMORY_BYTES`), then runs one script and exits.
- **Sandbox**: gbash with an in-memory VFS only — `/input` (read-only inputs),
  `/tmp` (scratch), `/output` (results). No host filesystem, no
  `WithWorkspace`/`WithHost`, no network client. Symlink traversal is denied,
  hard links are refused, and `/tmp`+`/output` are capped at 64 MiB on every
  write (including copies, archive extraction and `truncate`). Only regular
  files in `/output` are returned.
- **Budgets** are per run and shared by nested `bash`/`sh` and `awk system()`:
  10 000 commands, 10 000 loop iterations per loop, 100 000 glob operations,
  substitution depth 50.

`RLIMIT_AS` is deliberately not used: the Go runtime reserves address space far
beyond its heap and fails at start under any useful value. The hard memory bound
is the container limit; the child is the preferred OOM victim.

## Commands

Fixed registry: `awk` and `jq` (gbash contrib, pinned), `grep`, `sed`, `find`,
`sort`, `uniq`, `cut`, `head`, `tail`, `wc`, `diff`, `tr`, `tee`, `cat`, `paste`,
`join`, `comm`, `column`, `xargs`, file operations inside the VFS (`ls`, `cp`,
`mv`, `rm`, `mkdir`, `touch`, `tar`, `gzip`, …) and `bash`/`sh` for nested
scripts. See `internal/sandbox/registry.go`. Not registered: `curl`/`wget`,
package managers, identity and process commands, links, other shells. Unknown
commands fail with exit 127 and never reach the host.

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `SHELL_WORKER_TOKEN` | — (required) | Bearer token of the internal API; removed from the environment at start |
| `SHELL_WORKER_ADDR` | `:8080` | Listen address |
| `SHELL_WORKER_QUEUE_SIZE` | `4` | Runs allowed to wait behind the active one; see the note on queue wait below |
| `SHELL_WORKER_RETRY_AFTER_MS` | `1000` | `retry_after_ms` returned with `busy` |
| `SHELL_WORKER_CHILD_MEMORY_BYTES` | `268435456` | Go heap budget of one run |

`HTTPExecutor.Execute` bounds a call without a deadline at 2 minutes, and that
time includes waiting in the worker queue. A run takes at most about 14 s
(10 s limit, 3 s kill grace, 1 s reap), so the default queue drains well within
the bound. From `SHELL_WORKER_QUEUE_SIZE=8` up, a queued run can outlast it and
the caller gets `context.DeadlineExceeded`. What happens to the run depends on
when the deadline passes: a run that has not yet taken the active slot is
dropped unexecuted, while a run that already started is killed by the worker
when the caller's context ends. With a larger queue, give `Execute` a context
deadline that covers the whole queue.

`GET /healthz` returns 204 without authentication.

## Deployment

```sh
docker build -t iotauz/shell-worker:<tag> pkg/shell/worker
```

Run it as its own service, never inside the application container:

- `--memory 512m`, `--read-only`, `--cap-drop ALL`,
  `--security-opt no-new-privileges`, a small `--pids-limit`; the image runs as
  `nonroot` on distroless/static.
- No volumes, host mounts or Docker socket; no database credentials or
  application tokens in its environment.
- Network: reachable only from the application on the internal network, with
  egress denied (Kubernetes NetworkPolicy, an internal-only Docker network, or
  the platform equivalent). The guest has no network commands regardless; the
  policy keeps it that way even if a runtime bug is found.

## Wiring it into an application

```go
runner := shell.NewRunner(
	shell.NewHTTPExecutor(os.Getenv("SHELL_WORKER_URL"), os.Getenv("SHELL_WORKER_TOKEN"), nil),
	fileResolver,      // shell.FileResolver: opens a file_ref for the current user/tenant
	artifactPublisher, // shell.ArtifactPublisher: stores /output files as private files
)
res, err := runner.Run(ctx, shell.Request{Script: script, Files: files})
```

A tool adapter (for example the MCP `shell` tool) decodes its arguments into
`shell.Request`, returns `*shell.Result` as the tool result and maps
`*shell.Error` (`invalid_input`, `forbidden`, `limit_exceeded`, `busy`,
`dependency_unavailable`) to a tool error. A non-zero `exit_code` is a normal
result. Register the tool only when the worker is configured; its absence does
not affect other tools. Runs are never retried automatically.
