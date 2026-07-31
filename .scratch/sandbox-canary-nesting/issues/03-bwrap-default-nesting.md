Type: research
Status: resolved

## Question

`bwrap` is different in kind from Docker/Podman: it has **no default
invocation at all** — every namespace/bind decision is an explicit flag on
the command line, there's no vendor "default" the way a container runtime
has one. The current script (`scripts/run-probe-in-sandbox.sh`) builds a
minimal root from `--ro-bind /usr /usr --ro-bind /bin /bin ... --bind
"$PWD" /work`, omitting `/home` entirely.

So "vendor default" doesn't apply the same way here. Research: what's the
closest bwrap analogue to "genuinely nested in the seeded parent" —
options to investigate:
1. `--bind / /` (bind the entire host root, not a reconstructed minimal
   one) plus whatever `--unshare-*` flags the project already uses
   (`--unshare-user --unshare-ipc --unshare-uts --unshare-cgroup`) — does
   this still fingerprint as bubblewrap (the thing this row exists to
   prove, per `expect: '["bubblewrap", ...]'` in `scan-matrix.yaml`) while
   actually nesting in the seeded parent?
2. Whether bwrap has any documented "typical"/recommended minimal
   invocation (from its own docs/examples) that the project should treat as
   the closest thing to a default, if full root-bind changes the
   fingerprint or feels like it's testing something else.

This one may resolve differently from Docker/Podman/nspawn/gvisor precisely
because there's no vendor default to defer to — flag that explicitly in the
answer rather than forcing a false parallel.

Verify empirically using Docker Desktop's `desktop-linux` VM (bwrap is
Linux-only, installable there).

## Answer

Full write-up: `docs/research/bwrap-default-nesting.md` on branch
`research/bwrap-default-nesting` (commit `f0d9bc4`).

1. **`--bind / /` fingerprints correctly and genuinely nests.** The
   probe's bwrap detection (`GetBubbleWrap()`, `pkg/tasks/baseline/environment.go`)
   is a pure ancestor-process-name walk over `/proc` — filesystem-independent,
   so invariant to bind strategy as long as `bwrap` stays a visible parent
   (neither invocation uses `--unshare-pid`). Confirmed `sandbox_detection:
   "bubblewrap"` under both the current script and `--bind / /`. A canary
   at `~/.ssh/id_rsa_canary` was unreachable under the current script,
   reachable — and round-trip-writable, proving genuine nesting — under
   `--bind / /`.
2. **No formal vendor default exists** — confirmed from bwrap's own README:
   *"bubblewrap is not a complete, ready-made sandbox with a specific
   security policy."* This one genuinely doesn't parallel the
   container-runtime tickets, as flagged when the ticket was written.
   bwrap's own illustrative README example is itself a minimal,
   mostly-read-only reconstruction — closer to the current script than to
   full-root-bind.
3. **Full-root-bind has a real gap, not just cosmetic**: `--bind / /` is
   read-write for the entire host — a write to `/etc` fails only with
   ordinary `Permission denied` (Unix DAC, caller-dependent), while the
   minimal reconstruction's `--ro-bind /etc /etc` fails with `Read-only
   file system` (namespace-level enforcement, independent of caller
   permissions). A caller with broad host write access keeps that access
   under full-bind.

**Recommendation carried into [ticket 08](08-consolidate-nesting-design.md)**:
not full `--bind / /` — use `--ro-bind / /` plus one targeted writable
bind (the probe's own workdir), verified to preserve genuine nesting
without the DAC-vs-namespace gap. Closest bwrap gets to "vendor default"
here: least-privilege-by-construction nesting, not maximum sharing.
