#!/usr/bin/env bash
# Run sandbox-probe DIRECTLY inside a sandbox runtime (no agent, no model). Distinct keyless OS
# sandbox mechanisms; the probe fingerprints each. RUNTIME selects the wrapper:
#   srt      — @anthropic-ai/sandbox-runtime: bubblewrap (Linux) / Seatbelt (macOS) + network proxy
#   firejail — SUID namespaces + seccomp (Linux)
#   nono     — capability sandbox: Landlock + seccomp-notify (Linux) / Seatbelt (macOS)
#   podman   — rootless OCI container (Linux)
#   docker   — Docker container (Linux)
#   bwrap    — standalone bubblewrap, parent visible (the #38-detectable invocation; Linux)
#   apparmor — aa-exec under a named AppArmor profile (Linux)
#   nspawn   — systemd-nspawn container (Linux; needs ROOTFS)
#   gvisor   — runsc run, systrap platform, no KVM (Linux)
#
# Required env: PROBE, OUT, RUNTIME. Optional: RUNNER, PORT (unused), SCAN_ARGS, ROOTFS (nspawn).
set -eo pipefail

: "${PROBE:?PROBE (probe binary path) is required}"
: "${OUT:?OUT (report output path) is required}"
: "${RUNTIME:?RUNTIME (srt|firejail|nono|podman|docker|bwrap|apparmor|nspawn|gvisor) is required}"
RUNNER="${RUNNER:-$(uname -s)}"
SCAN_ARGS="${SCAN_ARGS:-scan --tasksets baseline}"

mkdir -p "$(dirname "$OUT")"
PROBE_ABS="$(cd "$(dirname "$PROBE")" && pwd)/$(basename "$PROBE")"
OUT_ABS="$(cd "$(dirname "$OUT")" && pwd)/$(basename "$OUT")"
TAGS="runner=${RUNNER},harness=${RUNTIME},sandbox=on,mode=via-sandbox"
CMD=("$PROBE_ABS" $SCAN_ARGS --tags "$TAGS" --output_path "$OUT_ABS")

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
    # the report lands back on the host. PROBE/OUT are under $PWD in CI.
    podman run --rm --network=none -v "$PWD:/work" -w /work docker.io/library/ubuntu:latest \
      "/work/${PROBE_ABS#"$PWD"/}" $SCAN_ARGS --tags "$TAGS" --output_path "/work/${OUT_ABS#"$PWD"/}" </dev/null || true
    ;;
  docker)
    # Same as podman but the Docker daemon; the probe fingerprints it as "docker".
    docker run --rm --network=none -v "$PWD:/work" -w /work ubuntu:latest \
      "/work/${PROBE_ABS#"$PWD"/}" $SCAN_ARGS --tags "$TAGS" --output_path "/work/${OUT_ABS#"$PWD"/}" </dev/null || true
    ;;
  bwrap)
    # Standalone bubblewrap with bwrap left visible as the parent — the invocation the probe DOES
    # fingerprint as "bubblewrap" (unlike Claude Code / srt; see controlplaneio/sandbox-probe#38).
    bwrap --ro-bind /usr /usr --ro-bind /bin /bin --ro-bind-try /sbin /sbin \
      --ro-bind /lib /lib --ro-bind-try /lib64 /lib64 --ro-bind /etc /etc --proc /proc --dev /dev \
      --bind "$PWD" /work --chdir /work --unshare-user --unshare-ipc --unshare-uts --unshare-cgroup --die-with-parent \
      "/work/${PROBE_ABS#"$PWD"/}" $SCAN_ARGS --tags "$TAGS" --output_path "/work/${OUT_ABS#"$PWD"/}" || true
    ;;
  apparmor)
    # aa-exec transitions into the pre-loaded (permissive-but-named) profile; the probe then reads
    # /proc/self/attr/current = "sandbox-probe (enforce)" -> "apparmor".
    sudo aa-exec -p sandbox-probe -- "${CMD[@]}" || true
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
