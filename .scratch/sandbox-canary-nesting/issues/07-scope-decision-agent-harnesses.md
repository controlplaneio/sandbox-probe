Type: grilling
Status: resolved

## Question

Does this map's fix extend to the agent-specific sandboxed harnesses
(`claude-sandbox`, `codex-sandbox`, `gemini-docker`, `trae-docker`), which
go through their own stub-runner scripts
(`run-probe-via-claude-stub.sh`/`run-probe-via-codex-stub.sh`/etc.), not
`run-probe-in-sandbox.sh` — or does this map stay scoped to the 5 generic
`sandbox`-family runtime rows for now, with the agent harnesses as a
follow-on?

Those stub runners may already nest correctly (the real agent CLI decides
its own sandbox invocation, which may or may not reconstruct a fresh
environment the same way) or may share the identical bug — genuinely
unknown until read. This is a HITL scope call, not something to assume
either way while charting.

## Answer

**In scope now** — Chris's call: pull the agent-specific harnesses into
this map rather than deferring them.

Peeked at the four stub scripts to ground the new tickets before writing
them (not a full investigation — that's what the new research tickets are
for):

- `run-probe-via-claude-stub.sh` / `run-probe-via-codex-stub.sh`: the real
  agent CLI applies its own sandbox (bwrap on Linux, Seatbelt on macOS)
  directly to the shell subprocess, in-process — no fresh rootfs or
  container gets constructed the way `run-probe-in-sandbox.sh` did. This
  looks structurally like the already-confirmed-fine `srt`/`firejail`/
  `nono` cases (same filesystem, policy-restricted) rather than the
  broken `docker`/`podman`/`bwrap`/`nspawn`/`gvisor` cases — but that's an
  assumption, not yet confirmed. See
  [ticket 11](11-claude-sandbox-nesting.md) /
  [ticket 12](12-codex-sandbox-nesting.md).
- `run-probe-via-gemini-stub.sh` (`GEMINI_SANDBOX=docker`): re-execs the
  **whole CLI** inside a container via gemini-cli's own `--sandbox` flag,
  which "mounts the workspace" (per the script's own comment) — unclear if
  that includes `$HOME` or just the project dir. Our stub already injects
  extra docker flags into gemini's own invocation via a `SANDBOX_FLAGS` env
  var (for mock-reachability reasons) — that's a promising existing
  injection point if extending the share turns out to be needed. See
  [ticket 13](13-gemini-docker-nesting.md).
- `run-probe-via-trae-stub.sh` (`TRAE_DOCKER=on`): trae's own
  `--docker-image` mode binds `--working-dir` into the container — same
  "vendor's own docker invocation, unclear what's actually shared" shape
  as gemini. See [ticket 14](14-trae-docker-nesting.md).

Consequence: [ticket 08](08-consolidate-nesting-design.md)'s "Blocked by"
now includes 11–14 alongside 01–06.
