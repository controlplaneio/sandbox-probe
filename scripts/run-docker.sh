#!/usr/bin/env bash

set -euo pipefail

# shellcheck disable=SC1091 # dynamic script-relative helper path.
source "$(dirname "$0")/runner-common.sh"
validate_runner_inputs "$@" || exit $?
PROBE=$1
OUTDIR=$2

cp "$PROBE" "$OUTDIR"

cd "$OUTDIR" || exit

docker run -w /data/ --rm -v "$(pwd):/data" ubuntu:latest \
  "/data/$(basename "$PROBE")" scan --tasks baseline_sandbox_task --tasksets none
