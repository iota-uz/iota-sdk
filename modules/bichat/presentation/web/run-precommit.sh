#!/bin/bash
# Wrapper script to run pre-commit hooks from web subdirectory
# This script changes to the project root before running commands

ROOT_DIR="$(cd "$(dirname "$0")/../../../.." && pwd)"
cd "$ROOT_DIR" || exit 1

just fix fmt && just fix imports && go vet ./...
