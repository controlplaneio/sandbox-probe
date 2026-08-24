# Research: does a sandbox's "can't see the host socket" mean anything?

Source ticket: [`.scratch/seed-ipc-targets/issues/08-namespace-parity-semantics.md`](https://github.com/controlplaneio/sandbox-probe-reports/blob/main/.scratch/seed-ipc-targets/issues/08-namespace-parity-semantics.md) in `sandbox-probe-reports`

## Question

For a file on a shared filesystem mount, "the sandboxed run can't see it" is a
clean, meaningful 🟩 — the sandbox actively blocked something that was
provably there on both sides. For a Unix domain socket bound on the host, does
the same logic hold, or does a mount/network-namespaced sandbox (Docker, etc.)
structurally never see a host-bound socket at all, **regardless of policy** —
making "sandboxed run doesn't see the seeded socket" a false-positive 🟩?

## Answer, up front

**It is purely mount-namespace / bind-mount driven, exactly like files — not
namespace isolation acting as an implicit policy.** A plain `docker run` with
no extra flags never sees a host-bound Unix socket, for the identical reason
it never sees an arbitrary host file: the container gets its own mount
namespace, and nothing propagates into it except what's explicitly bind
mounted (`-v`/`--mount`). There is no separate "socket firewall" in Docker —
sockets are ordinary filesystem inodes (`unix(7)`: a pathname socket "is bound
to a system-wide address ... entry in the file system"), so their visibility
follows the same mount-namespace rule as any regular file, and their *result*
(missing entirely, `ENOENT`) is indistinguishable from the reason a decoy file
outside a shared mount is invisible. It is not evidence the sandbox's
enforcement policy (seccomp/AppArmor/capabilities) did anything — it is
evidence only that nobody bind-mounted that path.

Conversely, once the containing path *is* shared (the same `-v host:/data`
mechanism `run-docker.sh` already uses for the probe's own data dir), the
socket is not just stat-able — it is a **live, connectable IPC channel**
straight through to the original listening process, even across separate PID
and network namespaces. So sandbox_probe's socket scanner is measuring mount
topology, not runtime policy, unless the seeded socket sits inside whatever
region is already shared with the sandboxed run.

## How sandbox_probe actually launches a "sandboxed run"

Read first, per the ticket:

- `tests/detect_docker.sh` calls `scripts/run-docker.sh "bin/sandbox-probe" $TMPDIR`.
- `scripts/run-docker.sh`:
  ```sh
  docker run -w /data/ --rm -it -v $(pwd):/data ubuntu:latest \
    /data/$(echo $1 | awk -F '/' '{print $NF}') scan --tasks baseline_sandbox_task --tasksets none
  ```

So the project's own "sandbox run" methodology is: default Docker isolation,
**one** explicit bind mount of a throwaway scratch dir (`$TMPDIR`) containing
the binary and its report — nothing else. No `--network=host`, no
`--pid=host`, no socket mounts. This is the exact configuration replicated
below.

`pkg/tasks/baseline/network.go`'s `DefaultSocketRoots()` scans
`/run`, `/var/run`, `/dev`, `/tmp`, `/var/tmp`, `/private/*` and
`$XDG_RUNTIME_DIR`/`$TMPDIR` for pathname sockets via `ScanSocketRoots`/
`GetSockets` — i.e. exactly the runtime-dir paths this experiment targets.

## Experiment

Docker Desktop (`desktop-linux` context, `linux/arm64`, kernel
`6.12.76-linuxkit`) was used as a disposable Linux VM. Everything below runs
inside that VM's containers; nothing touched the real Mac's `/tmp` or `/run`.

### Setup: a "host" container binds a real Unix socket at a scanned path

```
$ docker run -d --name probehost ubuntu:latest sleep infinity
$ docker exec probehost mkdir -p /run/probe-test
$ docker exec -d probehost python3 -c "
import socket, os
p = '/run/probe-test/agent.sock'
s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
s.bind(p); s.listen(1)
while True:
    conn, _ = s.accept()
    conn.sendall(b'hello from host socket\n'); conn.close()
"
$ docker exec probehost ls -la /run/probe-test/
srwxr-xr-x 1 root root 0 Jul 28 13:57 agent.sock
$ docker exec probehost sh -c "python3 -c \"import socket; s=socket.socket(socket.AF_UNIX); \
    s.connect('/run/probe-test/agent.sock'); print(s.recv(100))\""
b'hello from host socket\n'
```

`/run/probe-test/agent.sock` is a live, connectable pathname socket — this is
our "host-bound socket seeded at a `ScanSocketRoots` path".

### Experiment A — default isolation, mirroring `run-docker.sh` exactly

A sibling container, launched the same way `run-docker.sh` launches the
sandboxed probe (only a scratch data dir bind mounted, nothing else):

```
$ docker run --rm -v $SCRATCH/nsdata:/data ubuntu:latest ls -la /run
total 16
drwxr-xr-x 4 root root 4096 ... .
drwxr-xr-x 2 root root 4096 ... lock
drwxr-xr-x 2 root root 4096 ... systemd
# no probe-test directory at all — it's not merely empty, the path never existed

$ docker run --rm ubuntu:latest stat /run/probe-test/agent.sock
stat: cannot stat '/run/probe-test/agent.sock': No such file or directory (os error 2)
exit=1
```

Result: `ENOENT`, not `EACCES`/`EPERM`. Nothing in the container's `/run`
mount corresponds to the host container's `/run` at all — Docker builds a
fresh tmpfs-backed `/run` per container. This is structural absence, not a
denied access.

### Experiment B — path explicitly shared (mirrors the project's `-v host:/data`)

Recreated with a Docker named volume standing in for "the same directory
bind-mounted on both sides" (kept inside the Docker Desktop VM, not the real
Mac filesystem):

```
$ docker volume create probe-ns-test
$ docker run -d --name probehost2 -v probe-ns-test:/run/probe-test ubuntu:latest sleep infinity
$ docker exec -d probehost2 python3 -c "... bind /run/probe-test/agent.sock, listen, reply ..."

# Sibling WITHOUT the volume (default isolation) — same as Experiment A:
$ docker run --rm ubuntu:latest stat /run/probe-test/agent.sock
stat: cannot stat '/run/probe-test/agent.sock': No such file or directory (os error 2)

# Sibling WITH the same volume mounted (explicit share):
$ docker run --rm -v probe-ns-test:/run/probe-test ubuntu:latest stat /run/probe-test/agent.sock
  File: /run/probe-test/agent.sock
  ...
  size: 0 ... socket

# Not just visible — actually connects through to the ORIGINAL listener,
# in a container with its own separate PID and network namespace:
$ docker run --rm -v probe-ns-test:/run/probe-test python:3-slim python3 -c "
import socket
s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
s.connect('/run/probe-test/agent.sock')
print(s.recv(200))
"
b'hello from probehost2 real socket\n'
```

### Control — does sharing the *network* namespace matter?

`ScanSocketRoots` only walks pathname sockets on disk, so it's worth pinning
down which namespace actually gates them. Sharing the network namespace
(`--network=host`) without any mount sharing:

```
$ docker run --rm --network=host ubuntu:latest stat /run/probe-test/agent.sock
stat: cannot stat '/run/probe-test/agent.sock': No such file or directory (os error 2)
```

Still `ENOENT`. Confirms pathname-socket visibility is gated by the **mount**
namespace specifically, not the network namespace, and not some
Docker-specific "socket policy."

## Why this is true (kernel/namespace mechanism, not just this one experiment)

- **`unix(7)`** (Linux man-pages): a Unix domain "pathname" socket is created
  by `bind(2)` to a path and "exists in the file system and can be interacted
  with ... using the usual filesystem-related system calls." It is an
  ordinary filesystem object (type `S_IFSOCK`) with an inode — there is
  nothing about it that is inherently namespace-aware. The same man page notes
  the one socket flavour that *is* independently namespaced: **abstract**
  sockets (`sun_path[0] == '\0'`) have "Linux network namespace scope" — i.e.
  abstract sockets ride on the network namespace, but pathname sockets do not;
  they ride purely on whatever's in the mount table.
- **`namespaces(7)`** and **`mount_namespaces(7)`**: a mount namespace
  (`CLONE_NEWNS`) gives a process its own view of the filesystem mount table.
  Docker gives every container its own mount namespace by default and
  populates it only with the container's image layers plus whatever the user
  explicitly bind-mounts (`docs.docker.com/engine/storage/bind-mounts/`,
  `.../storage/volumes/`) — nothing from the host's `/run`, `/tmp` etc.
  propagates unless asked for. A pathname socket's dentry is exactly as
  invisible across that boundary as a regular file's would be, for exactly
  the same reason.
- **`network_namespaces(7)`**: governs interfaces, routes, ports — i.e. `AF_INET`/`AF_INET6` sockets and Linux abstract-namespace `AF_UNIX` addresses. It has no bearing on filesystem visibility, which is why `--network=host` above changed nothing for a pathname socket.

Put together: whether a container can see (and, if the path is shared, fully
communicate with) a pathname Unix socket is decided by exactly one thing —
is that path present in the container's mount namespace — the same variable
that already governs file decoys. There's no additional, socket-specific
sandboxing layer to worry about; sockets don't get a security property that
files don't already have.

## Practical consequence for sandbox_probe

Per `scripts/seed-decoys.sh`'s existing "PARITY IS LOAD-BEARING" comment for
file decoys: seeding only the baseline run means a sandboxed run "not
finding" the decoy is meaningless, because it was never structurally
reachable in the first place, independent of any policy. The exact same
argument applies to socket decoys, with one important twist:

- File decoys under `list-targets`/`seed-decoys.sh` are (by construction)
  planted under paths inside `$HOME`/workspace regions that both the
  baseline and the sandboxed run are expected to share — so parity seeding
  there closes a real gap.
- Socket decoys that `DefaultSocketRoots` scans for (`/run`, `/var/run`,
  `/dev`, `/tmp`, `/var/tmp`, `$XDG_RUNTIME_DIR`, `$TMPDIR`) sit in runtime
  directories that a container-based sandbox run (`run-docker.sh` et al.)
  **never shares by default** — only the one throwaway `$TMPDIR` data dir is
  mounted, and that is not one of the scanned roots on the container side.
  So today, a Docker/Podman/gVisor sandboxed run of sandbox-probe will
  *always* report zero sockets found, for every single sandbox, regardless of
  how tight or loose that sandbox's actual policy is — a permanent, contentless
  🟩 rather than a signal.

**Yes: socket decoys must be seeded inside the sandboxed environment too**,
mirroring the baseline/sandbox parity `seed-decoys.sh` already enforces for
files — otherwise every sandboxed run's "no sockets visible" result is an
artifact of container plumbing, not a measurement of anything the sandbox's
policy chose to allow or deny. Concretely, that means either (a) seeding a
decoy socket at a path that *is* part of whatever gets bind-mounted into the
sandboxed run (so the comparison is apples-to-apples), or (b) running the
seeder a second time from inside the sandboxed container against its own
local runtime dirs before the probe scans them — the same "seed on both
sides of the fence" rule the file decoys already follow.
