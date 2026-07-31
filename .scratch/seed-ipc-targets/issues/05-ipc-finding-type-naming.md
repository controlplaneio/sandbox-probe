Type: grilling
Status: resolved

Blocked by: 04

## Question

Once [the named-pipe enumeration research](04-windows-named-pipe-enumeration.md)
shows what the Windows detection code actually looks like, decide: does
Windows named-pipe detection get its own new `finding_type` (e.g.
`named_pipe_detection`), or does `unix_socket_detection` generalize into a
cross-platform IPC finding type that covers both?

This is a domain-modeling decision — touches `CONTEXT.md`'s Capability
category table (currently "IPC sockets" → `unix_socket_detection` only) and
the site's `FT2CAT` mapping in `site/app.js`. Update `CONTEXT.md` the
moment this crystallises.

## Answer

**Separate finding types.** `unix_socket_detection` stays as-is; Windows
gets a new `named_pipe_detection`, folded together under the "IPC sockets"
category — the same pattern `CONTEXT.md` already uses for "Network egress"
(`external_host_dns_resolution` + `external_host_connectivity`, folded).

Two reasons: (1) a report is always OS-scoped — one run, one OS, never
both platforms at once, which is already knowable from that report's own
`environment_detection` finding — so a generalized type would need a
discriminator field that's always redundant. (2) the category is already
the layer where cross-platform generalization happens; finding types stay
concrete and accurately named (a named pipe genuinely isn't a Unix
socket).

`CONTEXT.md`'s Capability category table updated: `IPC sockets` row now
reads `unix_socket_detection`, `named_pipe_detection` (folded). The
`README.md` "What it detects" table entry and `site/app.js`'s `FT2CAT`
mapping are implementation follow-ups for whoever builds this (not done
now — `named_pipe_detection` doesn't exist as code yet, this is the domain
decision, not the build).
