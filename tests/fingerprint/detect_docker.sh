#!/bin/sh

mkdir -p "$HOME/.sandbox-probe/tmp"
TMPDIR=$(mktemp -d -p "$HOME/.sandbox-probe/tmp")
echo "created TMPDIR: $TMPDIR"

# Get the directory of this script
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "Testing docker"

# Run the probe in Claude sandbox using absolute paths
"${PROJECT_ROOT}/scripts/run-docker.sh" "bin/sandbox-probe" "$TMPDIR"


# Display the report
if [ -f "$TMPDIR/report.json" ]; then
    printf '\n=== Report Generated ===\n'
    jq '.' "$TMPDIR/report.json"

    mkdir -p ./reports
    cp "$TMPDIR/report.json" ./reports/sandbox-docker.json

    # Verify Docker detection - produces boolean
    printf '\n=== Verifying Docker Detection ===\n'
    pass=$(jq 'any(.findings[]; .findingType == "sandbox_detection" and .task == "baseline_sandbox_detector" and .value == "docker")' "$TMPDIR/report.json")
    if [ "$pass" = "true" ]; then
        echo "Docker detected: ✓ Test passed"
        exit 0
    else
        echo "Docker not detected: ✗ Test failed"
        exit 1
    fi
else
    echo "ERROR: report.json not found"
    exit 1
fi
