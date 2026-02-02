#!/bin/sh

# TODO: narrow down to just be able to run the executable


cp $1 $2

cd $2


podman run  -w /data/ --rm -it -v $(pwd):/data ubuntu:latest /data/$(echo $1 | awk -F '/' '{print $NF}')