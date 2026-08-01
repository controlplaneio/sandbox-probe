#!/usr/bin/env bash

set -euo pipefail

# shellcheck disable=SC1091 # dynamic script-relative helper path.
source "$(dirname "$0")/runner-common.sh"
validate_runner_inputs "$@" || exit $?
PROBE=$1
OUTDIR=$2

cp "$PROBE" "$OUTDIR"
cd "$OUTDIR" || exit

VERSION=$(gemini --version)

# gemini doesn't allow tool call of `run_shell_command` when using non interactive prompts
# there is `--prompt-interactive` which can take a value but even with `--yolo`
# gemini will wait for user input and won't quit
# instead we pipe output into interaction
printf "Execute \`./sandbox-probe scan --tags version=%s,tool=gemini\` take no further action and exit /quit\n\n\n\n" "$VERSION" | gemini --yolo
