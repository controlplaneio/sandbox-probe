Type: research
Status: resolved

## Question

DeepWiki's nono documentation references a `nono audit` command (or
similarly-named verification tooling) in passing, without detail. Before
this map builds a new "declared vs actual" attestation capability, confirm
whether nono already has something that does this, partially or fully.

1. Find and read nono's actual `audit` command documentation (or whatever
   the real command is called — check `nono --help`/`nono audit --help`
   equivalents in the docs, or the CLI reference section of nono's docs
   site).
2. Does it verify runtime behavior against the profile's declared grants
   (the actual gap this map is trying to fill), or is it schema/structural
   validation only (matching the pattern already found for profile
   installation — Sigstore + JSON-schema checks, no behavioral
   verification)?
3. If it does some behavioral checking already, what exactly, and where's
   the gap between what it does and full empirical attestation (e.g. does
   it check declared-but-unreachable, reachable-but-undeclared, both,
   neither)?
4. Does nono have any existing integration point (a plugin hook, an output
   format, an API) that a tool like `sandbox-probe` could plug into rather
   than building a fully separate attestation mechanism — worth knowing
   before [the repo-split ticket](04-repo-split-scope.md) assumes a
   from-scratch build.

Deliverable: write findings to `docs/research/nono-audit-command.md`. Cite
nono's actual docs. If no such command exists or the docs are too thin to
answer definitively, say so plainly rather than speculating. Commit on
branch `research/nono-audit-overlap` (from current HEAD). Final response:
concise summary (under 300 words), file path + branch/commit.

## Answer

**Confirmed: the gap is real, nothing in nono fills it.** Full write-up:
`docs/research/nono-audit-command.md` on branch `research/nono-audit-overlap`
(commit `5da6ff7`). Verified against nono's own docs and the installed
`nono 0.68.0` binary empirically.

`nono audit` is a **forensic session log**, not a policy-compliance
checker — nono's own docs draw this distinction explicitly: audit recording
answers "what happened?", `audit verify` answers "has the log been
tampered with?" Neither answers "did the agent violate its declared
permissions?" Checked every adjacent command:
- `profile diff` — diffs two **declared** profiles against each other,
  never against an observed session.
- `profile validate` — schema only.
- `trust verify` — signature/provenance only.
- `nono why` — a static policy simulator (nothing executes), except
  `--self`, which lets an already-sandboxed process query its own live
  capability set — real runtime introspection, but self-service only, not
  an external declared-vs-observed diff.

**No command anywhere computes declared-but-unreachable or
reachable-but-undeclared.** This isn't a documentation gap — it's a real,
confirmed gap in nono itself.

**Integration surface (point 4)**: no plugin/hook/webhook API for
third-party verification tools, but both sides of a diff are already
machine-readable — `profile show --json` (declared grants) and
`audit show --json` / raw `audit-events.ndjson` (observed session data).
`sandbox-probe` wouldn't need to reverse-engineer either format, but would
still need to build the diffing logic itself from scratch.
