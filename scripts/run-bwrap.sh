#!/usr/bin/env bash

set -euo pipefail

# shellcheck disable=SC1091 # dynamic script-relative helper path.
source "$(dirname "$0")/runner-common.sh"
validate_runner_inputs "$@" || exit $?
PROBE=$1
OUTDIR=$2

cp "$PROBE" "$OUTDIR"

cd "$OUTDIR" || exit

BINARY_NAME=$(basename "$PROBE")
WORKDIR=$(pwd)

bwrap \
  --ro-bind /proc /proc \
  --ro-bind /usr /usr \
  --ro-bind /lib /lib \
  --ro-bind /lib64 /lib64 \
  --ro-bind /bin /bin \
  --ro-bind /sbin /sbin \
  --ro-bind /etc /etc \
  --bind "$WORKDIR" /data \
  --bind /tmp /tmp \
  --chdir /data \
  --unshare-user \
  --unshare-ipc \
  --unshare-uts \
  --unshare-cgroup \
  --share-net \
  --die-with-parent \
  "/data/${BINARY_NAME}" scan --tasks baseline_sandbox_task --tasksets none
