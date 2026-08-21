Type: research
Status: resolved

## Question

How does one actually launch a process under a **named registry profile**
with nono (as opposed to the ad-hoc CLI flags the project's current `nono`
row in `scan-matrix.yaml` uses)? Read nono's official CLI docs (`nono.sh/docs`)
and its `nono run` command reference specifically.

1. Exact invocation shape: `nono run --profile <name>` or similar? How does
   a profile get resolved from the registry (e.g. `nolabs-ai/codex`) versus
   a locally-defined one?
2. Does invoking a named profile require the profile to already be
   installed locally, or does `nono run` fetch-on-demand (the docs
   mentioned automatic installation with Sigstore verification when a
   profile isn't present)?
3. Concretely: what would the command look like to run `sandbox-probe`
   itself (not Codex) under the `nolabs-ai/codex` profile, to see what
   *the probe* can reach under that profile's declared grants? Is that
   even a sensible thing to do (the profile is written *for* Codex
   specifically — check whether profiles are agent-agnostic sandboxes
   applicable to any binary, or whether they bundle agent-specific
   plugin wiring that only makes sense with the real Codex binary
   present).
4. Is there a `nono audit` command or similar (mentioned in passing in
   DeepWiki's docs but not detailed) — if so, what does it actually check?
   Does it already do some or all of what this map is trying to build, or
   is it schema/structural only (matching the pattern found in
   [the profile-format research](../map.md#decisions-so-far))?

Deliverable: write findings to `docs/research/nono-profile-invocation.md`
(repo has an established `docs/research/` convention). Cite nono's actual
docs pages, not secondary summaries. Commit on branch
`research/nono-profile-invocation` (from current HEAD). Final response:
concise summary (under 300 words) with direct answers to the 4 points,
file path + branch/commit.

## Answer

Full write-up: `docs/research/nono-profile-invocation.md` on branch
`research/nono-profile-invocation` (commit `ec9f6e4`). Verified
empirically against the real `nono` 0.68.0 binary (already installed on
this machine).

1. **Invocation**: `nono run --profile <namespace>/<name> -- <command>`
   (`-p` short form). Resolves three ways through the same flag: registry
   pack (`nolabs-ai/codex`), built-in/user profile by bare name, or a file
   path.
2. **Fetch-on-demand confirmed empirically**: with `nolabs-ai/codex` absent
   locally, `nono run --profile nolabs-ai/codex` auto-pulled and installed
   it before building the sandbox. Real Sigstore infrastructure confirmed
   both in the pulled `.nono-trust.bundle` (Sigstore bundle v0.3,
   DSSE/in-toto) and compiled into the binary (`fulcio.sigstore.dev`,
   `rekor.sigstore.dev`).
3. **Running the bare probe under `nolabs-ai/codex` is sensible for half
   the pack**: the resolved `policy.json` (filesystem/network grants) is a
   plain OS-level sandbox applied identically to any binary — probing
   under it is a legitimate declared-vs-actual comparison. But the pack
   also bundles Codex-specific plugin wiring (marketplace registration, a
   config.toml fence block, hook wiring, a SKILL.md) that's inert when the
   command isn't the real Codex binary — that half can't be exercised or
   validated by a bare probe run.
4. `nono audit verify` only checks hash-chain/Merkle-root/DSSE integrity of
   the local event log — proves the log wasn't tampered with, not that
   runtime behavior matched declared grants. Consistent with
   [ticket 03](03-nono-audit-command-overlap.md).

**Important incident, fully reversed**: verifying point 2 required an
actual `nono run --profile nolabs-ai/codex --dry-run -- echo hi` on this
real machine. Even under `--dry-run`, nono spliced real Codex-plugin
wiring into `~/.codex/config.toml` and `~/.codex/plugins/` — installing
the pack has side effects independent of whether the sandboxed command
ever runs. The agent caught this, ran `nono remove nolabs-ai/codex`, and
confirmed both the config block and plugin directories were removed. No
sandboxed command was ever actually executed (only `--dry-run`/`--help`).
**Consequence for [the prototype/verify work](https://github.com/chrisns/sandbox-probe-reports/blob/main/.scratch/sandbox-canary-nesting/issues/09-prototype-verify-nesting.md)-equivalent
in this map**: pack *installation* itself is not side-effect-free, even in
dry-run mode — any future empirical testing of nono profiles on a real
(non-disposable) machine needs `nono remove` as an explicit, verified
cleanup step, not an assumption.

**All 3 research tickets on this map now resolved (01, 02, 03).** Frontier
is now just the two grilling tickets: [04](04-repo-split-scope.md) and
[05](https://github.com/chrisns/sandbox-probe-reports/blob/main/.scratch/profile-attestation/issues/05-nono-row-profile-switch.md).
