Type: research
Status: resolved

## Question

What is Docker's actual out-of-the-box default sharing behavior when a
container is launched from a parent host — with **zero** explicit
isolation/mount flags — and how can the probe binary get into a
default-configured container and the report get back out, **without**
that plumbing itself being an extra grant beyond what a bare `docker run`
would already provide?

Specifically:
1. Does a bare `docker run <image> <cmd>` (no `-v`, no `--network`) share
   anything from the host filesystem by default? (Expected answer: no —
   confirm rather than assume.) Does it share network by default? (Expected:
   yes, bridge networking is Docker's actual default, unlike the current
   script's explicit `--network=none`.)
2. `docker cp` can copy a file into a container's filesystem without a bind
   mount — verify it works into a container that hasn't started yet /
   right after `docker create` and before `docker start`, and that copying
   the probe binary in this way doesn't require any extra flag that isn't
   part of Docker's default posture. Same question in reverse for getting
   `report.json` back out after the scan (`docker cp` from a stopped/exited
   container).
3. Given 1 and 2, does testing "vendor default Docker" mean literally zero
   flags, or does the project want to test Docker Desktop's own *practical*
   default (e.g. does Docker Desktop route through a VM with its own
   default host-sharing rules on macOS, distinct from bare Linux Docker)?
   Flag this distinction rather than assuming — see
   [ticket 07](07-scope-decision-agent-harnesses.md)'s sibling question.

Verify empirically using Docker Desktop's `desktop-linux` VM.

## Answer

Full write-up: `docs/research/docker-default-nesting.md` on branch
`research/docker-default-nesting` (commit `525caf5`).

1. **Filesystem: none by default.** A bare `docker run` gets zero host
   filesystem access — confirmed via `/proc/mounts` inside a zero-flag
   container: every entry is the image's own overlay root or a kernel
   pseudo-fs, no bind mount of any kind. A decoy outside the container is
   unreachable by construction.
2. **Networking: bridge by default, not none.** A bare container gets a
   real `eth0` on the bridge subnet, a default route, working DNS, and
   real outbound connectivity (`curl https://1.1.1.1` → HTTP 301). With
   the current script's explicit `--network=none`, the same request fails
   outright. Materially different postures — the current test is stricter
   than Docker's actual default.
3. **`docker cp` needs no bind mount, at any lifecycle stage.** Verified
   both directions: copy-in to a `create`d-but-never-`start`ed container,
   copy-out of a report from an already-exited one. `docker inspect` shows
   `Mounts=[]`/`Binds=[]` throughout — `cp` rides the daemon API, not the
   mount table, adding nothing beyond default sharing.
4. **Docker Desktop vs bare Linux — flagged, not assumed.** Tested on
   Docker Desktop for Mac (daemon host is a Linux VM one hop away); its
   file-sharing allowlist only gates explicit `-v` mounts, doesn't affect
   zero-flag containers. One Desktop-only quirk: DNS proxied via
   `192.168.127.7` rather than the classic Engine's `127.0.0.11`. CI runs
   bare Linux Docker directly (`ubuntu-latest`) — findings should hold
   there (Engine-level, not Desktop-specific) but couldn't be independently
   re-verified against a real bare-Linux daemon on this machine.

Consequence for [ticket 08](08-consolidate-nesting-design.md): the fix is
two changes, not one — drop `--network=none` (not a default) AND replace
the `-v "$PWD:/work"` mount with `docker cp` plumbing (also not a
default). Both currently make the test stricter/more disconnected than
real Docker.
