#!/usr/bin/env python3
"""Self-verification for the pykernel-base image.

Asserts two things the image must provide:
  1. The standard-library modules the pykernel shim (bootstrap.py) imports — if
     any are missing the kernel cannot even start.
  2. The analysis libraries an agent program may import — with a trivial op each
     so a broken/musl wheel surfaces here, not at first run.

Exits non-zero on any failure so a Docker build, CI step, or container
healthcheck can gate on it:

    docker run --rm <image> python3 /opt/pykernel/smoke_test.py
"""
import importlib
import sys

# Stdlib the shim relies on (see pkg/pykernel/runtime/bootstrap.py).
SHIM_STDLIB = ["ast", "json", "os", "socket", "struct", "sys",
               "threading", "traceback", "queue", "ctypes"]


def check_stdlib() -> None:
    for name in SHIM_STDLIB:
        importlib.import_module(name)
    print(f"ok: shim stdlib imports ({len(SHIM_STDLIB)})")


def check_analysis_libs() -> None:
    import numpy as np
    import pandas as pd
    import duckdb
    import openpyxl  # noqa: F401  (import is the test)
    import matplotlib  # noqa: F401

    # Tiny ops so a wheel that imports but is ABI-broken is caught.
    assert int(np.arange(5).sum()) == 10
    df = pd.DataFrame({"a": [1, 2, 3]})
    assert int(df["a"].sum()) == 6
    assert duckdb.sql("select 21 * 2 as n").fetchone()[0] == 42

    print(f"ok: numpy {np.__version__}, pandas {pd.__version__}, "
          f"duckdb {duckdb.__version__}, openpyxl {openpyxl.__version__}, "
          f"matplotlib {matplotlib.__version__}")


def main() -> int:
    print(f"python {sys.version.split()[0]} ({sys.platform})")
    try:
        check_stdlib()
        check_analysis_libs()
    except Exception as exc:  # noqa: BLE001 — the test's whole job is to report
        print(f"FAIL: {type(exc).__name__}: {exc}", file=sys.stderr)
        return 1
    print("pykernel-base: OK")
    return 0


if __name__ == "__main__":
    sys.exit(main())
