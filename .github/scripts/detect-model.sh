#!/usr/bin/env bash
set -euo pipefail

# Detect Claude model from trigger content
# Input:
#   TRIGGER_BODY - The comment/issue body that triggered the workflow
#   TRIGGER_CONTEXT - Additional context (issue title+body, or PR summary+diff)
# Output: JSON to stdout with schema: {model, selection_method, reasoning}
#
# Priority 1: Direct triggers (@opus, @sonnet, @haiku)
# Priority 2: Dynamic analysis for plain @claude trigger
# Fallback: Default to sonnet

output_json() {
  jq -n --arg model "$1" --arg method "$2" --arg reason "$3" \
    '{model: $model, selection_method: $method, reasoning: $reason}'
}

BODY="${TRIGGER_BODY:-}"
CONTEXT="${TRIGGER_CONTEXT:-}"

# Priority 1: Direct model triggers (@opus, @sonnet, @haiku)
if [[ "$BODY" =~ @opus([^a-zA-Z0-9_-]|$) ]]; then
  output_json "opus" "direct_trigger" "Triggered with @opus"
  exit 0
fi

if [[ "$BODY" =~ @sonnet([^a-zA-Z0-9_-]|$) ]]; then
  output_json "sonnet" "direct_trigger" "Triggered with @sonnet"
  exit 0
fi

if [[ "$BODY" =~ @haiku([^a-zA-Z0-9_-]|$) ]]; then
  output_json "haiku" "direct_trigger" "Triggered with @haiku"
  exit 0
fi

# Priority 2: Plain @claude trigger - default to sonnet (skip dynamic analysis for speed)
if [[ "$BODY" =~ @claude([^a-zA-Z0-9_:-]|$) ]]; then
  output_json "sonnet" "default" "Plain @claude trigger, defaulting to sonnet"
  exit 0
fi

# Fallback: no trigger pattern matched
output_json "sonnet" "default_fallback" "No trigger pattern matched, defaulting to sonnet"
