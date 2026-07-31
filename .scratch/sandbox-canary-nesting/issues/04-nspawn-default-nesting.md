Type: research
Status: resolved

## Question

`systemd-nspawn` currently runs against a `ROOTFS` the workflow builds
fresh from a `docker export` of `ubuntu:latest` — never the seeded parent.
Research: does `systemd-nspawn` have a mode that boots/nests against the
**host's own root** (or a bind of it) rather than requiring a separate
prepared rootfs tree — e.g. `systemd-nspawn -D /` or an
`--overlay=`/bind-based invocation — while still fingerprinting correctly
(`container=systemd-nspawn`, the thing `expect: '["nspawn"]'` checks for)?
If nspawn structurally requires a separate rootfs directory (can't nest
directly on `/`), work out the closest equivalent: bind-mounting the
seeded parent's relevant paths (e.g. `$HOME`) into that rootfs at nspawn
launch time, via nspawn's own `--bind=` flag, as its actual documented
mechanism for sharing host paths in — not a workaround, but nspawn's real
intended way of doing this.

Needs root (`sudo`) — verify empirically using Docker Desktop's
`desktop-linux` VM if privileged enough, otherwise note what's blocking
and what a real test would need.

## Answer

Full write-up: `docs/research/nspawn-default-nesting.md` on branch
`research/nspawn-default-nesting` (commit `65a95dc`).

1. **`systemd-nspawn -D /` is a hard, compiled-in refusal**, not just
   discouraged — confirmed empirically (`Spawning container on root
   directory is not supported...`). nspawn does a full pivot_root-style
   takeover of the given tree, not a chroot overlay, so it structurally
   requires a separate prepared rootfs directory. Matches the man page.
2. **`--bind=`/`--bind-ro=` is vendor-intended**, not a workaround — the
   man page documents no other way to expose a host path inside the
   container's namespace, and its stated purpose is exactly this. The
   script already uses it for report output; extending it to bind in the
   seeded parent's `$HOME` uses the mechanism as documented.
3. **Empirically verified**: built the same `docker export`-based rootfs
   the workflow already builds, placed a decoy outside it — unreachable by
   default, reachable once shared via `--bind=`. The `container=
   systemd-nspawn` env var the probe already checks as a fingerprint
   fallback confirmed correctly set. (One test-environment wrinkle: had to
   run nested inside a privileged container inside Docker Desktop's VM,
   which lacks a host systemd, so `/run/systemd/container` didn't populate
   and `--register=no` was needed — a property of the double-nesting in
   this test setup, not of nspawn on a real Linux CI runner; the probe
   code already treats both signals as equivalent, so this doesn't affect
   the real fix.)

So unlike bwrap (still pending — genuinely no vendor default to defer to),
nspawn has both a clear "can't nest on host root at all" answer AND a
clear, already-partially-used, vendor-documented sharing mechanism — this
one's straightforward for [ticket 08](08-consolidate-nesting-design.md).
