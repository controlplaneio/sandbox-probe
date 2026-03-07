#!/bin/sh

cp $1 $2

cd $2

VERSION=$(gemini --version)

# gemini doesn't allow tool call of `run_shell_command` when using non interactive prompts
# there is `--prompt-interactive` which can take a value but even with `--yolo`
# gemini will wait for user input and won't quit
# instead we pipe output into interaction
printf "Execute \`./sandbox-probe scan --tags version=%s,tool=gemini\` take no further action and exit /quit\n\n\n\n" "$VERSION" | gemini --yolo
