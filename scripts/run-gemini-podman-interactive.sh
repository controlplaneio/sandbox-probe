#!/usr/bin/env bash

set -euo pipefail

# shellcheck disable=SC1091 # dynamic script-relative helper path.
source "$(dirname "$0")/runner-common.sh"
validate_runner_inputs "$@" || exit $?
PROBE=$1
OUTDIR=$2

cp "$PROBE" "$OUTDIR/sandbox-probe"
cd "$OUTDIR" || exit

VERSION=$(gemini --version)

printf "!!!! please ensure your ./bin/sandbox-probe is either statically linked\nor only links to standard locations /lib /lib64\nso it can run inside the podman sandbox"

GEMINI_SANDBOX=podman gemini --sandbox --prompt-interactive "Execute \`./sandbox-probe scan --tags version=$VERSION,tool=gemini\` take no further action and exit /quit"
