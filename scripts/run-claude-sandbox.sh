#!/bin/sh



cp $1 $2

cd $2

# TODO: narrow down to just be able to run the executable
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

VERSION=$(claude --version)

claude --settings ${SCRIPT_DIR}/config/claude-settings.json --allowedTools "Bash" -p "Execute !$1 scan --tags version=${VERSION},tool=claude,sandbox=true" 