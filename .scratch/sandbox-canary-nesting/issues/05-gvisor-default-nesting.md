Type: research
Status: resolved

## Question

`gvisor` (`runsc`) currently runs the same fresh-`docker export` `ROOTFS`
as nspawn, via an OCI `config.json` runsc builds. Research: does `runsc`
support mounting host paths into the container's namespace via the OCI
spec's `mounts` array (the workflow's own config-generation step already
adds one bind mount for the output dir — `.mounts += [...]` — so the
mechanism clearly exists; the question is what the *default*, un-widened
posture looks like once the seeded parent's relevant paths are the only
thing beyond that added, vs. today's fresh-rootfs-with-nothing-shared).
Does gVisor have any notion of a "default" mount set the way Docker does
with bridge networking, or is everything explicit via the OCI spec (making
"vendor default" here mean "whatever a typical `runsc run` invocation
looks like in gVisor's own docs/examples")?

Needs root (`sudo`) and gVisor's `systrap` platform (no KVM, per the
existing script's comment) — verify empirically using Docker Desktop's
`desktop-linux` VM if privileged enough, otherwise note what's blocking.

## Answer

Full write-up: `docs/research/gvisor-default-nesting.md` on branch
`research/gvisor-default-nesting` (commit `7e26503`).

1. **No implicit default at all** — 100% explicit via the OCI `mounts`
   array (unlike Docker's auto-attached bridge). `runsc spec`'s own
   generated template has only three entries (`/proc`, tmpfs `/dev`,
   `/sys`), zero host-facing. Confirmed against gVisor's filesystem docs
   (all host access mediated by the gofer, serving only what's in
   `mounts`), its OCI quick-start, the OCI runtime-spec itself, and
   empirically (dumped the raw generated `config.json`).
2. **Sharing mechanism**: one more entry on the same `.mounts` array the
   workflow already uses for the output-dir mount —
   `{"destination":$home,"source":$home,"type":"bind","options":["bind","ro"]}`.
   Same source/destination keeps `os.UserHomeDir()`-based checks working
   unmodified. This is gVisor's vendor-intended mechanism (Docker's own
   `-v` for gVisor lowers to the same OCI construct) — no alternative
   primitive exists.
3. **Empirically verified end-to-end** (root/systrap unavailable on macOS,
   used Docker Desktop's Linux VM as the disposable stand-in, matching the
   sibling namespace-parity research's approach): seeded a decoy SSH key on
   the "parent," ran `runsc run` twice — without the extra mount the decoy
   is invisible (matches today's disconnected behavior); **with one added
   `mounts` entry it's readable byte-for-byte**, confirmed via both the
   probe's own report and a direct `cat`. Fingerprint (`sandbox_detection:
   "gvisor"`) still fires correctly in both cases, via the `/proc/version`
   fallback.

**Two side findings, unrelated to the mounting mechanism itself:**
- `run-probe-in-sandbox.sh`'s gvisor comment incorrectly claims
  `/__runsc_containers__` exists — confirmed it doesn't under a bare
  `runsc run` invocation (matches `environment.go`'s own comment). Small
  doc fix, fold into [ticket 08](08-consolidate-nesting-design.md)'s
  consolidation alongside the real invocation change.
- **Separate probe bug**: `pkg/tasks/baseline`'s mount enumerator
  (`mounted_volumes_detections`) didn't surface the gVisor bind mount even
  though the file was demonstrably reachable through it. This is a gap in
  the probe's own detection code, not a nesting/methodology question —
  flagged to Chris directly, not blocking this map.
