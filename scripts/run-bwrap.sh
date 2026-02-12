#!/bin/sh

# TODO: narrow down to just be able to run the executable

cp $1 $2

cd $2

BINARY_NAME=$(echo $1 | awk -F '/' '{print $NF}')

bwrap \
  --ro-bind /proc /proc \
  --ro-bind /usr /usr \
  --ro-bind /lib /lib \
  --ro-bind /lib64 /lib64 \
  --ro-bind /bin /bin \
  --ro-bind /sbin /sbin \
  --ro-bind /etc /etc \
  --bind $(pwd) /data \
  --bind /tmp /tmp \
  --chdir /data \
  --unshare-user \
  --unshare-ipc \
  --unshare-uts \
  --unshare-cgroup \
  --share-net \
  --die-with-parent \
  /data/${BINARY_NAME} scan --tasks baseline_sandbox_task
