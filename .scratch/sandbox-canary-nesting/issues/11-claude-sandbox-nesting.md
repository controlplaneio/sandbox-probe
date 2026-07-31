Type: research
Status: resolved

## Question

`scripts/run-probe-via-claude-stub.sh` runs the real `claude` binary
(model stubbed) with `CLAUDE_SANDBOX=on`, which makes Claude Code apply its
own sandbox (bubblewrap on Linux, Seatbelt on macOS) directly to the shell
subprocess that runs the probe — no `docker run`/fresh rootfs construction
by our script. This looks structurally like the already-confirmed-fine
`srt`/`firejail`/`nono` cases (same filesystem, policy-restricted) rather
than the broken 5-runtime cases — but that's an assumption from a quick
peek, not confirmed.

Investigate empirically: run `seed-decoys.sh` on this Mac (or a disposable
Linux VM for the bwrap path), then run the claude-stub script with
`CLAUDE_SANDBOX=on`, and check whether the probe (running inside Claude
Code's own sandbox) can see the seeded decoys or not. Compare against
`CLAUDE_SANDBOX=off` (the paired baseline) run the same way. If Claude
Code's own bwrap/Seatbelt invocation reconstructs a disconnected
environment the way the original 5 runtimes did, that's a real finding,
not just a formality — don't assume the answer.

If it's already correctly nested (most likely outcome, matching srt/firejail/
nono), this ticket resolves as "no fix needed" — say so plainly rather than
manufacturing a change.

## Answer

**Already correctly nested — no fix needed.** Full write-up:
`docs/research/claude-sandbox-nesting.md` on branch
`research/claude-sandbox-nesting` (commit `49aaa40`).

Empirically verified on macOS (Seatbelt): seeded 17 real decoys under
`$HOME` via `seed-decoys.sh`, ran the claude stub with `CLAUDE_SANDBOX=off`
and `on`, diffed the reports. `sensitive_readable_paths` was byte-for-byte
identical between both runs (all 33 entries, every decoy included) —
proving the sandboxed run is the *same process tree on the same
filesystem*, not a fresh container. The sandbox was genuinely active, not
a no-op: network connectivity dropped from `["google.com"]` to `[]`, and
`sandbox_detection` correctly fired `"seatbelt"` only when sandboxed. Reads
aren't restricted because that's Seatbelt's own default policy under
Claude Code's config (read-allow-everywhere, write/network-deny-by-default
— matches Anthropic's `srt` shape), not a nesting bug. Confirms the
ticket's hypothesis: structurally the same as `srt`/`firejail`/`nono`.

Bubblewrap (Linux path) not tested — this machine is macOS. A real check
needs a Linux box/VM with `bwrap`; note for
[ticket 12](12-codex-sandbox-nesting.md) and consolidation, since Codex
shares the same bwrap/Seatbelt split.
