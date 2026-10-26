#!/bin/sh

# Skip rather than fail when the runtime is absent, which is what `make e2etests`
# advertises and what lets one CI job run the docker row without every other row
# turning a missing runtime into a red build.
if ! { command -v podman >/dev/null 2>&1 && podman info >/dev/null 2>&1; }; then
    echo "SKIP: podman not available (binary missing or not usable)"
    exit 0
fi
mkdir -p "$HOME/.sandbox-probe/tmp"
TMPDIR=$(mktemp -d -p "$HOME/.sandbox-probe/tmp")
echo "created TMPDIR: $TMPDIR"

# Get the directory of this script
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "Testing podman"

# Run the probe in Claude sandbox using absolute paths
"${PROJECT_ROOT}/scripts/run-podman.sh" "bin/sandbox-probe" "$TMPDIR"


# Display the report
if [ -f "$TMPDIR/report.json" ]; then
    printf '\n=== Report Generated ===\n'
    jq '.' "$TMPDIR/report.json"

    # Verify podman detection - produces boolean
    mkdir -p ./reports
    cp "$TMPDIR/report.json" ./reports/sandbox-podman.json

    printf '\n=== Verifying podman Detection ===\n'
    pass=$(jq 'any(.findings[]; .findingType == "sandbox_detection" and .task == "baseline_sandbox_detector" and .value == "podman")' "$TMPDIR/report.json")
    if [ "$pass" = "true" ]; then
        echo "podman detected: ✓ Test passed"
        exit 0
    else
        echo "podman not detected: ✗ Test failed"
        exit 1
    fi
else
    echo "ERROR: report.json not found"
    exit 1
fi

