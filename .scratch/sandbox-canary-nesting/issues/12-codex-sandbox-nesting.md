Type: research
Status: resolved

## Question

Same question as [the claude-sandbox ticket](11-claude-sandbox-nesting.md),
for Codex: `scripts/run-probe-via-codex-stub.sh` runs the real `codex` CLI
with `CODEX_SANDBOX=on` → `--sandbox workspace-write`, which applies
Codex's own sandbox (Seatbelt on macOS, bubblewrap+seccomp on Linux)
directly to the shell subprocess — no container/rootfs construction by our
script. Investigate empirically the same way: seed decoys, run with
`CODEX_SANDBOX=on` vs `off`, check whether the seeded parent's decoys are
reachable from inside Codex's own sandbox.

Note Codex's sandbox mode is specifically `workspace-write` (not a generic
deny-all) — worth checking whether that mode's own semantics (write
restricted to workspace, read presumably broader) changes what "reachable"
even means here compared to Claude Code's mode, rather than assuming
identical behavior just because the underlying tech (bwrap/Seatbelt) is
the same.

If already correctly nested, resolve as "no fix needed" — don't
manufacture a change.

## Answer

**No fix needed.** Full write-up: `docs/research/codex-sandbox-nesting.md`
on branch `research/codex-sandbox-nesting` (commit `217feb7`).

Empirically verified on macOS (Seatbelt, codex-cli 0.142.5): all 33
`sensitive_readable_paths` findings identical byte-for-byte between
`CODEX_SANDBOX=on`/`off` — `workspace-write` doesn't restrict reads, by
design. Writes genuinely confined: a `$HOME` write blocked under
`sandbox=on` (`Operation not permitted`), succeeded under `off`; `/tmp`
succeeded both ways, matching Codex's own `workspace-write [workdir, /tmp,
$TMPDIR]` policy banner. Fingerprint (`"seatbelt"`) fires correctly only
when sandboxed, cross-checked against macOS's real `sandbox_check()` API,
not just Codex's own claim. Confirms the same `srt`/`firejail`/`nono`
shape as [claude-sandbox](11-claude-sandbox-nesting.md) — policy-restricted
on the same filesystem, not rootfs-swapped.

Bubblewrap/Linux path not tested (macOS-only machine) — same gap as
ticket 11, inferred to behave the same by mechanism but unconfirmed.

Two of two agent-CLI-native sandboxes (claude, codex) now confirmed
already-correct — the pattern looks solid. Remaining uncertainty is
specifically the two docker-re-exec cases (13, 14), which are a different
mechanism entirely.
