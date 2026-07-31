Type: prototype
Status: open

Blocked by: 08

## Question

Build a minimal, throwaway prototype of the chosen per-runtime invocation
changes from [ticket 08](08-consolidate-nesting-design.md), and verify by
hand, per runtime, using Docker Desktop's `desktop-linux` VM (or a real
Linux box for nspawn/gvisor if that VM can't run them privileged):

1. A canary planted in the seeded parent (mirroring `seed-decoys.sh`) is
   at least *reachable in principle* from inside the newly-nested sandbox
   invocation (i.e. the sandbox is a genuine child now, not a sibling).
2. The runtime still fingerprints correctly (`sandbox_detection` still
   reports the right value — don't break the existing `expect` assertions
   in `scan-matrix.yaml`).
3. Whatever default restriction the runtime actually applies still shows
   up as a real, meaningful block where it should (e.g. if Docker's
   network default is bridge-not-none, does the probe's network-egress
   check now show something *real* instead of an artificial `--network=none`
   blank).

HITL prototype session (react to the concrete artifact together), one
runtime at a time or as a batch — Chris's call when resolving.

## Answer

(pending)
