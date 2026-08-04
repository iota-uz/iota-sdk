# pykernel-base — the co-located Python interpreter the pykernel manager spawns.
#
# Why a dedicated image: a Granite/SDK application binary ships on Alpine/musl,
# but pandas/numpy/duckdb publish only glibc (manylinux) wheels — they will not
# import on musl. The kernel therefore needs a glibc Python on PATH. This image
# IS that runtime: a glibc CPython 3.11 with the analysis libraries pre-installed.
# The application's final stage builds `FROM` this image and copies its static Go
# binary in (the binary is CGO-free, so it runs unchanged on glibc). See README.
#
# The pykernel shim (bootstrap.py) is delivered at spawn time by the Go manager
# (//go:embed → written to the per-run workdir); it is NOT baked here. The image's
# only job is to provide `python3` + the agent-importable libraries.
#
# Build (from this directory):
#   docker build -t iotauz/pykernel-base:3.11-analysis .
# Smoke test:
#   docker run --rm iotauz/pykernel-base:3.11-analysis python3 /opt/pykernel/smoke_test.py
FROM python:3.11-slim-bookworm

# uv: fast, deterministic installs from the pinned requirements.txt. Installed
# via pip (rather than `COPY --from=ghcr.io/astral-sh/uv`) so the image builds on
# the legacy Docker builder too — no buildx/BuildKit requirement.
RUN pip install --no-cache-dir uv==0.5.11

ENV UV_SYSTEM_PYTHON=1 \
    UV_NO_CACHE=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

# Minimal OS deps: tzdata (so a migration that localises timestamps has zoneinfo)
# and ca-certificates (TLS roots) only. No compilers, no clients — all data I/O
# is via host capabilities, not the kernel.
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt /opt/pykernel/requirements.txt
RUN uv pip install --system -r /opt/pykernel/requirements.txt

# Smoke test lives in the image so a deploy can self-verify (CI / healthcheck).
COPY smoke_test.py /opt/pykernel/smoke_test.py

# Run kernels as a non-root, no-login user. The Go manager additionally sandboxes
# each spawn (rlimits, process group, env scrub, jailed workdir); this is the
# baseline-least-privilege floor under that.
RUN useradd --create-home --uid 10001 --shell /usr/sbin/nologin kernel
USER kernel
WORKDIR /home/kernel

# The interpreter the pykernel manager should launch (datamig: DATAMIG_PYTHON_PATH;
# generic: manager.Config.PythonPath). It is on PATH, so "python3" suffices.
ENV PYKERNEL_PYTHON=python3

# Default command is a no-op identity probe; real use FROMs this image and runs
# the application binary (which spawns python3 itself).
CMD ["python3", "/opt/pykernel/smoke_test.py"]
