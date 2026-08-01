#!/usr/bin/env bash

set -euo pipefail


# shellcheck disable=SC1091 # dynamic script-relative helper path.
source "$(dirname "$0")/runner-common.sh"
validate_runner_inputs "$@" || exit $?
PROBE=$1
OUTDIR=$2

cp "$PROBE" "$OUTDIR"

cd "$OUTDIR" || exit

# TODO: narrow down to just be able to run the executable
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

CLAUDE_HOME="$(mktemp -d)"
trap 'rm -rf "$CLAUDE_HOME"' EXIT
mkdir -p "$CLAUDE_HOME/.claude"
printf '{}\n' > "$CLAUDE_HOME/.claude/settings.json"

VERSION=$(HOME="$CLAUDE_HOME" claude --version)

HOME="$CLAUDE_HOME" claude --settings "${SCRIPT_DIR}/config/claude-settings.json" --allowedTools "Bash" -p "Execute !./$(basename "$PROBE") scan --tags version=${VERSION},tool=claude,sandbox=true"
