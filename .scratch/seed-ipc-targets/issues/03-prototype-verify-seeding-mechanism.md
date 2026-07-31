Type: prototype
Status: resolved

Blocked by: 02

## Question

Build a minimal, throwaway prototype of the mechanism chosen in
[the mechanism design ticket](02-safe-seeding-mechanism-design.md), and
verify by hand that it:

1. Never touches anything real — run it first inside Docker Desktop's
   `desktop-linux` VM (disposable, safe to trash), confirm zero effect on
   the host Mac.
2. Actually produces a finding — run `sandbox-probe scan` before and after
   seeding, confirm the relevant `finding_type` (`unix_socket_detection`,
   `process_detection`) appears only after seeding.
3. Cleans up completely — after the scan, confirm no seeded artifact
   remains (no orphan socket file, no orphan process), including after a
   simulated crash (kill -9 mid-seed) if the mechanism has any window where
   that matters.
4. Only *then* — carefully, on the real Mac, in a scratch location — repeat
   the same verification as final confirmation the guarantee holds outside
   a disposable VM too.

This is a HITL prototype session (react to the concrete artifact together),
not a solo agent build.

## Answer

**Mechanism from [ticket 02](02-safe-seeding-mechanism-design.md) verified
end-to-end, both in Docker's disposable Linux VM and on the real Mac.**

Socket: `bind()` + `close()` at a scratch path — before seeding, 0 matches
in `unix_socket_detection`; after, the probe's real scan finds it; after
cleanup (`rm`), gone. Process: spawned via `exec -a seed-mech-canary sleep
120` (argv0 renamed so it's identifiable/verifiable, self-timeout as the
"belt") — before seeding, absent from `process_detection`; after, found;
explicit `kill` (the "suspenders") confirmed via direct `ps -p <exact
pid>` to leave zero trace, both in the container and on the real Mac.

**Real finding, not just confirmation**: macOS enforces the standard
`AF_UNIX` `sun_path` length limit (~104 bytes) — binding a socket under
this session's deeply-nested scratchpad path failed with `OSError: AF_UNIX
path too long`. Retried successfully with a short path (`/tmp/spmt-<pid>`).
**This is a real constraint on the eventual mechanism**, not a testing
artifact — conventional real-world paths (`~/.docker/run/docker.sock`,
`/run/user/<uid>/gnupg/S.gpg-agent`) are short enough to stay under the
limit, but the implementation needs to defensively handle or at least
surface a clear error if a future catalogue entry's path is too long,
rather than failing silently or cryptically. Worth a line in
[ticket 09](09-populate-catalogue-and-registry.md)'s eventual write-up.

One test-harness false alarm, resolved: an ad-hoc `grep` count on raw
`process_detection` JSON showed non-zero "before" matches due to
quote-variant duplication in the query, not an actual leftover process —
confirmed via direct `ps -p <exact pid>` (the unambiguous check) that
nothing from either the failed long-path attempt or the successful
short-path run was still running afterward.

**This map's mechanism design + verification is now fully done.** Only
[ticket 07](07-windows-dev-machine-footprint-capture.md) (Windows VM,
hands-on) blocks [ticket 09](09-populate-catalogue-and-registry.md) from
starting; [ticket 10](10-write-final-adr.md) (the ADR) is next after that.
