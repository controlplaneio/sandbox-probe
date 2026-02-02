#!/bin/sh

cp $1 $2

cd $2

gemini -y -p "Execute !$1"