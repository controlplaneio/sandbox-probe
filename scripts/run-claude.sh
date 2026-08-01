#!/usr/bin/env bash

set -euo pipefail

# shellcheck disable=SC1091 # dynamic script-relative helper path.
source "$(dirname "$0")/runner-common.sh"
validate_runner_inputs "$@" || exit $?
PROBE=$1
OUTDIR=$2

cp "$PROBE" "$OUTDIR"
cd "$OUTDIR" || exit

VERSION=$(claude --version)

claude --allowedTools "Bash" -p "Execute !./$(basename "$PROBE") scan --tags version=${VERSION},tool=claude"
