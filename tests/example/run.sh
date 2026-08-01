#!/usr/bin/env bash
# tests/example/run.sh
# Build the sandbox-probe Docker example and run the alice/bob boundary demo.
#
# This demonstrates sandbox-probe's reporting capability using two Docker users:
#   alice — AI agent user (runs sandbox-probe)
#   bob   — host operator whose credentials are planted but should be blocked
#
# NOTE: Docker is not a substitute for Landlock/MAC enforcement. This fixture
# uses Unix DAC permissions, so the must_block checks should pass; real
# sandboxing must still be verified with a nono/Landlock policy.
#
# Usage: bash tests/example/run.sh
#        make docker-test

set -uo pipefail

# ── Colours ───────────────────────────────────────────────────────────────────
RED='\033[0;31m'; YELLOW='\033[0;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

# ── Preflight: binary ─────────────────────────────────────────────────────────
if [[ ! -f "bin/sandbox-probe" ]]; then
  echo -e "${RED}${BOLD}ERROR: bin/sandbox-probe not found.${RESET}"
  echo "  Build it first: make build"
  exit 1
fi

# ── Preflight: Docker ─────────────────────────────────────────────────────────
if ! docker info >/dev/null 2>&1; then
  echo -e "${RED}${BOLD}ERROR: Docker is not running or not installed.${RESET}"
  echo "  Start Docker and retry."
  exit 1
fi

echo ""
echo -e "${BOLD}${CYAN}── sandbox-probe Docker Example ──────────────────────────────${RESET}"
echo -e "${BOLD}${CYAN}   Scenario: alice (AI agent) vs bob (host operator)${RESET}"
echo ""
echo "  Building Docker image..."
docker build -q -t sandbox-probe-example -f tests/example/Dockerfile . \
  || { echo -e "${RED}Docker build failed${RESET}"; exit 1; }
echo "  Image built: sandbox-probe-example"
echo ""
echo "  Running sandbox-probe as alice inside the container..."
echo ""

docker run --rm sandbox-probe-example
PROBE_EXIT=$?

echo ""
echo -e "${YELLOW}${BOLD}NOTE:${RESET} This Docker fixture demonstrates Unix DAC permissions only."
echo "  For real sandbox enforcement, run sandbox-probe inside a nono/Landlock sandbox."
echo ""

if [[ "${PROBE_EXIT}" -eq 0 ]]; then
	echo -e "${GREEN}${BOLD}✅ PASS — sandbox-probe verified the fixture boundaries${RESET}"
	else
	echo -e "${RED}${BOLD}❌ FAIL — sandbox-probe reported boundary violations${RESET}"
	echo "  Exit code: ${PROBE_EXIT}"
	exit "${PROBE_EXIT}"
fi

echo ""
