# Research: enumerating Windows named pipes from Go

For [wayfinder ticket 04](../../.scratch/seed-ipc-targets/issues/04-windows-named-pipe-enumeration.md).
No existing `docs/research/` convention in this repo — establishing one here;
future research notes should land alongside this file.

## 1. How to enumerate active named pipes

The device driver behind named pipes is a filesystem driver, `NPFS.SYS`
("Named Pipe File System"), and the pipe namespace `\\.\pipe\` is a listable
directory in that filesystem — but this is **not exposed through the Win32
API**. Microsoft's own Sysinternals `PipeList` tool documentation states
this explicitly:

> "This fact is not documented, nor is it possible to do this using the
> Win32 API. Directly using `NtQueryDirectoryFile`, the native function that
> the Win32 `FindFile` APIs rely on, makes it possible to list the pipes."
> — [Pipelist – Sysinternals | Microsoft Learn](https://learn.microsoft.com/en-us/sysinternals/downloads/pipelist)

So the mechanism is: call the **native** (`ntdll.dll`) function
`NtQueryDirectoryFile` against a handle opened on `\\.\pipe\`, the same
underlying call the Win32 `FindFirstFile`/`FindNextFile` family is built on
for ordinary directories — it just isn't wired up for the pipe namespace at
the Win32 layer.

**Go stdlib does not support this today.** This is a confirmed, tracked
upstream limitation, not a gap in how the code would call it:

- [golang/go#61918](https://github.com/golang/go/issues/61918) — `os.ReadDir(\\.\pipe\)` fails on Windows as of Go 1.21
- [golang/go#32423](https://github.com/golang/go/issues/32423) — `os`/`ioutil` cannot iterate named pipes
- [golang/go#41755](https://github.com/golang/go/issues/41755) — Windows named pipes are misreported as regular files by `Stat`

`os.ReadDir`/`os.Open` on `\\.\pipe\` report "not a directory" and fail —
consistent with the Win32-API gap above, since stdlib's directory-reading
path goes through the Win32 `FindFile` family, not raw `NtQueryDirectoryFile`.

**Practical path for this codebase**: `golang.org/x/sys/windows` (the
official Go sub-repo, not a third-party dependency) exposes low-level NT/Win32
bindings and is the natural place to reach for `NtQueryDirectoryFile`-level
access. This is a **new dependency** — `go.mod`/`go.sum` currently have no
`golang.org/x/sys` entry at all. Sketch of the shape (illustrative, not
verified to compile):

```go
//go:build windows

package tasks

import "golang.org/x/sys/windows"

func scanNamedPipes() ([]string, error) {
	h, err := windows.CreateFile(
		windows.StringToUTF16Ptr(`\\.\pipe\`),
		windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(h)
	// NtQueryDirectoryFile via golang.org/x/sys/windows (or a direct
	// syscall.NewLazyDLL("ntdll.dll") call — x/sys/windows doesn't wrap
	// NtQueryDirectoryFile directly as of writing, so this may need a raw
	// ntdll syscall binding rather than a ready-made x/sys helper) to
	// enumerate FILE_DIRECTORY_INFORMATION entries.
	...
}
```

This is meaningfully more involved than the existing Unix socket scan
(`ScanSocketRoots`, a plain recursive directory walk with `os.Stat` for
socket-typed entries) — it needs a raw NT syscall, not just an
already-wrapped stdlib/x/sys call. Third-party pure-Go named-pipe libraries
exist (`natefinch/npipe`, `golang.zx2c4.com/wireguard/ipc/namedpipe`) but
they implement pipe *client/server* connections (dial/listen/accept), not
directory enumeration — not directly useful for this task.

## 2. Privilege requirements

No source found states an administrator/elevation requirement for
*enumerating* pipes (as opposed to opening a specific pipe instance's data
stream, which is governed by that pipe's own DACL — see
[Named Pipe Security and Access Rights](https://learn.microsoft.com/en-us/windows/win32/ipc/named-pipe-security-and-access-rights)).
The Sysinternals `PipeList` documentation doesn't flag any elevation
requirement (Sysinternals tools that need admin rights say so prominently —
e.g. Process Explorer's driver-based features). Named pipes' default
security descriptor grants read access to `Everyone` and anonymous by
default per the Win32 docs above, which is consistent with unprivileged
enumeration being the norm, though this is inference from absence of a
documented restriction rather than an explicit "no admin needed" statement —
worth confirming empirically once code exists, ideally in
[the Windows dev-machine footprint capture ticket](../../.scratch/seed-ipc-targets/issues/07-windows-dev-machine-footprint-capture.md).

## 3. Shape of the equivalent to `DefaultSocketRoots`/`ScanSocketRoots`

Unlike Unix sockets — which can live under several real directories
(`/run`, `/tmp`, `$XDG_RUNTIME_DIR`, etc.) that the current code walks as a
list of roots — the pipe namespace has **exactly one root**, `\\.\pipe\`,
with no subdirectory structure to walk. So the Windows equivalent isn't a
multi-root recursive walk; it's a single `NtQueryDirectoryFile` listing
against that one handle. The abstraction shape changes: `DefaultSocketRoots`
has no Windows analogue (nothing to configure — there's only one root), and
`ScanSocketRoots`'s recursion/symlink-dedup logic is irrelevant on the
Windows side — a Windows equivalent function would be a flat, one-shot
enumeration, not a walk.

## Sources

- [Pipelist – Sysinternals | Microsoft Learn](https://learn.microsoft.com/en-us/sysinternals/downloads/pipelist)
- [Named Pipe Security and Access Rights – Win32 apps | Microsoft Learn](https://learn.microsoft.com/en-us/windows/win32/ipc/named-pipe-security-and-access-rights)
- [golang/go#61918](https://github.com/golang/go/issues/61918)
- [golang/go#32423](https://github.com/golang/go/issues/32423)
- [golang/go#41755](https://github.com/golang/go/issues/41755)
