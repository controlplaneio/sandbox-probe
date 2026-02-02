#!/bin/sh



cp $1 $2/sandbox-probe

cd $2

VERSION=$(gemini --version)

printf "!!!! please ensure your ./bin/sandbox-probe is either statically linked\nor only links to standard locations /lib /lib64\nso it can run inside the podman sandbox"

# gemini doesn't allow tool call of `run_shell_command` when using non interactive prompts
# there is `--prompt-interactive` which can take a value but even with `--yolo`
# gemini will wait for user input and won't quit
# instead we pipe output into interaction
printf "Execute \`./sandbox-probe scan --tags version=%s,tool=gemini\` take no further action and exit /quit\n\n\n\n" "$VERSION" | GEMINI_SANDBOX=podman gemini -y --sandbox
