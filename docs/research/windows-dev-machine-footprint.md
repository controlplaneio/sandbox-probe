# Research: Windows developer-machine named-pipe/process footprint

For [wayfinder ticket 07](../../.scratch/seed-ipc-targets/issues/07-windows-dev-machine-footprint-capture.md).
Captured against the `Win11.utm` VM (Windows 11 24H2, build 26100.4875) via
`utmctl exec`/`utmctl file` — the QEMU guest agent, once installed inside
the guest, turned this from a fully-manual task into something drivable
directly. Method and its limits documented below since they matter as much
as the data.

## Method

`utmctl exec Win11 --cmd cmd.exe /c "powershell.exe -ExecutionPolicy Bypass -File C:\<script>.ps1"`,
with the script redirecting its own output to a file via `Out-File`, then
`utmctl file pull` to retrieve it. Two real gotchas hit along the way,
worth recording so a future session doesn't rediscover them the hard way:

- `utmctl exec` reports only the launched process's *start* succeeding
  (exit code), not the full lifetime of what it spawns — pulling a result
  file immediately after can race a still-writing process (`The process
  cannot access the file because it is being used by another process`). A
  few seconds' wait before pulling is necessary.
- `Invoke-WebRequest`'s default progress-bar rendering makes downloads
  dramatically slower with no interactive console attached (a well-known
  PowerShell quirk) — a ~100 MB download took several minutes. Not fixed
  here (`$ProgressPreference = 'SilentlyContinue'` is the known
  workaround) since it didn't block the actual finding, but worth setting
  proactively next time.

**Execution context caveat, same shape as the sibling Linux research**:
everything above runs as `nt authority\system` via the QEMU guest agent,
not an interactive logged-in user session. This matters for two reasons
found empirically, not assumed:

1. `whoami`/process ownership reflects SYSTEM, not a real user — some
   per-user services (anything that only starts on interactive logon)
   won't appear in this footprint at all.
2. **SYSTEM processes launched this way run in Windows Session 0** (the
   isolated services session, walled off from the interactive desktop
   since Windows Vista specifically so services can never show or drive
   GUI dialogs). Confirmed directly: after triggering a VS Code silent
   install, the installer process was alive and consuming a trickle of CPU
   but **no running process had any `MainWindowTitle` at all** — meaning
   any dialog it might have raised was rendered on a desktop nobody, not
   even a user physically looking at the VM's screen, could ever see or
   dismiss. This is a **hard wall, not a flaky timeout**: GUI-touching
   installers cannot be driven to completion via `utmctl exec` as currently
   used, regardless of `/VERYSILENT`-style flags, because the isolation is
   at the session level, above where any installer-silence flag operates.

## Baseline footprint (before installing anything)

**Named pipes** — almost entirely generic Windows system plumbing:
`lsass`, `ntsvcs`, `eventlog`, `epmapper`, `wkssvc`, `srvsvc`, `atsvc`,
`InitShutdown`, several `Winsock2\CatalogChangeListener-*` — plus
already-running-by-default app pipes: Microsoft Edge's `mojo.*`
IPC pipes (multiple `msedgewebview2`/`msedge` processes ship active by
default), OneDrive, PowerShell's own `PSHost.*` pipe, and two SPICE/UTM
guest-integration pipes (`spice-webdavd`, and `qemu-ga` itself as a running
process) — the latter two are artifacts of the VM platform, not anything a
real bare-metal Windows dev machine would have.

**Processes** — standard Windows service/svchost sprawl (~140 processes),
Edge + its `msedgewebview2` helper swarm, OneDrive, Windows Defender
(`MsMpEng`, `NisSrv`, `MpDefenderCoreService`), no development tooling
present in a clean image, as expected.

## OpenSSH: already installed, no seeding needed for the client itself

Unlike Linux/macOS (where `ssh-agent` needs installing or is
tool-dependent), **Windows 11 ships OpenSSH Client pre-installed by
default** — confirmed via `Get-Command ssh` (`C:\WINDOWS\System32\OpenSSH\ssh.exe`,
version 9.5.5.1) and `Get-WindowsCapability -Online -Name OpenSSH.Client*`
(`State: Installed`). The `ssh-agent` *service* ships present but
`Stopped` by default — starting it is a one-line toggle
(`Start-Service ssh-agent`), not an install:

```
Before: Status=Stopped, Name=ssh-agent, DisplayName="OpenSSH Authentication Agent"
After:  Status=Running, Name=ssh-agent, DisplayName="OpenSSH Authentication Agent"
```

Resulting pipe, confirmed via directory diff: **`\\.\pipe\openssh-ssh-agent`**
— the direct Windows equivalent of macOS/Linux's `SSH_AUTH_SOCK` socket.
Cheapest, cleanest catalogue entry this research found: no install, no
download, just a service start.

## Docker Desktop for Windows: not captured, blocked by the Session 0 wall

Not attempted after the VS Code install demonstrated the Session 0
limitation firsthand — Docker Desktop's own first-run flow is
*more* GUI/interaction-dependent than a plain installer (WSL2 backend
provisioning, a licence-acceptance dialog, typically a reboot), so it
would hit the same wall, likely worse. **Not captured empirically in this
session** — but the resulting pipe path doesn't need to be: Docker Desktop
for Windows's engine endpoint is already documented fact from earlier
research this session (see the original branding/design-work thread) —
**`\\.\pipe\docker_engine`**, the direct equivalent of `/var/run/docker.sock`.
Worth listing in the eventual catalogue on that basis, flagged as
documented-not-empirically-confirmed-here rather than silently presented
as equally solid to the ssh-agent finding.

## VS Code: install path blocked, but the target IPC mechanism is known

The actual install got stuck (see Session 0 finding above) and was killed
cleanly, no trace left on the guest. VS Code's own IPC mechanism on
Windows is documented upstream as a named pipe under
`\\.\pipe\vscode-ipc-<hash>-sock` (or similar, version-dependent) once a
real editor window or a Remote-SSH/extension-host connection is active —
not independently confirmed here since the install itself couldn't
complete, so this is explicitly a documentation-only lead, not a verified
finding, unlike the ssh-agent result above.

## What would unblock GUI-dependent installs

Not attempted in this session (scope/time), but the concrete options if a
future session wants Docker Desktop / VS Code footprints for real:

1. **A one-time manual install** done by Chris directly in the UTM app's
   own VM window (the actual interactive Session 1, not the automation
   path) — after that, `utmctl exec` can drive everything else
   (starting/stopping the tool, capturing before/after pipe diffs)
   perfectly well, since the *tool* would then be running normally, just
   not *installed* by remote automation.
2. **Windows Task Scheduler as an interactive-session bridge** — `schtasks`
   can register a task to run in the logged-in user's session rather than
   Session 0, which is a known technique for driving GUI processes from a
   non-interactive caller. Untested here.

## Consequence for the catalogue ticket

[Ticket 09](../../.scratch/seed-ipc-targets/issues/09-populate-catalogue-and-registry.md)
gets one fully-empirical, high-confidence Windows entry (`ssh-agent` →
`\\.\pipe\openssh-ssh-agent`) and two documented-but-unverified leads
(Docker Desktop, VS Code) that should be labeled with that distinction in
whatever the catalogue ends up being, not presented as equally solid.
