#!/bin/sh



cp $1 $2

cd $2

VERSION=$(gemini --version)

GEMINI_SANDBOX=podman gemini -y --sandbox -p "Execute !$1 scan --tags version=${VERSION},tool=gemini"