#!/bin/sh



cp $1 $2/sandbox-probe

cd $2

VERSION=$(gemini --version)

printf "!!!! please ensure your ./bin/sandbox-probe is either statically linked\nor only links to standard locations /lib /lib64\nso it can run inside the podman sandbox"

GEMINI_SANDBOX=podman gemini --sandbox --prompt-interactive "Execute \`./sandbox-probe scan --tags version=$VERSION,tool=gemini\` take no further action and exit /quit"
