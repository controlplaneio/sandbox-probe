#!/usr/bin/env bash
# GATED end-to-end check: run the probe inside a real gVisor sandbox that has a known bind mount of
# a host directory, and fail unless the mount finding names that mount.
#
# It needs root and a gVisor installation, so it is NOT in the per-push suite and nothing runs it
# implicitly — `make e2etest-gvisor-bind`, or the gvisor-bind-mount workflow (workflow_dispatch).
#
# Why it exists: the table test in pkg/tasks/baseline drives the enumerator from pinned mount-table
# captures. Those captures can drift from what the kernel actually reports; this run cannot, because
# it reads the real thing. gVisor is the runtime that exposed the bug (#7): it spells every mount's
# source "none", so the enumerator's old source-shape filter dropped a reachable host bind and the
# Host mounts category read "blocked" while the host filesystem was exposed.
#
# On a machine that cannot run runsc directly (macOS), run it inside a privileged container, where
# runsc needs one extra flag:
#   docker run --privileged --rm -v "$PWD:/w" -w /w ubuntu:22.04 bash -c '
#     apt-get update && apt-get install -y wget jq sudo golang-go
#     wget -qO /usr/local/bin/runsc "https://storage.googleapis.com/gvisor/releases/release/latest/$(uname -m)/runsc"
#     chmod +x /usr/local/bin/runsc
#     RUNSC_FLAGS=--ignore-cgroups tests/gated/gvisor_bind_mount.sh'
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

for tool in runsc jq go; do
  command -v "$tool" >/dev/null || { echo "PREREQUISITE: $tool is not installed; this check needs root and a gVisor installation"; exit 1; }
done

BASE="$(mktemp -d)"
trap 'rm -rf "$BASE"' EXIT

# The bind source: a throwaway directory of our own, never a real catalogue path. gVisor maps the
# sandbox root to an unprivileged host uid, so it has to be world-readable to read through.
HOSTBIND="$BASE/hostbind"
mkdir -p "$HOSTBIND"
echo "sandbox-probe gated gvisor bind marker" > "$HOSTBIND/marker.txt"
chmod -R a+rX "$BASE"

# CGO_ENABLED=0 makes the probe static, so the sandbox's root filesystem can be an empty directory —
# no rootfs to build, no libc to ship, one less thing to go wrong before the assertion.
CGO_ENABLED=0 go build -o "$BASE/probe" "$PROJECT_ROOT"
mkdir -p "$BASE/rootfs" "$BASE/out"

# Only the mount scanner: the baseline taskset resolves real hostnames and scans ports, none of
# which this check reads.
# Self-contained on purpose: the multi-runtime launcher this used to call is comparison
# methodology and now lives in sandbox-probe-reports. This check asserts a probe capability —
# that the mount enumerator names a reachable host mount — so it must not depend on it.
OUT_JSON="$BASE/out/report.json"
BUNDLE="$(mktemp -d)"
( cd "$BUNDLE" && runsc spec )
ARGS_JSON="$(printf '%s\n' /probe scan --tasksets none --tasks baseline_mount_task \
  --output_path "$OUT_JSON" | jq -R . | jq -s .)"
BIND_JSON="$(jq -n --arg src "$HOSTBIND" \
  '[{"destination":"/hostbind","source":$src,"type":"bind","options":["bind","ro"]}]')"
cp "$BASE/probe" "$BASE/rootfs/probe"
TMPCFG="$(mktemp)"
jq --arg root "$BASE/rootfs" --arg out "$BASE/out" --argjson args "$ARGS_JSON" --argjson bind "$BIND_JSON" \
  '.root.path=$root | .process.args=$args | .process.terminal=false
   | .mounts += [{"destination":$out,"source":$out,"type":"bind","options":["bind","rw"]}] + $bind' \
  "$BUNDLE/config.json" > "$TMPCFG" && mv "$TMPCFG" "$BUNDLE/config.json"
# gVisor maps the container root to an unprivileged host uid; make the report dir writable to it.
chmod 0777 "$BASE/out"
set +e
runsc $RUNSC_FLAGS --platform=systrap --network=none run -bundle "$BUNDLE" gvisor-bind-probe </dev/null
launch=$?
set -e
rm -rf "$BUNDLE"

# Two distinct failures, and the output must say which. A sandbox that never started says nothing
# about the enumerator; only a report that omits the bind is the regression.
if [ ! -f "$BASE/out/report.json" ]; then
  echo "FAIL (sandbox): gVisor did not run the probe to completion — no report was written (runsc exited $launch)."
  echo "  This is an environment failure, NOT the mount-enumeration regression. Check runsc, root and the bundle above."
  exit 1
fi

mounts="$(jq -r '.findings[] | select(.findingType == "mounted_volumes_detections") | .value[]' "$BASE/out/report.json")"

if ! printf '%s\n' "$mounts" | grep -q -- ' -> /hostbind ('; then
  echo "FAIL (regression): the sandbox ran and the probe reported, but the bind mount at /hostbind is ABSENT from the mount finding."
  echo "  A reachable host mount that is not enumerated makes the Host mounts category read blocked while the host filesystem is exposed."
  echo "  Mounts reported:"
  printf '%s\n' "$mounts" | sed 's/^/    /'
  exit 1
fi

echo "PASS: the /hostbind bind mount is named in the mount finding:"
printf '%s\n' "$mounts" | grep -- ' -> /hostbind ('
