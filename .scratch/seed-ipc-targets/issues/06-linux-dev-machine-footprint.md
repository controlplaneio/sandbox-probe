Type: research
Status: resolved

## Question

What does a representative Linux developer machine's IPC/process footprint
actually look like — real sockets and processes worth seeding as decoys,
analogous to the macOS data already gathered (Docker's `vmnetd`, VS Code
git-integration sockets, `ssh-askpass`, browser singleton sockets, Claude
Code's own daemon sockets — see the map's Notes)?

Investigate empirically using Docker Desktop's `desktop-linux` VM (already
running locally, disposable): install/run representative dev tooling
(Docker-in-Docker or the Docker CLI talking to the host daemon, VS Code
Server / a headless IDE session, an SSH agent, anything else genuinely
common) and observe what actually shows up under the runtime dirs
`ScanSocketRoots` walks (`/run`, `/var/run`, `/tmp`, `$XDG_RUNTIME_DIR`) and
in the process table.

Be honest about what "a Linux dev machine" means here — a container is not
a full desktop session, so note explicitly which parts of the footprint a
container can and can't represent (no GUI browser, no desktop ssh-agent
GUI prompt, etc.) rather than silently presenting container-only findings
as the full picture.

Write findings to a Markdown file in the repo, citing what was actually
observed (commands run, output). Capture on a throwaway
`research/linux-dev-footprint` branch.

## Answer

Full write-up with every command/raw output: `docs/research/linux-dev-machine-footprint.md`
on branch `research/linux-dev-footprint` (commit `10497a8`). Method: an
`ubuntu:22.04` container on Docker Desktop's `desktop-linux` VM, one tool
installed/started at a time, diffing the process table and every
`ScanSocketRoots` dir after each.

**Sockets/processes found, attributed to real tools:**
- `dockerd`/`containerd` (Docker-in-Docker): `/run/docker.sock` plus 5 more
  (`containerd.sock`, `.sock.ttrpc`, `containerd-debug.sock`,
  `libnetwork/<id>.sock`, `metrics.sock`) — the Linux analog of macOS's
  `vmnetd`.
- `ssh-agent`: `/tmp/ssh-XXXXXXXXXX/agent.<pid>`.
- `gpg-agent`: 4 sockets (`S.gpg-agent{,.browser,.ssh,.extra}`) — location
  depends entirely on `$XDG_RUNTIME_DIR`: `/run/user/<uid>/gnupg/`
  (in-scope for `ScanSocketRoots`) if set, falls back to `~/.gnupg`
  (out-of-scope) if not.
- `dbus-daemon`: system bus `/run/dbus/system_bus_socket` + session bus
  `$XDG_RUNTIME_DIR/bus`.
- `code-server` (headless VS Code, stand-in for Remote-SSH's
  `~/.vscode-server`): its own IPC socket at
  `~/.local/share/code-server/code-server-ipc.sock` — notably **outside
  every existing `ScanSocketRoots` dir**, a real gap the current scanner
  roots don't cover.

**Honest limits** (per the ticket's own instruction not to overclaim): a
container fully represents background-daemon sockets (Docker, ssh-agent,
gpg-agent, D-Bus, a headless IDE server) and even surfaced a real
environmental gap (no `$XDG_RUNTIME_DIR` without logind/systemd — had to
fake it). It cannot represent GUI `ssh-askpass` prompts, browser singleton
sockets, or VS Code's extension-host/git-integration sockets (only spawn on
a real editor/websocket connection) — those are reasoned by analogy to the
macOS findings already gathered, not independently reproduced here.

Consequence for [ticket 09](09-populate-catalogue-and-registry.md): the
`code-server` IPC socket path is a concrete example of a real target
outside the current scan roots — the catalogue ticket should decide whether
`ScanSocketRoots` itself needs a new root, not just a new seeded target.
