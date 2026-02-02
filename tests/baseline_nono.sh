#!/bin/sh

mkdir -p $HOME/.sandbox-probe/tmp
TMPDIR=$(mktemp -d -p $HOME/.sandbox-probe/tmp)
echo "created TMPDIR: $TMPDIR"

# note, this isn't really the baseline, this is the most permissive nono
# if we can output a list of the built-in profiles we could have normal
# nono be baseline and then have a report for each profile
echo "Testing no sandbox mode (extra permissive) with Nono"

cp ./bin/sandbox-probe $TMPDIR/
OLDDIR="$PWD"
cd "$TMPDIR"

# file needs to exist before nono allows access
touch report.json

nono run --silent --allow-cwd --allow / ./sandbox-probe scan

# Display the report
if [ -f "$TMPDIR/report.json" ]; then
    echo "\n=== Report Generated ==="
    jq '.' $TMPDIR/report.json
else
    echo "ERROR: report.json not found"
    exit 1
fi

cd "$OLDDIR"

mkdir -p ./reports
cp $TMPDIR/report.json ./reports/baseline-nono.json
