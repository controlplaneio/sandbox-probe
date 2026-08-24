# Research: contributed real Linux desktop inventory

For [ADR 0002](../adr/0002-seed-ipc-and-process-targets.md) and
[the (closed) seed-ipc-targets map](https://github.com/controlplaneio/sandbox-probe-reports/blob/main/.scratch/seed-ipc-targets/map.md).
A contributor sent a collection script + its own real output from their
daily-driver Linux desktop, as a second, independent data source for the
seed catalogue. Both reviewed before use.

## The contributed tooling

`scripts/collect-local-inventory.sh` (source reviewed in full before
running anything it produced): read-only enumeration only — `ss`/`lsof`/
`netstat`/`stat`, explicitly "special devices are never opened or read"
in its own header comment, matching what the code actually does. No
writes outside its own output file, no network calls, no privilege
escalation. Safe, and does what it claims.

It also proposes a **socket-owner-attribution methodology**
(`docs/socket-owner-attribution.md`) — an evidence ladder for how
confidently a *live scan* can attribute an observed socket to its true
owner:

1. Direct listener (high confidence) — `ss -p`/`lsof` reports PID +
   executable directly.
2. Service manager (medium) — resolve the PID's parent chain,
   `/proc/<pid>/cgroup`, `systemctl status`; a systemd socket unit can own
   the listener while activating something else.
3. Container controller (medium) — match cgroup paths to a runtime ID,
   report both the in-container PID and the host controller.
4. Inherited listener (low) — a child can inherit a socket from a
   parent/supervisor; preserve both PIDs rather than guessing one owner.
5. Unknown (explicit) — permissions, kernel sockets, races, namespaces,
   or unnamed sockets can all block attribution outright.

Proposed fixture fields: `endpoint`, `protocol`, `socket_kind`,
`observed_pid`, `observed_command`, `observed_user`, `parent_pid`,
`manager`, `container_context`, `attribution_level`, `attribution_reason`.

**Deliberately not adopted in ADR 0002.** This answers "who really owns
this socket during a real scan" — a live-detection capability question,
different in kind from "why is this entry in our seed catalogue," which
is what ADR 0002 actually needed. Preserved here in full rather than
discarded, since it's a genuinely well-thought-out methodology that would
be the right starting point if a future effort takes on confidence-graded
ownership attribution as its own piece of work.

## The contributed data

The contributor's own machine, real output
(`sandbox-probe-local-inventory-retest.txt`, 2241 lines): Linux
`7.0.9+deb13-amd64`, a genuine interactive desktop session — Xorg, dbus,
pipewire audio, ibus input, gnome-keyring — not a headless container, the
exact gap the container-based Linux research (this map's ticket 06)
explicitly flagged as unable to reproduce.

Unix-domain socket owners, by process, full list preserved (counts =
distinct sockets observed for that process):

```
439 brave              46 node-MainThread    9 xdg-dbus-proxy      4 nm-applet
 99 dbus-daemon         43 signal-desktop      9 wireplumber         4 ibus-portal
 77 cef_server          35 claude              8 synergy-service     4 chrome_crashpad
 73 Xorg                28 pipewire-pulse      7 synergy-tray        4 awesome
 58 zulip               24 pipewire             6 vicinae-server      4 at-spi2-registr
 54 spotify             18 1password            6 pasystray           3 xdg-document-po
 47 obsidian            17 ibus-daemon          6 "2.1.158"           3 mpris-proxy
                         17 copyq                5 kitty               3 ibus-x11
                         15 StreamControlle      5 ibus-ui-gtk3        3 ibus-dconf
                         15 dropbox              5 ibus-extension-     3 gvfsd-network
                         13 java                 5 goa-daemon          3 gvfsd-dnssd
                         12 xdg-desktop-por      5 gnome-keyring-d     3 gvfs-udisks2-vo
                         10 python               4 xscreensaver-sy     3 goa-identity-se
                         10 espanso              4 voxtype             2 (10 more, 1-2 each: ydotoold, xdg-permission-, ibus-engine-sim, gvfsd-recent, gvfsd-metadata, gvfs-mtp-volume, gvfs-gphoto2-vo, gvfs-goa-volume, gvfs-afc-volume, gdm-x-session, gcr-ssh-agent, flatpak-session, demo-console, dconf-service, xscreensaver, p11-kit-server, ntfy, kitten, gvfsd, gvfsd-trash, gvfsd-fuse, bun, bash, awk, at-spi-bus-laun)
                                                  4 uv
                                                  4 synergy-core
                                                  4 socat
                                                  4 rescuetime
                                                  4 opencode
```

## What this changes vs. the container-based research

- **Confirms** the container research's own findings (docker/containerd,
  ssh-agent, gpg-agent, dbus all present here too), independent
  corroboration from a completely different real machine.
- **Fills the explicitly-flagged gap**: browsers are real and prominent
  (`brave`, 439 sockets — the single largest source on the machine,
  consistent with browsers being chatty by nature) — the container
  research's exclusion of browser sockets as "reasoned by macOS analogy
  only" is superseded for Linux now.
- **New categories not previously considered at all**: a password manager
  (`1password`, 18 sockets) — arguably a higher-value decoy target than
  most of what's in the catalogue, given what password managers guard.
  Chat clients (`zulip`, `signal-desktop`). `claude` (35 sockets) — see
  ADR 0002's corrected reasoning on why this is now included, scoped to a
  sibling-session decoy, not the running session's own socket.
  `opencode` (4 sockets) — another AI coding agent already a supported
  harness in this project, same "concurrent agent instances" reasoning
  potentially applies, not pursued further here.
- **Still not confirmed**: `ssh-askpass` specifically — transient,
  per-auth-prompt, and a static inventory snapshot (this one or the
  container research) wouldn't catch one mid-prompt either way. Remains
  reasoned-by-analogy only.

## Consequence

Two catalogue entries added to ADR 0002 on this basis (browser
confirmation, 1Password addition) — deliberately not the full list above,
to keep the catalogue change focused rather than sprawling. The rest of
this data is preserved here for whoever picks up the next round of
catalogue expansion — matches the "open invitation for contributions"
framing this was given from the start.
