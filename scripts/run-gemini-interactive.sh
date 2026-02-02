#!/bin/sh

cp $1 $2

cd $2

VERSION=$(gemini --version)

gemini --prompt-interactive "Execute \`./sandbox-probe scan --tags version=$VERSION,tool=gemini\` take no further action and exit /quit"
