#!/bin/sh

mkdir -p $HOME/.sandbox-probe/tmp
TMPDIR=$(mktemp -d -p $HOME/.sandbox-probe/tmp)
echo "created TMPDIR: $TMPDIR"

# Get the directory of this script
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "Testing no sandbox mode with Claude"

# Run the probe in Claude sandbox using absolute paths
"${PROJECT_ROOT}/scripts/run-claude.sh" "bin/sandbox-probe" $TMPDIR


# Display the report
if [ -f "$TMPDIR/report.json" ]; then
    echo "\n=== Report Generated ==="
    jq '.' $TMPDIR/report.json
else
    echo "ERROR: report.json not found"
    exit 1
fi

mkdir -p ./reports
cp $TMPDIR/report.json ./reports/baseline-claude.json
