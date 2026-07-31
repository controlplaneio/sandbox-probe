#!/bin/sh

mkdir -p $HOME/.sandbox-probe/tmp
TMPDIR=$(mktemp -d -p $HOME/.sandbox-probe/tmp)
echo "created TMPDIR: $TMPDIR"

echo "Testing sandbox mode (normal usage) with Nono"

cp ./bin/sandbox-probe $TMPDIR
OLDDIR="$PWD"
cd "$TMPDIR"

# file needs to exist before nono allows access
touch report.json

nono run --silent --allow-cwd --allow-file ./report.json ./sandbox-probe scan
# need to --allow-cwd for unprompted non-interactive
# may be addressed in update

# # extra locked down alternative
# nono run --net-block --read-file ./sandbox-probe --allow-cwd --allow-file ./report.json ./sandbox-probe scan
# # super locked down alternative
# nono run --net-block --read-file ./sandbox-probe --allow-cwd --allow-command ./sandbox-probe --allow-file ./report.json ./sandbox-probe scan

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
cp $TMPDIR/report.json ./reports/sandbox-nono.json
