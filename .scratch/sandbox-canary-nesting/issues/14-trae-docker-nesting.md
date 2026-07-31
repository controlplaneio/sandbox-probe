Type: research
Status: resolved

## Question

`scripts/run-probe-via-trae-stub.sh` with `TRAE_DOCKER=on` uses
`trae-agent`'s own `--docker-image` mode, which binds `--working-dir` into
the container (per the script's own comment: "docker mode uses the
in-container /workspace mount (trae binds --working-dir there) and the
report is copied back after"). Same shape of question as
[the gemini-docker ticket](13-gemini-docker-nesting.md): this is the
*vendor's* (bytedance/trae-agent's) own docker invocation, not our
script's.

Investigate:
1. What does trae-agent's `--docker-image` mode actually mount by default
   — read trae-agent's own source (it's checked out locally as
   `TRAE_SRC` per the stub script; find the actual docker-invocation code)
   rather than inferring. Does it touch `$HOME`, or only the working
   directory?
2. Empirically: seed decoys in `$HOME`, run with `TRAE_DOCKER=on`, check
   reachability from inside trae's own container.
3. Same reasoning caveat as the gemini ticket: if trae's own default is
   genuinely narrower than `$HOME` and that's what real trae-agent users
   get too, that's a legitimate result, not a bug to paper over. Only
   extend sharing if there's a way to do it that doesn't change trae's own
   default behavior for other users (check if `DOCKER_FLAGS` or similar in
   the stub script is an available extension point, the way `SANDBOX_FLAGS`
   is for gemini).

## Answer

**No fix needed — but for a different reason than claude/codex.** Full
write-up: `docs/research/trae-docker-nesting.md` on branch
`research/trae-docker-nesting` (commit `07b85bf`).

Read trae-agent's own installed source directly (bytedance/trae-agent,
`e839e559`): `DockerManager.start()` constructs exactly one bind mount —
`--working-dir` → `/workspace` — nothing else. No `$HOME`, no credential
dirs, and no CLI flag or config field exists to add a second mount.

Empirically confirmed: seeded real decoys in `$HOME`, ran a real
`trae-cli --docker-image ubuntu:latest` through the stub. The probe's own
`mounted_volumes_detections`, from inside trae's real container, shows
exactly the working-dir mount and nothing else — zero matches for anything
under `$HOME`/`.ssh`/`.aws` in the full report. Networking is open
(bridge, real DNS/connectivity) since trae never restricts it — matches
Docker's own bare-default from [ticket 01](01-docker-default-nesting.md).

**This is a genuine container on the real Docker daemon on the real seeded
host, using trae's own unmodified, unextendable mount logic** — not a
harness-constructed disconnected environment the way the original 5
runtimes were. It's correctly nested by construction; there's nothing to
fix. No extension point exists either (`DOCKER_FLAGS` in the stub is
purely internal, not a Docker passthrough) — and forcing one via
`--docker-container-id` against an externally-built container would
reintroduce the exact "harness constructs its own environment" anti-pattern
this whole map exists to eliminate. So even if wider sharing were desired,
it's correctly *not* being added.
