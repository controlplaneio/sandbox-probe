#!/usr/bin/env bash
# Run sandbox-probe DIRECTLY inside a sandbox runtime (no agent, no model). Distinct keyless OS
# sandbox mechanisms; the probe fingerprints each. RUNTIME selects the wrapper:
#   srt      — @anthropic-ai/sandbox-runtime: bubblewrap (Linux) / Seatbelt (macOS) + network proxy
#   firejail — SUID namespaces + seccomp (Linux)
#   nono     — capability sandbox: Landlock + seccomp-notify (Linux) / Seatbelt (macOS)
#   podman   — rootless OCI container (Linux)
#   docker   — Docker container (Linux)
#   bwrap    — standalone bubblewrap, parent visible (the #38-detectable invocation; Linux)
#   nspawn   — systemd-nspawn container (Linux; needs ROOTFS)
#   gvisor   — runsc run, systrap platform, no KVM (Linux; needs ROOTFS)
#
# Required env: PROBE, OUT, RUNTIME. Optional: RUNNER, PORT (unused), SCAN_ARGS, ROOTFS.
set -eo pipefail

: "${PROBE:?PROBE (probe binary path) is required}"
: "${OUT:?OUT (report output path) is required}"
: "${RUNTIME:?RUNTIME (srt|firejail|nono|podman|docker|bwrap|nspawn|gvisor) is required}"
RUNNER="${RUNNER:-$(uname -s)}"
SCAN_ARGS="${SCAN_ARGS:-scan --tasksets baseline}"

mkdir -p "$(dirname "$OUT")"
PROBE_ABS="$(cd "$(dirname "$PROBE")" && pwd)/$(basename "$PROBE")"
OUT_ABS="$(cd "$(dirname "$OUT")" && pwd)/$(basename "$OUT")"

# Capture the sandbox tool's own version into the report tags, so results are comparable across tool
# upgrades over time (the kernel/OS is recorded separately by the probe's environment_detection
# finding, on every run). awk consumes the whole stream (no `head` — that would SIGPIPE under
# pipefail); each extractor picks the version token from the tool's first --version line.
sandbox_version() {
  case "$1" in
    srt)      srt --version            2>/dev/null | awk 'NR==1{print $1}' ;;
    firejail) firejail --version       2>/dev/null | awk 'NR==1{print $NF}' ;;
    nono)     nono --version           2>/dev/null | awk 'NR==1{print $NF}' ;;
    bwrap)    bwrap --version          2>/dev/null | awk 'NR==1{print $NF}' ;;
    podman)   podman --version         2>/dev/null | awk 'NR==1{print $3}' ;;
    docker)   docker --version         2>/dev/null | awk 'NR==1{gsub(/,/,"",$3); print $3}' ;;
    gvisor)   runsc --version          2>/dev/null | awk 'NR==1{print $NF}' ;;
    nspawn)   systemd-nspawn --version 2>/dev/null | awk 'NR==1{print $2}' ;;
  esac
}
RUNTIME_VERSION="$(sandbox_version "$RUNTIME")" || RUNTIME_VERSION=""
TAGS="runner=${RUNNER},harness=${RUNTIME},${RUNTIME}=${RUNTIME_VERSION:-unknown},sandbox=on,mode=via-sandbox"
CMD=("$PROBE_ABS" $SCAN_ARGS --tags "$TAGS" --output_path "$OUT_ABS")

# The container runtimes mount $PWD at /work and strip the $PWD prefix from the probe/report paths;
# that only holds when both live under $PWD. Fail clearly otherwise (instead of a cryptic broken
# /work//abs path yielding a "did not produce" error with no cause).
require_under_pwd() { case "$1" in "$PWD"/*) ;; *) echo "::error::$2 ($1) must be under \$PWD ($PWD) for the ${RUNTIME} container mount"; exit 1 ;; esac; }

echo "::group::sandbox ${RUNTIME}"
case "$RUNTIME" in
  srt)
    # Deny-by-default; allow writes only to the workspace + tmp so the probe can emit its report.
    SETTINGS="$(mktemp)"
    cat > "$SETTINGS" <<JSON
{ "filesystem": { "denyRead": [], "allowRead": [], "allowWrite": ["${PWD}", "/tmp", "/private/tmp"], "denyWrite": [] },
  "network": { "allowedDomains": [], "deniedDomains": [] } }
JSON
    srt --settings "$SETTINGS" "${CMD[@]}" || true
    rm -f "$SETTINGS"
    ;;
  firejail)
    firejail --quiet --net=none --seccomp "${CMD[@]}" || true
    ;;
  nono)
    # Read-only cwd, write only to the report dir, no network. stdin from /dev/null so a denial
    # never blocks on an interactive prompt.
    nono run --silent --allow-cwd --allow "$(dirname "$OUT_ABS")" --block-net "${CMD[@]}" </dev/null || true
    ;;
  podman)
    # Run the probe inside a rootless container so it fingerprints podman; mount the workspace so
    # the report lands back on the host. PROBE/OUT must be under $PWD (mounted at /work).
    require_under_pwd "$PROBE_ABS" PROBE; require_under_pwd "$OUT_ABS" OUT
    podman run --rm --network=none -v "$PWD:/work" -w /work docker.io/library/ubuntu:latest \
      "/work/${PROBE_ABS#"$PWD"/}" $SCAN_ARGS --tags "$TAGS" --output_path "/work/${OUT_ABS#"$PWD"/}" </dev/null || true
    ;;
  docker)
    # Same as podman but the Docker daemon; the probe fingerprints it as "docker".
    require_under_pwd "$PROBE_ABS" PROBE; require_under_pwd "$OUT_ABS" OUT
    docker run --rm --network=none -v "$PWD:/work" -w /work ubuntu:latest \
      "/work/${PROBE_ABS#"$PWD"/}" $SCAN_ARGS --tags "$TAGS" --output_path "/work/${OUT_ABS#"$PWD"/}" </dev/null || true
    ;;
  bwrap)
    # Standalone bubblewrap with bwrap left visible as the parent — the invocation the probe DOES
    # fingerprint as "bubblewrap" (unlike Claude Code / srt; see controlplaneio/sandbox-probe#38).
    # Same $PWD->/work remap as the container cases, so the same guard applies.
    require_under_pwd "$PROBE_ABS" PROBE; require_under_pwd "$OUT_ABS" OUT
    bwrap --ro-bind /usr /usr --ro-bind /bin /bin --ro-bind-try /sbin /sbin \
      --ro-bind /lib /lib --ro-bind-try /lib64 /lib64 --ro-bind /etc /etc --proc /proc --dev /dev \
      --bind "$PWD" /work --chdir /work --unshare-user --unshare-ipc --unshare-uts --unshare-cgroup --die-with-parent \
      "/work/${PROBE_ABS#"$PWD"/}" $SCAN_ARGS --tags "$TAGS" --output_path "/work/${OUT_ABS#"$PWD"/}" || true
    ;;
  nspawn)
    # ROOTFS is a prepared root filesystem (built by the workflow); copy the probe in and bind the
    # report dir back to the host. Inside, container=systemd-nspawn -> "nspawn".
    : "${ROOTFS:?ROOTFS (prepared rootfs dir) is required for nspawn}"
    sudo cp "$PROBE_ABS" "$ROOTFS/probe"
    sudo systemd-nspawn -q -D "$ROOTFS" --bind="$(dirname "$OUT_ABS")" \
      /probe $SCAN_ARGS --tags "$TAGS" --output_path "$OUT_ABS" </dev/null || true
    ;;
  gvisor)
    # Full runsc container (systrap platform — no KVM needed) so /__runsc_containers__ exists ->
    # "gvisor". ROOTFS (built by the workflow from an ubuntu image, /.dockerenv removed) provides libc
    # for the dynamically-linked probe; bind the report dir back to the host.
    : "${ROOTFS:?ROOTFS (prepared rootfs dir) is required for gvisor}"
    sudo cp "$PROBE_ABS" "$ROOTFS/probe"
    OUTDIR="$(dirname "$OUT_ABS")"
    BUNDLE="$(mktemp -d)"
    ( cd "$BUNDLE" && runsc spec )
    ARGS_JSON="$(printf '%s\n' /probe $SCAN_ARGS --tags "$TAGS" --output_path "$OUT_ABS" | jq -R . | jq -s .)"
    TMPCFG="$(mktemp)"
    jq --arg root "$ROOTFS" --arg out "$OUTDIR" --argjson args "$ARGS_JSON" \
      '.root.path=$root | .process.args=$args | .process.terminal=false
       | .mounts += [{"destination":$out,"source":$out,"type":"bind","options":["bind","rw"]}]' \
      "$BUNDLE/config.json" > "$TMPCFG" && mv "$TMPCFG" "$BUNDLE/config.json"
    # gVisor maps the container root to an unprivileged host uid; make the report dir writable to it.
    chmod 0777 "$OUTDIR"
    sudo runsc --network=none run -bundle "$BUNDLE" gvisor-probe </dev/null || true
    sudo rm -rf "$BUNDLE"
    ;;
  *)
    echo "::error::unknown RUNTIME '${RUNTIME}'"; exit 1 ;;
esac
echo "::endgroup::"

if [ ! -f "$OUT" ]; then
  echo "::error::sandbox ${RUNTIME} did not produce ${OUT}"
  exit 1
fi
echo "sandbox ${RUNTIME} wrote ${OUT}"
