Type: task
Status: resolved

Blocked by: 02, 03, 05, 08, 09

## Question

Consolidate every resolved ticket on this map into the actual destination
artifact: a new `docs/adr/000X-...md` (matching the rigor/style of
`docs/adr/0001-client-side-site-over-data-branch.md`) covering the seeding
mechanism, the safety guarantee and how it was verified, the finding-type
decision, the namespace-parity semantics, and the populated catalogue —
plus updating the deferred bullet in `docs/reporting-site-plan.md`'s
"Track 2 — seeder" section to point at the new ADR instead of restating it.

## Answer

Written: `docs/adr/0002-seed-ipc-and-process-targets.md`, consolidating
every resolved ticket on this map (mechanism, finding-type naming,
catalogue, safety findings, cross-references to the sibling maps this
effort spawned or depends on). `docs/reporting-site-plan.md`'s deferred
"network/socket decoys" bullet updated to point at the ADR instead of
restating it. Both currently uncommitted in the working tree, alongside
this session's `CONTEXT.md` edit from ticket 05 — not pushed/PR'd without
Chris's explicit go-ahead, matching how the release-pipeline and
windows-binaries fixes were handled.

**This map is now fully clear — every ticket (01–10) resolved.** Ready to
hand off to `/to-spec`, which collapses the linked decisions into a
buildable plan for the actual implementation (the `kind` field, per-kind
seeding logic, the Windows named-pipe detection task) — none of which has
been built yet; this map decided the design, not the code.
