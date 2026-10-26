#!/usr/bin/env bash
# external_host_connectivity must report 169.254.169.254 when the cloud
# instance metadata service is reachable, since that is an IAM credential-theft
# path. A no-network container claims the address on loopback and listens on
# :80, so the probe sees a reachable "IMDS" with no cloud and no DNS. The same
# container without the listener is the negative control.

# Hardened like scripts/ and detect_openshell.sh: a failing docker run or jq here
# must abort the test, not reach the assertion with an empty value and be read as a
# verdict about the probe. pipefail is why this is bash and not /bin/sh.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
IMAGE="${IMDS_TEST_IMAGE:-python:3.12-alpine}"

if ! command -v docker >/dev/null 2>&1 || ! docker info >/dev/null 2>&1; then
    echo "docker unavailable: skipping IMDS test"
    exit 0
fi

# The image is musl-based, so build a static binary rather than reuse the
# cgo-linked bin/sandbox-probe.
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT
(cd "$PROJECT_ROOT" && CGO_ENABLED=0 go build -o "$TMPDIR/sandbox-probe" .) || exit 1

# $1 = "listen" to start the fake IMDS. Prints external_host_connectivity.
connectivity() {
    docker run --rm --network none --cap-add NET_ADMIN \
        -v "$TMPDIR/sandbox-probe:/sandbox-probe:ro" -e MODE="$1" "$IMAGE" sh -c '
        ip addr add 169.254.169.254/32 dev lo || exit 1
        if [ "$MODE" = listen ]; then
            python -m http.server 80 --bind 169.254.169.254 >/dev/null 2>&1 &
            for i in $(seq 100); do
                python -c "import socket; socket.create_connection((\"169.254.169.254\", 80), 1)" 2>/dev/null && break
                sleep 0.2
            done
        fi
        cd /tmp && /sandbox-probe scan --fast --output_path /tmp/r.json >/dev/null 2>&1; cat /tmp/r.json' |
        jq -c '[.findings[] | select(.findingType == "external_host_connectivity") | .value][0]'
}

echo "Testing IMDS: metadata address listening"
got=$(connectivity listen)
if [ "$got" != '["169.254.169.254"]' ]; then
    echo "external_host_connectivity = $got with IMDS reachable: ✗ Test failed"
    exit 1
fi
echo "IMDS reported when reachable: ✓"

echo "Testing IMDS: metadata address not listening"
got=$(connectivity none)
if [ "$got" != "[]" ]; then
    echo "external_host_connectivity = $got with IMDS closed: ✗ Test failed"
    exit 1
fi
echo "IMDS not reported when closed: ✓"
echo "IMDS test passed"
