Type: research
Status: resolved

## Question

Map nono's declared `Profile`/`CapabilitySet` schema fields (filesystem
grants, network access, credentials, etc. — per the profile-format
reference already partially fetched during charting, cite
`deepwiki.com/always-further/nono/3.3-profile-format-reference` and cross-
check against nono's own official schema docs if available, e.g. a JSON
Schema file in the nono repo) to `sandbox-probe`'s existing `finding_type`
values (`sensitive_readable_paths`, `writeable_paths`,
`external_host_dns_resolution`, `external_host_connectivity`,
`tcp_ports_open`, `udp_ports_open`, `unix_socket_detection`,
`process_detection`, `mounted_volumes_detections`, `user_context_detection`
— full list and semantics in `README.md`'s "What it detects" table and
`pkg/tasks/tasks.go`).

For each nono schema field category (filesystem read/write grants, network
allowlist, env_credentials, workdir access, etc.), identify:
1. Which probe `finding_type`(s) would empirically observe whether that
   grant is actually reachable.
2. Any category where nono grants something the probe currently has NO way
   to observe at all (a probe capability gap, not a schema-mapping
   problem) — flag these explicitly, don't force a mapping that doesn't
   exist.
3. Any category where the probe observes something nono's schema has no
   corresponding declaration for (e.g. does nono's schema say anything
   about process visibility or host mounts, which the probe checks but
   nono's format reference didn't mention?).

This mapping is what "declared-but-unreachable" / "reachable-but-
undeclared" diffing will be built on, so precision matters more than
completeness — where the mapping is genuinely ambiguous, say so rather than
guessing.

Deliverable: write findings to `docs/research/nono-schema-findings-mapping.md`.
Cite nono's actual schema/docs. Commit on branch
`research/nono-schema-mapping` (from current HEAD). Final response:
concise summary (under 300 words), the mapping table, file path +
branch/commit.

## Answer

Full write-up: `docs/research/nono-schema-findings-mapping.md` on branch
`research/nono-schema-mapping` (commit `c3f3b76`). Sourced from nono's
**real JSON Schema files**, fetched directly from `nolabs-ai/nono` (the
org migrated from `always-further` — confirmed again here) —
`crates/nono-cli/data/nono-profile.schema.json` (author-facing `Profile`)
and `crates/nono/schema/capability-manifest.schema.json` (resolved
`CapabilitySet`) — cross-checked against the Rust struct and the authoring
docs, not just prose.

**Clean mappings**: `filesystem.read/write/allow(_file)` →
`sensitive_readable_paths`/`writeable_paths`; `network.block`/
`allow_domain` → `external_host_dns_resolution`/`external_host_connectivity`;
`open_port`/`listen_port` → `tcp_ports_open`; `filesystem.unix_socket*` →
`unix_socket_detection`; `security.process_info_mode` →
`process_detection`/`parent_process_detection`.

**Probe-side gap, genuinely surprising**: nono declares `env_credentials`/
`secrets`, and the probe has code that looks purpose-built to observe
exactly this — a `models.EnvFinding` type and a `detectSensitiveEnvVars()`
function (gitleaks over env vars) — **but neither is wired to a
`finding_type`. It's dead code.** Also no coverage for
`security.signal_mode`, `security.ipc_mode`, `allow_gpu`/
`allow_launch_services`, `resources.*` ceilings, or L7 method+path
`endpoints` filtering.

**Nono-side gap**: `mounted_volumes_detections`, `hostname_detection`,
`user_context_detection` have no nono equivalent — nono mediates via
Landlock/Seatbelt, never a namespace/rootfs swap, so there's nothing for
it to declare there by design (consistent with
[the firejail/nono/srt flag audit](../sandbox-canary-nesting/issues/06-firejail-nono-srt-flag-audit.md)
in the sibling map).

**Side finding, flagged not silently resolved**: nono's own checked-in
JSON Schema is itself stale — it omits the `unix_socket*` fields that the
real Rust struct and authoring docs both define and use.

**Ambiguous, called out rather than guessed**: whether nono's proxy/TLS-
intercept env vars land in the standard `HTTP_PROXY`-family vars
`proxy_detection` already scans for — needs an empirical run to confirm.

**Consequence**: the `env_credentials` dead-code finding is a real,
separate, small bug in the probe itself (wire `EnvFinding`/
`detectSensitiveEnvVars()` to an actual `finding_type`) — worth fixing
regardless of whether profile attestation ships, since it's the exact
category most nono profiles restrict most tightly.
