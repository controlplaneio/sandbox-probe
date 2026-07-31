Type: grilling
Status: resolved

## Question

The project's current `nono` row in `scan-matrix.yaml` uses ad-hoc CLI
flags (`--allow-cwd --allow <dir> --block-net`) chosen by the project
itself — not a registry profile. Now that nono's registry has real,
declared, agent-specific profiles (e.g. `nolabs-ai/codex`), decide:

1. Should the existing `nono` row switch to using a named registry profile
   instead of ad-hoc flags? Which profile — a generic one, or does this
   only make sense per-agent (i.e. retire the generic `nono` row and add
   agent-specific ones)?
2. Should there be a **new** comparison row: an agent (starting with
   Codex, since that's the profile already researched) run *under nono's
   community profile* for it, alongside the agent's own native sandbox row
   (`codex-sandbox`, confirmed fine in the sibling map) — i.e. "codex via
   its own Seatbelt/bwrap" vs "codex via nono's declared profile" as two
   directly comparable rows?
3. If so, this becomes the flagship case for the map's attestation
   capability — the same run would both (a) compare codex-native-sandbox
   vs codex-via-nono as two sandboxing choices, and (b) attest whether
   codex-via-nono's actual behavior matches what the `nolabs-ai/codex`
   profile declares. Confirm this framing is right before it becomes the
   destination's worked example.

Depends on [ticket 01](01-nono-run-under-profile.md) for the exact
invocation mechanics.

## Answer

Resolves as two separate, complementary rows rather than one switch —
Chris's confirmation was a hedged "i think so," recorded as agreed but
flagging here explicitly in case it needs revisiting once there's an
actual implementation to react to (a `/prototype` session would be the
natural place to firm this up further).

1. **Existing generic `nono` row (no agent) stays as-is**, minus the small
   flag fix already agreed in
   [sandbox-canary-nesting](../sandbox-canary-nesting/issues/06-firejail-nono-srt-flag-audit.md).
   Not circular the way bare `docker run` is — nono is designed to be
   configured with a hand-authored inline policy, so testing that is a
   legitimate deployment pattern on its own terms. No generic "just nono"
   registry profile exists to switch it to anyway (profiles are
   agent-specific by design, per [ticket 01](01-nono-run-under-profile.md)).
2. **New row 1 — pure attestation**: run `sandbox-probe` itself under
   `nono run --profile nolabs-ai/codex -- ./sandbox-probe scan`, diff its
   findings against the profile's declared grants (via
   [ticket 02](02-profile-schema-to-findings-mapping.md)'s schema
   mapping). No real agent needed — tests the OS-level policy half of the
   pack only, per ticket 01's limit. **This is the flagship new
   capability** — the actual regression test nono profiles don't have
   today.
3. **New row 2 — real sandboxing-choice comparison**: run the real
   `codex` CLI under the same profile (`codex-nono`), alongside the
   existing `codex-sandbox` row (native Seatbelt/bwrap). Uses the
   project's existing baseline-normalized methodology unchanged — just
   another harness row, now legitimate per this map's own criteria (a
   real, externally-authored policy decision).

[Ticket 06 (fix release pipeline)](06-fix-release-pipeline.md) is now the
only open item on this map.
