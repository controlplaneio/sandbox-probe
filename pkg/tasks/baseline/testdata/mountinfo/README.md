# Mount-table fixtures

Real `/proc/…/mountinfo` captures, one per runtime, driving `Test_GetHostMounts`.
None is hand-written: a hand-written table would tune the enumerator to an
imagined mount table, which is how the current source-shape heuristic came about.

All three were captured on 2026-07-31 from an arm64 macOS host running Docker
Desktop 29.6.1 (Linux VM kernel, virtiofs host shares). `$BIND` below is a
throwaway directory on the macOS host holding a single marker file.

## `gvisor-bind.mountinfo`

**Runtime:** gVisor `runsc` release-20260727.0 (aarch64), `--platform` default
(systrap), `--rootless --network=none --ignore-cgroups`, run inside a privileged
Debian container because the macOS host cannot run `runsc` directly.

**Configuration:** an OCI bundle from `runsc spec` over an exported `alpine`
rootfs, with one extra mount added to `config.json` — a read-only bind of the
host directory at `/hostbind` — and `process.args` set to
`cat /proc/self/mountinfo`:

```sh
docker run --rm --privileged \
  -v "$BIND:/hostbindsrc:ro" -v "$PWD/alpine.tar:/alpine.tar:ro" \
  debian:bookworm-slim bash -c '
    apt-get update -qq && apt-get install -y -qq wget jq
    wget -q "https://storage.googleapis.com/gvisor/releases/release/latest/$(uname -m)/runsc" \
      -O /usr/local/bin/runsc && chmod 755 /usr/local/bin/runsc
    mkdir -p /bundle/rootfs && tar -C /bundle/rootfs -xf /alpine.tar && cd /bundle
    runsc spec
    jq ".process.terminal=false
        | .process.args=[\"cat\",\"/proc/self/mountinfo\"]
        | .mounts += [{\"destination\":\"/hostbind\",\"type\":\"bind\",
                       \"source\":\"/hostbindsrc\",\"options\":[\"bind\",\"ro\"]}]" \
      config.json > c2.json && mv c2.json config.json
    runsc --rootless --network=none --ignore-cgroups run probe-mnt'
```

The `/hostbind` entry is the bind that is reachable from inside and that the
current filter drops.

## `docker-bind.mountinfo`

**Runtime:** Docker 29.6.1 (Docker Desktop for Mac), `alpine` container.

**Configuration:** started with one explicit read-only bind of the host
directory at `/mnt/hostbind`:

```sh
docker run --rm -v "$BIND:/mnt/hostbind:ro" alpine cat /proc/self/mountinfo
```

## `unconfined-host.mountinfo`

**Runtime:** none — the Linux host itself, the Docker Desktop VM, read from PID 1
in the host mount namespace:

```sh
docker run --rm --privileged --pid=host alpine cat /proc/1/mountinfo
```
