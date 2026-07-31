Type: research
Status: resolved

## Question

`sandbox-probe` has no Windows equivalent of `unix_socket_detection` at all
today — `pkg/tasks/baseline/network.go`'s socket scanner is Unix-only, and
its own tests explicitly skip on Windows. Named pipes
(`\\.\pipe\...`, e.g. Docker Desktop's `\\.\pipe\docker_engine`) are the
real equivalent.

Research, against primary sources (Go stdlib docs, Windows API docs, the
existing `pkg/tasks/baseline/filesystem_windows.go` for the codebase's own
existing build-tag pattern for Windows-specific code):

1. How can Go enumerate active named pipes on Windows — is there a
   directory-listing-style approach analogous to scanning `/run` for
   socket-typed entries (e.g. via the pipe namespace under `\\.\pipe\`), or
   does it require a different Windows API (`FindFirstFile`/`FindNextFile`
   against the pipe namespace, `NtQueryDirectoryFile`, something else)?
2. Does enumerating require elevated privileges, or is it available to a
   normal user process (matters for parity with the unprivileged Unix
   socket scan)?
3. What would the equivalent of `DefaultSocketRoots`/`ScanSocketRoots` look
   like for Windows — a single well-known root (the pipe namespace itself
   has no subdirectories the way `/run` vs `/tmp` do), or something else?

Write findings to a Markdown file in the repo (match `docs/` convention),
citing sources. Capture on a throwaway `research/windows-named-pipes`
branch.

## Answer

Full write-up: `docs/research/windows-named-pipe-enumeration.md` on branch
`research/windows-named-pipes` (commit `694f248`).

1. **No Win32 API path exists** — Microsoft's own Sysinternals `PipeList`
   docs state enumeration is undocumented and impossible via Win32; the
   real mechanism is the native `NtQueryDirectoryFile` (the function
   `FindFirstFile`/`FindNextFile` are themselves built on, just not wired up
   for the pipe namespace at the Win32 layer). Go's stdlib doesn't expose
   this — `os.ReadDir`/`os.Open` on `\\.\pipe\` are confirmed broken,
   tracked upstream as golang/go#61918, #32423, #41755. Practical path:
   `golang.org/x/sys/windows` (official Go sub-repo — **new dependency**,
   not currently in `go.mod`) to reach `NtQueryDirectoryFile`, likely via a
   raw ntdll syscall binding rather than an already-wrapped helper.
2. **No admin/elevation requirement found** for enumeration itself (as
   opposed to opening a specific pipe's data stream, gated by that pipe's
   own DACL). Inferred from absence of any stated requirement in
   Sysinternals' docs (which do flag elevation prominently where it's
   needed) plus named pipes' default DACL granting read to `Everyone` —
   worth confirming empirically once code exists.
3. **The `DefaultSocketRoots`/`ScanSocketRoots` abstraction doesn't carry
   over** — the pipe namespace has exactly one root (`\\.\pipe\`) with no
   subdirectory structure, so the Windows equivalent is a flat one-shot
   listing, not a multi-root recursive walk.

Consequence for [ticket 05](05-ipc-finding-type-naming.md): this is
real new Go code (a new dependency, a raw syscall, no reuse of the
existing Unix socket scanner's shape) — not a thin platform shim, which
should weigh on whether it gets its own `finding_type` or generalizes
`unix_socket_detection`.
