#!/usr/bin/env bash
# Run sandbox-probe via gemini-cli with the matrix's chosen sandbox backend.
# Required env: PROBE, LABEL, RUNNER, BACKEND, OUT, MODEL
# Optional: GEMINI_SANDBOX (empty = no --sandbox), GEMINI_API_KEY.
#
# Workaround for google-gemini/gemini-cli#26964 (broken in v0.42.0): the
# sandbox image has a default ENTRYPOINT ["gemini"], so we force
# --entrypoint "" via SANDBOX_FLAGS. Drop once a fixed release lands.
set -eo pipefail

: "${PROBE:?}" "${LABEL:?}" "${RUNNER:?}" "${BACKEND:?}" "${OUT:?}" "${MODEL:?}"

mkdir -p "$(dirname "$OUT")"

VERSION=$(gemini --version)
TAGS="runner=${RUNNER},backend=${BACKEND},gemini=${VERSION},mode=via-gemini"
PROMPT="Run this exact shell command and then exit: ${PROBE} scan --tasksets baseline --tags ${TAGS} --output_path ${OUT}"

SANDBOX_FLAG=()
[ -n "${GEMINI_SANDBOX:-}" ] && SANDBOX_FLAG=(--sandbox)

for attempt in 1 2 3; do
  echo "::group::gemini attempt ${attempt}/3"
  if gemini --yolo --skip-trust --model "$MODEL" --prompt "$PROMPT" "${SANDBOX_FLAG[@]}" 2>&1 | tee gemini.log; then
    echo "::endgroup::"
    break
  fi
  echo "::endgroup::"
  if grep -qi 'quota\|rate.*limit\|429' gemini.log; then
    echo "::error::gemini quota/rate-limit — not retrying"
    exit 1
  fi
  [ "$attempt" -lt 3 ] && sleep $((attempt * 5))
done

[ -f "$OUT" ] || { echo "::error::gemini did not produce $OUT"; exit 1; }
