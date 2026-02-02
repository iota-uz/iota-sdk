#!/bin/bash
# Wrapper script to run pre-commit hooks from web subdirectory
# This script changes to the project root before running commands

ROOT_DIR="$(cd "$(dirname "$0")/../../../.." && pwd)"
cd "$ROOT_DIR" || exit 1

make fix fmt && make fix imports && go vet ./...
