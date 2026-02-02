#!/bin/sh

mkdir -p $HOME/.sandbox-probe/tmp
TMPDIR=$(mktemp -d -p $HOME/.sandbox-probe/tmp)
echo "created TMPDIR: $TMPDIR"

# Get the directory of this script
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "Testing podman"

# Run the probe in Claude sandbox using absolute paths
"${PROJECT_ROOT}/scripts/run-podman.sh" "bin/sandbox-probe" $TMPDIR


# Display the report
if [ -f "$TMPDIR/report.json" ]; then
    echo "\n=== Report Generated ==="
    jq '.' $TMPDIR/report.json

    # Verify podman detection - produces boolean
    echo "\n=== Verifying podman Detection ==="
    pass=$(jq 'any(.findings[]; .findingType == "sandbox_detection" and .task == "baseline_sandbox_detector" and .value == "podman")' $TMPDIR/report.json)
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
