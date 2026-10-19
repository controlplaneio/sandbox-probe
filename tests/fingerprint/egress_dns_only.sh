#!/bin/sh
# external_host_connectivity must mean "a TCP connect succeeded", not "DNS
# answered". The probe runs in a container with no network whose /etc/hosts
# still resolves google.com, the shape of a sandbox that answers DNS but
# refuses egress, and must report no connectivity there. A second run on the
# default network is the positive control, asserted only when the container
# itself can open a TCP connection, so an offline runner doesn't fail it.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BIN="${PROJECT_ROOT}/bin/sandbox-probe"
IMAGE="${EGRESS_TEST_IMAGE:-debian:trixie}"

if ! command -v docker >/dev/null 2>&1 || ! docker info >/dev/null 2>&1; then
    echo "docker unavailable: skipping egress test"
    exit 0
fi

# Prints the external_host_connectivity value as compact JSON.
connectivity() {
    docker run --rm "$@" -v "${BIN}:/sandbox-probe:ro" "$IMAGE" \
        sh -c 'cd /tmp && /sandbox-probe scan --fast --output_path /tmp/r.json >/dev/null 2>&1; cat /tmp/r.json' |
        jq -c '[.findings[] | select(.findingType == "external_host_connectivity") | .value][0]'
}

echo "Testing egress: DNS resolves, network absent"
got=$(connectivity --network none --add-host google.com:192.0.2.1)
if [ "$got" != "[]" ]; then
    echo "external_host_connectivity = $got with no network: ✗ Test failed"
    exit 1
fi
echo "No connectivity reported without a network: ✓"

echo "Testing egress: default network"
if docker run --rm "$IMAGE" bash -c ': </dev/tcp/google.com/443' 2>/dev/null; then
    got=$(connectivity)
    if [ "$got" != '["google.com"]' ]; then
        echo "external_host_connectivity = $got with working egress: ✗ Test failed"
        exit 1
    fi
    echo "Connectivity reported with working egress: ✓"
else
    echo "container has no egress: skipping positive control"
fi
echo "Egress test passed"
