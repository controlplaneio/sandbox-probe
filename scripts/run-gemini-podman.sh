#!/bin/sh



cp $1 $2

cd $2

GEMINI_SANDBOX=podman gemini -y --sandbox -p "Execute !$1 scan"