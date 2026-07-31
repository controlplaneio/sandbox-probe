Type: research
Status: resolved

## Question

`scripts/run-probe-via-gemini-stub.sh` with `GEMINI_SANDBOX=docker`
re-execs the **entire gemini-cli** inside a container via gemini-cli's own
`--sandbox` flag (not our script constructing the container directly, the
way the original `docker`/`podman` runtime tickets found — this is the
*vendor's* docker invocation). The script's own comment says this "mounts
the workspace." Our stub already injects extra docker flags into that
invocation via a `SANDBOX_FLAGS` env var (currently used only for
mock-reachability: `--add-host host.docker.internal:host-gateway
--entrypoint "" -e ...`), which is a promising existing extension point if
more sharing turns out to be needed.

Investigate:
1. Exactly what does gemini-cli's own `--sandbox docker` mode mount by
   default — read gemini-cli's own source/docs (this is Google's
   `@google/gemini-cli`, check its actual sandbox implementation) rather
   than inferring from the comment alone. Does "the workspace" mean the
   project directory only, or does it also touch `$HOME`?
2. Empirically: seed decoys in `$HOME`, run with `GEMINI_SANDBOX=docker`,
   check whether they're reachable from inside gemini-cli's own container.
3. If not reachable and that's *not* what a typical/default user of
   gemini-cli's docker sandbox would get either (i.e. this genuinely is
   gemini's vendor default, just narrower than `$HOME`), that's a
   legitimate, real "blocked" result — don't force sharing in artificially
   to manufacture a comparison gemini-cli's own users wouldn't have. Only
   if `SANDBOX_FLAGS` can extend the mount *without changing gemini-cli's
   own default behavior for anyone else using it normally* does adding a
   `$HOME` share via that mechanism make sense — reason through this
   distinction explicitly, don't just add a mount because the previous
   five tickets did.

This one may resolve differently in kind from the original five — say so
plainly if gemini's own default already IS a meaningful, vendor-shipped
boundary rather than an accidental disconnection.

## Answer

**No fix needed — different in kind from tickets 01–06.** Full write-up:
`docs/research/gemini-docker-nesting.md` on branch
`research/gemini-docker-nesting` (commit `243c8ca`).

Read gemini-cli's own sandbox source directly (installed `@google/gemini-cli`
0.49.0, cross-checked byte-for-byte against the `v0.49.0` GitHub tag): the
docker sandbox genuinely nests in the seeded parent — the workspace is
bind-mounted at its literal host path (no fresh rootfs), plus a small named
allowlist (`$HOME/.gemini`, temp dir, conditionally `~/.config/gcloud` and
a `$GOOGLE_APPLICATION_CREDENTIALS` file, both read-only). **Full `$HOME`
is not shared by default.** The probe's seedable decoy targets
(`~/.ssh/*`, `~/.aws/*`, `~/.npmrc`, etc.) fall outside that allowlist
entirely — an "unreachable" result there is a real vendor-default
boundary, not a disconnected-harness artifact. `SANDBOX_FLAGS` should stay
mock-plumbing-only; extending it to add a `$HOME` mount would test a
boundary no normal gemini-cli user has, exactly the trap
[ticket 13](#) was written to watch for.

Side finding: the vendor's own bundled `sandbox.md` docs undersell what's
actually shared (says "workspace only," omits `.gemini`/tmp/gcloud) — a
gemini-cli documentation gap, not this project's problem to fix.

Live empirical confirmation was blocked by a macOS-only quirk (gemini-cli
picked Seatbelt over an explicitly-requested Docker sandbox 4/4 attempts,
root cause not pinned down) — but `scan-matrix.yaml` runs this row on
`ubuntu-latest` where Seatbelt doesn't exist, so very likely irrelevant to
the real pipeline; documented as a flag, not chased further given the
source-level answer is already solid.

**All four agent-harness tickets (11–14) now resolved: three "no fix
needed" for different, specific reasons (claude/codex: same process tree;
trae: real container, vendor's own minimal mount, no widen point; gemini:
real container, vendor's own broader-but-still-bounded allowlist). Zero
new script changes needed for any of the four** —
[ticket 08](08-consolidate-nesting-design.md) is now unblocked on
everything (01–06, 11–14 all resolved) and scoped entirely to the original
5 generic runtimes.
