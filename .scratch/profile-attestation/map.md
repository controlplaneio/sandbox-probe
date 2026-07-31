# Profile attestation (+ repo split)

wayfinder:map

## Destination

Two coupled decisions, one map because the second motivates the first:

1. **Whether/how to split the comparison/orchestration layer out of
   `sandbox-probe`** into its own repository — `sandbox-probe` itself
   stays a minimal, standalone fingerprinting binary; `scan-matrix.yaml`,
   `seed-decoys.sh`, `run-probe-in-sandbox.sh` + the agent stub scripts,
   `site/`, and the whole baseline-normalized comparison methodology move
   to a new repo built *on* the probe, not inside it.
2. **A new "profile attestation" capability**: given a declared, versioned,
   externally-authored sandbox profile — starting with nono's registry
   packs (e.g. `nolabs-ai/codex`, formerly `always-further/codex`) — run
   `sandbox-probe` under that profile and diff the *empirically observed*
   reachable surface against the profile's *declared* grants. Surfaces two
   new finding classes the project doesn't have today: **declared-but-
   unreachable** (the profile overclaims) and **reachable-but-undeclared**
   (the profile has a real gap). This is the regression test nono profiles
   don't currently have.

## Why this map exists (how it was found)

Working [sandbox-canary-nesting](../sandbox-canary-nesting/map.md)'s
ticket 08, Chris pushed back hard on the premise: for the 5 generic
`sandbox`-family runtimes (`docker`/`podman`/`bwrap`/`nspawn`/`gvisor`,
no agent driving them), *any* mount/sharing flags the project's own script
adds are **our own choice**, not a vendor's — testing "did the sandbox
block what we chose to expose" is circular regardless of which flags get
picked. The comparisons that are actually meaningful are the ones where
*someone else* — an agent vendor (Claude Code choosing its own Seatbelt
config, confirmed in
[ticket 11](../sandbox-canary-nesting/issues/11-claude-sandbox-nesting.md)/
[12](../sandbox-canary-nesting/issues/12-codex-sandbox-nesting.md)) or a
declared policy — made the configuration decision, and we're observing it.

That reframing surfaced two things:
- The project has been conflating two different pieces of software: a
  fingerprinting probe (self-contained, minimal) and a comparison/
  benchmarking harness (methodology-heavy, everything that's been
  generating friction across both prior maps).
- **nono itself already has exactly the missing "someone else made a real
  decision" case** — its registry ships signed, schema-valid, versioned
  profiles per agent (e.g. `nolabs-ai/codex` declares specific
  filesystem/network grants for running Codex) — but confirmed via
  research (see Decisions so far) those profiles ship with **zero runtime
  behavioral verification**. Sigstore signing proves provenance; schema
  validation proves the JSON is well-formed; nothing proves the declared
  grants match what actually happens when the profile runs.
- Also found while researching: the project's *current* `nono` row in
  `scan-matrix.yaml` doesn't use a registry profile at all — it's ad-hoc
  CLI flags (`--allow-cwd --allow <dir> --block-net`) the project chose
  itself, the exact same circularity as the docker/podman/bwrap rows, even
  though nono has a legitimate declared-profile mechanism sitting right
  there unused. See [ticket 05](issues/05-nono-row-profile-switch.md).

## Notes

- Domain: same `CONTEXT.md` as the sibling maps, plus new vocabulary this
  map will need to define via `/domain-modeling` once terms stabilize —
  candidates: "declared profile", "attestation", "drift" (declared vs
  actual mismatch). Don't invent the final names without a domain-modeling
  pass once the mechanism ticket resolves.
- Standing preference: **plan, don't do** — same as both sibling maps.
- Primary sources so far (all fetched during charting, not yet a formal
  research ticket — cite these, don't re-derive): nono's official docs
  (`nono.sh/docs/cli/clients/codex`), DeepWiki's nono profile-format
  reference (`deepwiki.com/always-further/nono/3.3-profile-format-reference`),
  the registry page itself (`registry.nono.sh/packages/...` — client-rendered,
  fetched poorly, needs a research ticket to get real content, e.g. via
  its underlying API if one exists).
- Namespace note: nono's registry namespace moved from `always-further` to
  `nolabs-ai` at some point — the repo's own README still references
  `github.com/always-further/nono`; worth a quick check/update once this
  map's findings land, not urgent.
- Relationship to sibling maps:
  - [sandbox-canary-nesting](../sandbox-canary-nesting/map.md): its
    agent-driven tickets (11–14) stay valid untouched. Ticket 08's "fix
    the 5 generic runtimes' mount flags" is retired by this map's finding
    — see that map's own updated Notes.
  - [seed-ipc-targets](../seed-ipc-targets/map.md): stays relevant
    regardless of methodology — sockets/pipes/processes still need
    seeding. Its mechanism-design ticket should account for serving *both*
    baseline-vs-sandbox and declared-vs-actual comparisons once this map's
    shape is clearer.

## Decisions so far

- [Nono row profile switch](issues/05-nono-row-profile-switch.md) — two new rows, not a switch: (1) `sandbox-probe` itself run under `nono run --profile nolabs-ai/codex`, diffed against declared grants — the flagship attestation capability; (2) real `codex` CLI run under the same profile (`codex-nono`), alongside `codex-sandbox`, using existing methodology unchanged. Existing generic `nono` row stays (legitimate hand-authored-policy pattern, not circular). Confirmation was hedged ("i think so") — flagged for revisiting once there's something concrete to react to.
- [Repo split scope](issues/04-repo-split-scope.md) — **fully resolved**: reuse dormant `controlplaneio/sandbox-probe-reports`; `list-targets` stays with the probe, everything comparison-side moves; fresh start, no history migration; `go.mod`-pinned dependency for automatic Dependabot bumps (checkout-pinned `ref:` would NOT get coverage); split happens first, before sibling-map fixes land, once the release pipeline (ticket 06) is fixed; sibling maps' `.scratch/` dirs move wholesale, this map splits at ticket 05.
- [Release pipeline fixed](issues/06-fix-release-pipeline.md) — **resolved, verified, branch ready for review** (`fix/release-pipeline`, not pushed). Real root cause was more precise than first framed: not a missing cross-toolchain, but goreleaser itself forcing `CGO_ENABLED=0` for cross-compiled targets by default — fixed with a templated per-OS env, plus moving the release job to `macos-latest` (Linux still can't link Apple's private frameworks regardless). Reproduced the original failure locally, then reproduced full success end-to-end with real goreleaser. Release trigger switched to push-to-`main` + `lukaszraczylo/semver-generator`, matching `chrisns/MacWhisperAuto`.

**This map is now fully clear — every ticket (01–06) resolved.**

- **nono profiles have no runtime behavioral verification today** (established during charting, not yet a formal ticket — see Notes for sources): Sigstore signing = provenance only; schema validation = well-formedness only; nothing confirms declared grants match actual runtime behavior. This is the gap the map's destination fills.
- [nono audit command overlap](issues/03-nono-audit-command-overlap.md) — **confirmed empirically, gap is real**: `nono audit` is a forensic session log (what happened + tamper detection), not a policy-compliance checker. No command anywhere in nono computes declared-vs-observed drift. Both sides are already machine-readable (`profile show --json`, `audit show --json`/`audit-events.ndjson`) — nothing to reverse-engineer, but the diff logic itself doesn't exist and would need building.
- [Profile schema → probe findings mapping](issues/02-profile-schema-to-findings-mapping.md) — clean mappings exist for filesystem/network/sockets/process. **Surprising probe-side finding**: the probe already has dead code purpose-built for the `env_credentials` category (`models.EnvFinding`, `detectSensitiveEnvVars()`) that's never wired to a `finding_type` — a real, separate small bug worth fixing regardless of this map's outcome, since credentials are what nono profiles restrict most tightly. Mount/hostname/user-context findings have no nono equivalent by design (nono never swaps namespaces). Nono's own checked-in JSON Schema is itself stale (missing fields the real Rust struct has) — flagged, not silently worked around.
- [nono profile invocation](issues/01-nono-run-under-profile.md) — `nono run --profile <ns>/<name> -- <cmd>`, fetch-on-demand confirmed empirically (real Sigstore verification). Running the bare probe under a profile is sensible for the OS-level policy half of a pack, not the agent-specific plugin-wiring half. **Pack installation has real side effects even under `--dry-run`** (spliced into `~/.codex/config.toml` on this machine, fully reversed via `nono remove` and verified) — future empirical work needs explicit cleanup, not an assumption of dry-run safety.

**All 3 research tickets resolved.** Frontier is now the two grilling tickets (04, 05) — both waiting on Chris.

## Not yet specified

- Exact mapping from nono's `Profile`/`CapabilitySet` schema fields to
  `sandbox-probe`'s finding types — sharp enough to ticket once
  [ticket 02](issues/02-profile-schema-to-findings-mapping.md) reports back.
- What the new repo is actually called, where it lives, how CI/history
  migrates — not sharp until [ticket 04](issues/04-repo-split-scope.md)
  resolves.
- Whether other declarative profile systems (beyond nono) exist and are
  worth the same treatment — deferred until nono's own case is proven out.

## Out of scope

- Fixing the 5 generic runtimes' mount flags in
  `sandbox-canary-nesting` (docker/podman/bwrap/nspawn/gvisor with no
  agent, no declared profile) — ruled out by this map's core finding, not
  fog. See that map's ticket 08 for the formal closure.
