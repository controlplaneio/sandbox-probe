Type: research
Status: resolved

## Question

Same question as [the Docker ticket](01-docker-default-nesting.md), for
Podman specifically: what does a bare `podman run` share by default
(filesystem, network), and how does the probe binary get in / report get
out without adding sharing beyond that default? Podman is rootless by
default (unlike Docker) — check whether that changes what's reachable from
a seeded parent in ways Docker's answer doesn't cover (e.g. does rootless
mode change how `/run`-family paths map, relevant to the sibling
seed-ipc-targets map's socket seeding too).

Verify empirically using Docker Desktop's `desktop-linux` VM if Podman is
available there, or note if it needs installing.

## Answer

Full write-up: `docs/research/podman-default-nesting.md` on branch
`research/podman-default-nesting` (commit `2e2f066`). Podman v6.0.2,
rootless VM via `podman machine`.

1. **Filesystem**: none shared by default — a canary file was invisible to
   a bare `podman run` (`ENOENT`, `/tmp`/`/home`/`/Users` all
   empty/absent). Identical structural-absence signature to Docker.
2. **Rootless mode**: doesn't change *what's* shared by default (still
   nothing), but changes *ownership resolution* once something IS
   bind-mounted: container UID 0 maps 1:1 to the real invoking host user
   (`/proc/self/uid_map`), UIDs 1+ map through `/etc/subuid`. A host file
   owned by the invoking user reads as "root" inside the container
   automatically — relevant for the sibling seed-ipc-targets socket work:
   permission checks on a bind-mounted `/run/user/<uid>` socket resolve
   through this mapping, not literal UID equality.
3. **Networking**: shared by default — bridge network, NAT, working DNS,
   outbound internet (via slirp4netns/pasta), mirroring Docker's default.
4. **`podman cp`**: works with zero bind mounts, same two-step
   create-then-copy shape as `docker cp` — copy-in before start, copy-out
   after exit, both verified.

Same conclusion as [the Docker ticket](01-docker-default-nesting.md): both
current script flags (`--network=none`, `-v "$PWD:/work"`) are harness
additions, not Podman defaults.
