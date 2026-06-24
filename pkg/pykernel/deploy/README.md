# pykernel-base image

The co-located Python interpreter the [`pykernel`](../) manager spawns. The
manager runs `python3 <shim>` as a sandboxed subprocess; this image is what
provides that `python3` (plus the libraries an agent program may import).

## Why a dedicated image

A Granite/SDK application binary ships on **Alpine/musl**, but `pandas` / `numpy`
/ `duckdb` publish only **glibc** (manylinux) wheels — they won't import on musl.
So the kernel needs a glibc CPython on `PATH`. This image is that runtime: a
`python:3.11-slim-bookworm` base with the analysis libraries pre-installed. The
application's final image stage builds **`FROM` this image** and copies its
static (CGO-free) Go binary in — the binary runs unchanged on glibc.

The shim (`pkg/pykernel/runtime/bootstrap.py`) is **not baked in** — the Go
manager delivers it at spawn time (`//go:embed` → per-run workdir). The image's
only job is `python3` + the importable libraries.

## Contents

`requirements.txt` (the "analysis" profile, shared with the Ali REPL and the
datamig engine):

| lib | for |
|---|---|
| numpy, pandas | dataframes / transforms over capability-returned data |
| duckdb | local SQL over in-memory data |
| openpyxl | reading/writing `.xlsx` (some migrations ingest spreadsheets) |
| matplotlib | charts (Ali analyst) |

### Why no DB drivers

There are deliberately **no** `psycopg` / `pymongo` / `minio` / `mysql` packages.
The kernel has **no DSN and no network egress** — every byte of data reaches it
through a host capability over the bridge — so a driver would be both unusable
and an unnecessary risk surface. (This diverges from the original plan's literal
"full tag (+pymongo/minio/mysql)"; the capability model makes those dead weight.)

## Build & smoke test

```sh
cd pkg/pykernel/deploy
docker build --platform linux/amd64 -t iotauz/pykernel-base:3.11-analysis .
docker run --rm --platform linux/amd64 iotauz/pykernel-base:3.11-analysis
# → python 3.11.x; shim stdlib OK; numpy/pandas/duckdb/openpyxl/matplotlib OK
```

`smoke_test.py` is copied into the image at `/opt/pykernel/smoke_test.py` so a CI
step or container healthcheck can self-verify the wheels loaded (it asserts the
shim's stdlib imports **and** runs a trivial op per analysis lib, exiting non-zero
on any failure).

Publish to wherever the SDK images live (same registry as `iotauz/sdk:base-*`):

```sh
docker push iotauz/pykernel-base:3.11-analysis
```

## Adopting it in an application image

Point the application's **final runtime stage** at this image instead of Alpine,
and tell the manager where Python is (it's already on `PATH`, so `python3`):

```dockerfile
# was: FROM alpine:3.21
FROM iotauz/pykernel-base:3.11-analysis AS production
USER root
# re-add the OS tools the app needs (apt names, not apk):
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl bash postgresql-client zstd tzdata \
    && rm -rf /var/lib/apt/lists/*
COPY --from=builder /run_server /run_server
# … the rest of the existing runtime stage …
# datamig reads DATAMIG_PYTHON_PATH (default "python3"); generic consumers set
# manager.Config.PythonPath. Both resolve "python3" on PATH here.
ENV DATAMIG_PYTHON_PATH=python3
```

## The size tradeoff (a deploy decision)

This image is **~590 MB** vs an Alpine runtime's ~50 MB (glibc base + pandas/
numpy/duckdb/matplotlib). Baking it into the *main* application image makes the
kernel always available but enlarges **every** deploy of that image, even ones
that never run datamig or Ali.

Two ways to adopt — pick per your deploy topology:

- **Single image** — swap the main runtime base to this (snippet above).
  Simplest; one image; pays the size everywhere.
- **Kernel-enabled variant** — keep the lean Alpine image as default and build a
  second tag `FROM pykernel-base` for the service(s) that actually run the
  kernel. Keeps unrelated deploys small at the cost of a second image in CI.

The kernel **must** share the container with the application process (the manager
`os/exec`s `python3` in the same PID namespace) — a separate sidecar container
does not work.

## Security

The image is the least-privilege *floor*: non-root `kernel` user, no compilers,
no DB clients, minimal apt set. The real isolation is enforced by the Go manager
per spawn (rlimits, process group, env scrub so no secret/DSN reaches the kernel,
jailed workdir). Strongest prod hardening (network namespace + seccomp
deny-egress) is an orchestration-layer follow-up, not part of this image.
