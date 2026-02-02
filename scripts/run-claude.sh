#!/bin/sh

# TODO: narrow down to just be able to run the executable


cp $1 $2

cd $2

VERSION=$(claude --version)

claude --allowedTools "Bash" -p "Execute !$1 scan --tags version=${VERSION},tool=claude" 