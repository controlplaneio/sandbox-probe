Type: task
Status: resolved

## Question

Capture what a representative Windows developer machine's named-pipe and
process footprint actually looks like, using the existing `Win11.utm` VM
(disposable — safe to install/trash freely in there).

No agent automation exists into the UTM guest today (`utmctl` controls VM
lifecycle, not in-guest command execution), so this is hands-on for Chris:

1. Boot `Win11.utm`.
2. Install/run representative dev tooling — Docker Desktop for Windows (
   named pipe `\\.\pipe\docker_engine`), VS Code, an SSH client with agent
   forwarding, a browser.
3. Capture a directory-style listing of `\\.\pipe\` (e.g.
   `[System.IO.Directory]::GetFiles("\\\\.\\pipe\\")` in PowerShell) and a
   process list, before and after each tool starts, to attribute pipes to
   tools.
4. Drop the raw capture somewhere in this ticket's answer or a linked file.

This unblocks [the catalogue ticket](09-populate-catalogue-and-registry.md)
alongside the Linux research and the existing macOS data.

## Answer

**Turned out more automatable than assumed** — Chris did the one-time
manual step (installing UTM's guest tools ISO inside the VM), and once the
QEMU guest agent was running, `utmctl exec`/`utmctl file` drove essentially
everything else. Full write-up:
`docs/research/windows-dev-machine-footprint.md` on branch
`research/windows-dev-machine-footprint` (commit `7803756`).

**Real findings**:
- **OpenSSH ships pre-installed on Windows 11** — no seeding needed for the
  client itself. `ssh-agent` is a one-line service-start
  (`Start-Service ssh-agent`), producing `\\.\pipe\openssh-ssh-agent` —
  confirmed via a real before/after directory diff. Cheapest, cleanest
  catalogue entry this ticket found.
- **A hard structural wall, confirmed not assumed**: `utmctl exec` runs as
  `nt authority\system` in Windows **Session 0** — the services session,
  isolated from the interactive desktop since Vista specifically so
  services can never show/drive GUI dialogs. Triggering a VS Code silent
  install proved this directly: the installer process stayed alive
  (small nonzero CPU) but **no process had any `MainWindowTitle` at all**
  — any dialog it raised was on a desktop nobody could ever see or click,
  automation or a human looking at the VM screen alike. Killed cleanly, no
  trace left.
- **Docker Desktop and VS Code not empirically captured** as a result —
  their pipe/IPC paths (`\\.\pipe\docker_engine`, VS Code's
  `vscode-ipc-*` pattern) are documented from other sources, not verified
  here, and should be labeled with that distinction in the catalogue, not
  presented as equally solid as the ssh-agent finding.
- Two real PowerShell gotchas documented for next time: `utmctl exec`
  doesn't wait out the full child-process lifetime (race pulling results
  too soon), and `Invoke-WebRequest`'s default progress rendering makes
  downloads far slower with no console attached.

**Unblocking path for Docker Desktop/VS Code, not attempted this session**:
either a one-time manual install by Chris in the VM's real interactive
session (same pattern as the guest-tools install), after which `utmctl
exec` can drive everything else against the now-installed tool; or
Task Scheduler as a known technique for bridging into the interactive
session from a non-interactive caller (untested).

[Ticket 09 (catalogue)](09-populate-catalogue-and-registry.md) is now
unblocked — all three of its blockers (05, 06, 07) are resolved.
