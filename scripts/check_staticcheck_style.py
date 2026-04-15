#!/usr/bin/env python3

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
from pathlib import Path


CHECKS = "ST1000,ST1016,ST1020,ST1021,ST1022"
GENERATED_FILE_RE = re.compile(r"(_templ|_gen)\.go$|(^|/)generated\.go$")
GENERATED_MARKERS = ("Code generated", "DO NOT EDIT")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Run selected staticcheck style rules on handwritten Go code.",
    )
    parser.add_argument(
        "--tags",
        default="",
        help="Comma-separated Go build tags passed to staticcheck.",
    )
    return parser.parse_args()


def list_packages() -> list[str]:
    proc = subprocess.run(
        ["go", "list", "./..."],
        check=True,
        capture_output=True,
        text=True,
    )
    return [line for line in proc.stdout.splitlines() if line.strip()]


def is_generated_file(path: str) -> bool:
    normalized = path.replace(os.sep, "/")
    if GENERATED_FILE_RE.search(normalized):
        return True

    file_path = Path(path)
    try:
        with file_path.open("r", encoding="utf-8") as handle:
            for _ in range(5):
                line = handle.readline()
                if not line:
                    break
                if any(marker in line for marker in GENERATED_MARKERS):
                    return True
    except OSError:
        return False

    return False


def run_staticcheck(packages: list[str], tags: str) -> list[dict[str, object]]:
    cmd = ["staticcheck", "-checks", CHECKS, "-f", "json"]
    if tags:
        cmd.extend(["-tags", tags])
    cmd.extend(packages)

    proc = subprocess.run(
        cmd,
        check=False,
        capture_output=True,
        text=True,
    )

    diagnostics: list[dict[str, object]] = []
    for line in proc.stdout.splitlines():
        line = line.strip()
        if not line:
            continue
        diagnostics.append(json.loads(line))

    if proc.returncode not in (0, 1):
        sys.stderr.write(proc.stderr)
        raise SystemExit(proc.returncode)

    return diagnostics


def main() -> int:
    args = parse_args()
    packages = list_packages()

    chunk_size = 25
    remaining: list[dict[str, object]] = []

    for start in range(0, len(packages), chunk_size):
        diagnostics = run_staticcheck(packages[start : start + chunk_size], args.tags)
        for diagnostic in diagnostics:
            location = diagnostic.get("location", {})
            file_path = location.get("file", "")
            if isinstance(file_path, str) and is_generated_file(file_path):
                continue
            remaining.append(diagnostic)

    if not remaining:
        return 0

    for diagnostic in remaining:
        location = diagnostic["location"]
        position = location["position"]
        check_name = diagnostic.get("check", "staticcheck")
        sys.stderr.write(
            f"{location['file']}:{position['line']}:{position['column']}: "
            f"{diagnostic['message']} ({check_name})\n"
        )

    return 1


if __name__ == "__main__":
    raise SystemExit(main())
