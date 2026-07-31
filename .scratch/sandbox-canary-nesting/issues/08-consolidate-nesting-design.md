Type: grilling
Status: resolved

Blocked by: 01, 02, 03, 04, 05, 06, 11, 12, 13, 14

## Question

With per-runtime defaults and non-invasive plumbing mechanisms researched
for docker/podman/bwrap/nspawn/gvisor, and the firejail/nono/srt flags
audited, decide the concrete implementation approach per runtime: the
exact invocation change to `scripts/run-probe-in-sandbox.sh` (and any
sibling scripts) that nests each sandbox in the seeded parent using only
its real default posture, plus how probe-binary-in/report-out plumbing
works for each without altering that posture.

Also decide: how does this interact with
[seed-ipc-targets](../seed-ipc-targets/map.md)'s future socket/pipe/process
seeding — once a runtime nests correctly, do seeded sockets on the parent
become automatically reachable (if the runtime shares the relevant
directories) or does each still need explicit handling? Cross-reference
that map's [mechanism-design ticket](../seed-ipc-targets/issues/02-safe-seeding-mechanism-design.md).

## Answer

**Retired, not designed — the premise was wrong.** Started grilling this
ticket and Chris caught a fundamental circularity: for the 5 generic
runtimes (docker/podman/bwrap/nspawn/gvisor, no agent driving them), *any*
mount/sharing flags this project's own script adds are our own choice, not
a vendor's — "did the sandbox block what we chose to expose" is meaningless
regardless of which flags get picked. Testing them this way was never
going to produce a real security comparison, no matter how the mount
question got resolved.

**Resolution, split three ways:**
1. **docker/podman/bwrap/nspawn/gvisor (no agent, no declared profile)**:
   no script fix. These don't belong in the baseline-normalized comparison
   matrix without *something* making a real configuration decision — see
   the new [profile-attestation map](../profile-attestation/map.md),
   spawned directly from this realization. The per-runtime research
   (tickets 01–05) isn't wasted — it's the evidence base for *why* these
   are excluded, not guesswork.
2. **firejail/nono**: still gets the small fix identified in
   [ticket 06](06-firejail-nono-srt-flag-audit.md) — drop `--net=none`/
   `--block-net`, non-default restrictions on tools that otherwise nest
   correctly. Independent of the circularity problem (these don't
   reconstruct an environment, so there's no vendor-decision gap to fill).
3. **claude-sandbox/codex-sandbox/gemini-docker/trae-docker (11–14)**:
   unaffected — real vendor/agent decisions either way, already confirmed
   fine.

[Ticket 09](09-prototype-verify-nesting.md) and
[ticket 10](10-write-final-adr.md) should be re-scoped down to just the
firejail/nono flag fix (small) plus documenting the retirement reasoning
for the 5 generic runtimes — hold both until
[the sibling map's ticket 05](../profile-attestation/issues/05-nono-row-profile-switch.md)
decides whether nono's own row changes shape, since that affects what the
final ADR actually describes.
