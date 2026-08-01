#!/bin/sh

mkdir -p "$HOME/.sandbox-probe/tmp"
TMPDIR=$(mktemp -d -p "$HOME/.sandbox-probe/tmp")
echo "created TMPDIR: $TMPDIR"

# note, this isn't really the baseline, this is the most permissive nono
# if we can output a list of the built-in profiles we could have normal
# nono be baseline and then have a report for each profile
echo "Testing no sandbox mode (extra permissive) with Nono"

cp ./bin/sandbox-probe "$TMPDIR/"
OLDDIR="$PWD"
cd "$TMPDIR" || exit

# This is intentionally relaxed: it exposes the probe's runtime, system and
# credential target paths while excluding nono's protected state root.
PROFILE="$OLDDIR/tests/nono/relaxed-baseline.json"
if ! nono run --silent --profile "$PROFILE" ./sandbox-probe scan; then
    echo "ERROR: relaxed nono baseline could not initialize"
    exit 1
fi

# Display the report
if [ -f "$TMPDIR/report.json" ]; then
    printf '\n=== Report Generated ===\n'
    jq '.' "$TMPDIR/report.json"
else
    echo "ERROR: report.json not found"
    exit 1
fi

cd "$OLDDIR" || exit

mkdir -p ./reports
cp "$TMPDIR/report.json" ./reports/baseline-nono.json
