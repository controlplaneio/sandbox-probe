# Linux dev-machine IPC/process footprint

Research for [`issues/06-linux-dev-machine-footprint.md`](../../.scratch/seed-ipc-targets/issues/06-linux-dev-machine-footprint.md),
feeding the [seed-ipc-targets catalogue ticket](../../.scratch/seed-ipc-targets/issues/09-populate-catalogue-and-registry.md).
Companion to the macOS data already gathered (see the map's Notes): Docker
Desktop's `vmnetd` socket, three VS Code git-integration sockets, four
`ssh-askpass` GUI-prompt sockets, two browsers' singleton sockets, and an AI
coding assistant's own daemon sockets.

## Method

Docker Desktop's `desktop-linux` context (a real Linux ARM64 VM, disposable)
was used to run one `ubuntu:22.04` container, into which representative dev
tooling was installed and started one piece at a time. After each addition,
the process table and the exact directories `ScanSocketRoots` walks
(`pkg/tasks/baseline/network.go`: `/run`, `/var/run`, `/dev`, `/tmp`,
`/var/tmp`, plus `$XDG_RUNTIME_DIR`/`$TMPDIR` when set) were inspected for
socket-typed files, to attribute each socket/process to the tool that made
it. The container was destroyed afterwards; nothing on the host was touched.

```
$ docker context ls
NAME              DESCRIPTION                               DOCKER ENDPOINT                             ERROR
default           Current DOCKER_HOST based configuration   unix:///var/run/docker.sock
desktop-linux *   Docker Desktop                            unix:///Users/cns/.docker/run/docker.sock

$ docker version --format '{{.Server.Os}}/{{.Server.Arch}}'
linux/arm64

$ docker run -d --name dev-footprint --privileged \
    -v /var/run/docker.sock:/hostdocker.sock ubuntu:22.04 sleep infinity
```

### Baseline (empty container, nothing installed)

```
$ docker exec dev-footprint ps aux
USER       PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
root         1  0.0  0.0   2228  1048 ?        Ss   13:53   0:00 sleep infinity
root         7  0.0  0.0   6448  2472 ?        Rs   13:53   0:00 ps aux

$ docker exec dev-footprint sh -c 'find /run /var/run /dev /tmp /var/tmp -type s 2>/dev/null'
(no output)

$ docker exec dev-footprint sh -c 'echo $XDG_RUNTIME_DIR'
(empty)
```

Confirms the container itself contributes nothing — every socket found
below was created by the tool being tested, and a plain container has no
`XDG_RUNTIME_DIR` (no systemd/logind session) unless one is set up by hand.

## Tools installed and what they left behind

Installed via `apt-get install -y openssh-client docker.io git curl gnupg
ca-certificates dbus`, plus `code-server` via its official install script
(`curl -fsSL https://code-server.dev/install.sh | sh`, landed
`code-server 4.130.0`, arm64 `.deb`).

### 1. `ssh-agent`

```
$ docker exec dev-footprint sh -c 'ssh-agent -a /tmp/does-not-matter'
SSH_AUTH_SOCK=/tmp/does-not-matter; export SSH_AUTH_SOCK;
SSH_AGENT_PID=3554; export SSH_AGENT_PID;

$ docker exec dev-footprint sh -c 'ssh-agent'   # default invocation, no -a
SSH_AUTH_SOCK=/tmp/ssh-XXXXXXrwjomb/agent.3575; export SSH_AUTH_SOCK;
SSH_AGENT_PID=3576; export SSH_AGENT_PID;
```

Sockets found: `/tmp/does-not-matter` (explicit `-a` path) and
`/tmp/ssh-XXXXXXrwjomb/agent.3575` (default naming: a random
`ssh-XXXXXXXXXX` directory under `/tmp`, containing `agent.<pid>`). Both
are inside `ScanSocketRoots`. The default naming convention (`/tmp/ssh-*/agent.*`)
is the realistic decoy shape — matches the macOS `ssh-askpass` finding's
sibling.

### 2. `dockerd` (Docker-in-Docker, i.e. native Linux Docker Engine — not Docker Desktop)

```
$ docker exec -d dev-footprint dockerd
$ docker exec dev-footprint ps aux
...
root      3590  2.2  0.1 2347636 78896 ?       Ssl  13:54   0:00 dockerd
root      3606  1.2  0.0 2160668 45476 ?       Ssl  13:54   0:00 containerd --config /var/run/docker/containerd/containerd.toml

$ docker exec dev-footprint sh -c 'find /run /var/run /dev /tmp /var/tmp -type s 2>/dev/null'
/run/docker/metrics.sock
/run/docker/libnetwork/118bc750693e.sock
/run/docker/containerd/containerd.sock.ttrpc
/run/docker/containerd/containerd-debug.sock
/run/docker/containerd/containerd.sock
/run/docker.sock
```

Six sockets, two processes. `/run/docker.sock` is the headline decoy target
(the one every "am I in a sandbox that can reach the Docker socket" check
looks for). This is the *native-Linux* equivalent of macOS's Docker Desktop
`vmnetd` socket — on Linux, `dockerd` talks directly over `/run/docker.sock`,
there is no separate VM-helper daemon.

### 3. `code-server` (headless VS Code server — the realistic stand-in for both
a self-hosted browser IDE and, structurally, for Microsoft's own
`~/.vscode-server` that Remote-SSH installs on a Linux devbox)

```
$ docker exec -d dev-footprint sh -c \
    'HOME=/root code-server --auth none --bind-addr 0.0.0.0:8080 /root'
[...] info  HTTP server listening on http://0.0.0.0:8080/
[...] info  Session server listening on /root/.local/share/code-server/code-server-ipc.sock

$ docker exec dev-footprint ps aux
...
root      3878  ...  /usr/lib/code-server/lib/node /usr/lib/code-server ...
root      3897  ...  /usr/lib/code-server/lib/node /usr/lib/code-server/out/node/entry

$ docker exec dev-footprint sh -c 'find /run /var/run /dev /tmp /var/tmp -type s 2>/dev/null'
(unchanged from step 2 — nothing new)

$ docker exec dev-footprint sh -c 'find / -xdev -type s 2>/dev/null | grep -v "^/run\|^/var/run\|^/dev\|^/tmp\|^/var/tmp"'
/root/.local/share/code-server/code-server-ipc.sock
```

**Notable finding**: code-server's own session IPC socket lives under
`$XDG_DATA_HOME` (`~/.local/share/code-server/`), *outside every directory
`ScanSocketRoots` walks*. A plain unauthenticated `GET /` to the HTTP port
did not spawn an extension host or pty-host process (those spawn on the
first real websocket/editor connection, which needs a browser client, not
just curl — see caveats). The macOS finding of "three VS Code
git-integration sockets" almost certainly are extension-host-spawned
sockets of this same kind, and were not reproduced here for the same
reason.

### 4. `gpg-agent` — socket location depends entirely on `$XDG_RUNTIME_DIR`

```
# Without XDG_RUNTIME_DIR set (this container's default state):
$ docker exec dev-footprint sh -c \
    'export GNUPGHOME=/root/.gnupg; gpgconf --launch gpg-agent'
$ find ... -type s
/root/.gnupg/S.gpg-agent
/root/.gnupg/S.gpg-agent.browser
/root/.gnupg/S.gpg-agent.ssh
/root/.gnupg/S.gpg-agent.extra

# With XDG_RUNTIME_DIR set (simulating a real logind/systemd desktop session):
$ docker exec dev-footprint sh -c \
    'gpgconf --kill gpg-agent; mkdir -p /run/user/0; chmod 700 /run/user/0; \
     export XDG_RUNTIME_DIR=/run/user/0 GNUPGHOME=/root/.gnupg; \
     gpgconf --launch gpg-agent'
$ find /run /var/run /dev /tmp /var/tmp -type s 2>/dev/null
/run/user/0/gnupg/S.gpg-agent
/run/user/0/gnupg/S.gpg-agent.browser
/run/user/0/gnupg/S.gpg-agent.extra
/run/user/0/gnupg/S.gpg-agent.ssh
```

Four sockets (main, browser, ssh, extra — one per gpg-agent protocol). On
any real logind-managed Linux desktop session `$XDG_RUNTIME_DIR` is always
set (e.g. `/run/user/1000`), so this is the realistic shape: these four
sockets land squarely inside `ScanSocketRoots` via the `$XDG_RUNTIME_DIR`
root. A container with no init/logind is the one environment where they'd
be missed — worth noting as a scanner blind spot if a sandbox similarly
lacks a real user session.

### 5. `dbus-daemon` (system + session bus)

```
$ docker exec dev-footprint sh -c 'dbus-daemon --system --fork'
$ docker exec dev-footprint sh -c \
    'export XDG_RUNTIME_DIR=/run/user/0; \
     dbus-daemon --session --fork --address=unix:path=/run/user/0/bus'

$ find /run /var/run /dev /tmp /var/tmp -type s 2>/dev/null
/run/dbus/system_bus_socket
/run/user/0/bus
...(plus everything above)
```

Two more sockets: `/run/dbus/system_bus_socket` (system bus, always present
on any real Linux desktop — used by NetworkManager, logind, udisks, etc.)
and `$XDG_RUNTIME_DIR/bus` (per-user session bus). D-Bus is a much bigger
part of a real Linux desktop's IPC surface than of macOS's — worth
including in the catalogue even though no specific dev *tool* claims it.

## Final combined state

```
$ docker exec dev-footprint ps aux
USER       PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
root         1  0.0  0.0   2228  1048 ?        Ss   13:53   0:00 sleep infinity
root      3554  0.0  0.0   7080  2136 ?        Ss   13:53   0:00 ssh-agent -a /tmp/does-not-matter
root      3576  0.0  0.0   7080  2132 ?        Ss   13:53   0:00 ssh-agent
root      3590  0.0  0.1 2347636 78960 ?       Ssl  13:54   0:00 dockerd
root      3606  0.2  0.0 2160668 47748 ?       Ssl  13:54   0:00 containerd --config /var/run/docker/containerd/containerd.toml
root      3872  0.0  0.0   2332  1404 ?        Ss   13:54   0:00 sh -c HOME=/root code-server ...
root      3878  0.1  0.1 1260820 74616 ?       Sl   13:54   0:00 /usr/lib/code-server/lib/node /usr/lib/code-server ...
root      3897  0.5  0.2 1508176 113864 ?      Sl   13:54   0:00 /usr/lib/code-server/lib/node /usr/lib/code-server/out/node/entry
root      4007  0.0  0.0  77732  2436 ?        Ss   13:55   0:00 gpg-agent --homedir /root/.gnupg --use-standard-socket --daemon
message+  4034  0.0  0.0   7620  2332 ?        Ss   13:56   0:00 dbus-daemon --system --fork
root      4042  0.0  0.0   7620  2268 ?        Ss   13:56   0:00 dbus-daemon --session --fork --address=unix:path=/run/user/0/bus

$ docker exec dev-footprint sh -c 'find /run /var/run /dev /tmp /var/tmp -type s 2>/dev/null | sort'
/run/dbus/system_bus_socket
/run/docker.sock
/run/docker/containerd/containerd-debug.sock
/run/docker/containerd/containerd.sock
/run/docker/containerd/containerd.sock.ttrpc
/run/docker/libnetwork/118bc750693e.sock
/run/docker/metrics.sock
/run/user/0/bus
/run/user/0/gnupg/S.gpg-agent
/run/user/0/gnupg/S.gpg-agent.browser
/run/user/0/gnupg/S.gpg-agent.extra
/run/user/0/gnupg/S.gpg-agent.ssh
/tmp/does-not-matter
/tmp/ssh-XXXXXXrwjomb/agent.3575

$ docker exec dev-footprint sh -c \
    'stat -c "%F %n" /run/docker.sock /run/user/0/gnupg/S.gpg-agent /tmp/ssh-*/agent.*'
socket /run/docker.sock
socket /run/user/0/gnupg/S.gpg-agent
socket /tmp/ssh-XXXXXXrwjomb/agent.3575
```

14 socket-typed files, spot-checked with `stat` to confirm they're genuinely
socket type, not just named like one. Container removed afterwards
(`docker rm -f dev-footprint`) — nothing persisted on the host.

## Attribution summary

| Socket(s) | Tool | Inside `ScanSocketRoots`? |
|---|---|---|
| `/run/docker.sock`, `/run/docker/containerd/{containerd.sock,containerd.sock.ttrpc,containerd-debug.sock}`, `/run/docker/libnetwork/<id>.sock`, `/run/docker/metrics.sock` | `dockerd`/`containerd` (native Linux Docker Engine) | yes (`/run`) |
| `/tmp/ssh-XXXXXXXXXX/agent.<pid>` (default) | `ssh-agent` | yes (`/tmp`) |
| `$XDG_RUNTIME_DIR/gnupg/S.gpg-agent{,.browser,.ssh,.extra}` | `gpg-agent` | yes, but **only if `$XDG_RUNTIME_DIR` is set** (falls back to `~/.gnupg` otherwise — outside scanned roots) |
| `/run/dbus/system_bus_socket` | system `dbus-daemon` | yes (`/run`) |
| `$XDG_RUNTIME_DIR/bus` | session `dbus-daemon` | yes, same caveat as gpg-agent |
| `~/.local/share/code-server/code-server-ipc.sock` | `code-server` | **no** — outside every scanned root |

Processes attributable to specific tools: `sleep infinity` (container init
placeholder, not a real finding), `ssh-agent` (x2, one per invocation),
`dockerd`, `containerd`, `code-server`'s launcher shell + two `node`
processes, `gpg-agent`, `dbus-daemon` (x2, system + session).

## What a container can and cannot represent

**Can represent** (all reproduced above, empirically):

- Background daemons and their control sockets: `dockerd`/`containerd`,
  `gpg-agent`, `dbus-daemon`, `ssh-agent` — anything that's really just a
  Linux process bound to a Unix socket under `/run`, `/tmp`, or
  `$XDG_RUNTIME_DIR`.
- A headless IDE server process and its non-GUI IPC socket (`code-server`).
- The dependency of several sockets' location on `$XDG_RUNTIME_DIR` being
  set — itself a useful finding: a container with no init/logind is *not*
  representative of a real desktop session for that reason alone, and a
  sandbox that similarly lacks a real login session would show the same
  gap.

**Cannot represent** (container-fundamental, not just "wasn't tried here"):

- **No GUI at all**: no `ssh-askpass` GUI prompt socket (that's a
  polkit/desktop-agent-invoked askpass binary reacting to a *graphical*
  prompt request — meaningless with no display), no desktop keychain/agent
  UI.
- **No browser**: browser singleton sockets (`SingletonSocket` for Chrome,
  Firefox's `.parentlock`/multiprocess socket) require an actual browser
  process with a profile directory and, typically, a display to attach to;
  installing a browser binary in a container without a display doesn't
  produce the same singleton-lock IPC surface a running desktop browser
  does.
- **No real VS Code editor session**: `code-server`'s extension host and
  pty-host processes (and *their* sockets — the likely source of macOS's
  "three VS Code git-integration sockets") only spawn once a real
  editor/websocket client connects and opens a workspace; a bare `curl` GET
  to the HTTP port doesn't trigger this. Driving that would need a real
  browser or a websocket-speaking client, which is out of scope for a
  plain container investigation.
- **No logind/systemd user session by default**: `$XDG_RUNTIME_DIR` had to
  be created and exported by hand to reproduce the gpg-agent/D-Bus
  session-bus behaviour a real desktop login gets for free. This is a
  genuine environmental difference, not a container limitation as such —
  but it means container-only testing under-counts sockets that depend on
  it unless you set it up explicitly, as done here.

In short: this container investigation is solid ground truth for the
*daemon-and-socket* half of a Linux dev machine's IPC footprint (Docker,
ssh-agent, gpg-agent, D-Bus, a headless IDE server), and honestly cannot
speak to the *desktop-session* half (GUI prompts, browsers, a live editor
workspace) — those need either a real Linux desktop VM or are reasoned by
analogy from the macOS findings rather than independently reproduced here.
