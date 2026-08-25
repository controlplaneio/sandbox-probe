package tasks

import "strings"

// Writeable paths are not all the same capability, and counting them as if they were makes a
// sandbox look weaker than no sandbox at all.
//
// Measured on a GitHub runner, GitHub Copilot CLI's Linux sandbox (MXC's bubblewrap backend)
// against the same runner unconfined:
//
//	unconfined   writeable_paths = [/opt]
//	confined     writeable_paths = [/usr /dev /var /opt]
//
// The confined run reports FOUR. The writes are real — an actual create in /usr succeeds inside
// that sandbox — so this is not a false positive that a stricter check would remove. What changed
// is what /usr *is*. The sandbox builds a fresh tmpfs root and bind-mounts host content at the
// CHILDREN only:
//
//	tmpfs     -> /            (tmpfs,  root=/newroot)
//	/dev/root -> /usr/bin     (ext4,   root=/usr/bin)
//	/dev/root -> /usr/lib     (ext4,   root=/usr/lib)
//	/dev/root -> /opt/pipx_bin(ext4,   root=/opt/pipx_bin)
//
// There is no mount at /usr, /var or /opt themselves. They are bare directories on the sandbox's
// own tmpfs, owned by the sandbox user, existing only as mount points. Writing to them writes to
// memory that dies with the sandbox and cannot touch the host's /usr. The unconfined run's single
// /opt is the real thing on disk.
//
// So the honest split is not "sandbox versus host" — the probe cannot attest who built a mount —
// but PERSISTENT versus EPHEMERAL, which follows from the backing filesystem's type and is
// kernel-attested from /proc/self/mountinfo on any host, sandboxed or not. Under that split the
// same measurement reads the right way round: the sandbox takes persistent write access from one
// path to none.

// memoryBacked reports whether a filesystem type keeps its contents only in memory, so a write
// that lands on it cannot outlive the mount or modify anything another mount exposes.
//
// devtmpfs is included and is not a special case: /dev inside the sandbox above is a synthesised
// tmpfs with individual device nodes bound in, and a write to the directory itself is as ephemeral
// as any other. Disk-backed overlays are deliberately absent — an overlay's upper layer persists,
// so writes to it are not ephemeral even though a container may discard them later. That is a
// lifecycle claim about the container, which this cannot attest.
func memoryBacked(fstype string) bool {
	switch fstype {
	case "tmpfs", "ramfs", "devtmpfs":
		return true
	}

	return false
}

// nonPersistentFS reports whether a write landing on this filesystem can leave nothing behind on
// any real filesystem — either because the contents live only in memory, or because the kernel
// synthesises them and a file cannot be created there at all.
//
// The second half is not a refinement, it is the same defect again. Measured in the same
// comparison as above, /proc is reported writeable on BOTH sides:
//
//	unconfined  writeable_paths = [... /sys /proc /dev ...]
//	confined    writeable_paths = [/usr /proc /dev /var]
//
// isWritable trusts access(2) for directories and never attempts a write (see filesystem_unix.go,
// "Access above is sufficient"). Running as root, access() answers yes for /proc because the mode
// bits allow it — while an actual create fails, because procfs has no backing store to create in.
// Counting that as write access to the system overstates the capability exactly as counting the
// sandbox's tmpfs scaffolding did.
//
// isKernelPseudoFilesystem is reused rather than a second list being written here: it already
// enumerates the interfaces the kernel synthesises, and one list that two callers share cannot
// drift out of step with itself.
func nonPersistentFS(fstype string) bool {
	return memoryBacked(fstype) || isKernelPseudoFilesystem(fstype)
}

// mountCovers reports whether mountPoint is at or above path.
//
// Compared by path component, never by raw string prefix: "/usr" must not be read as covering
// "/usrlocal", which a plain strings.HasPrefix would do and which would attribute a path to the
// wrong filesystem — silently, and only on hosts that happen to have such a directory.
func mountCovers(mountPoint, path string) bool {
	if mountPoint == path || mountPoint == "/" {
		return true
	}

	return strings.HasPrefix(path, strings.TrimSuffix(mountPoint, "/")+"/")
}

// backingMount returns the mount that actually serves path: the deepest one at or above it.
//
// Depth decides, because mounts nest. /usr/bin bound over a tmpfs root covers /usr/bin, while the
// root covers everything — and only the deeper of the two describes what a write to /usr/bin
// touches. Returns false when no mount covers the path, which on a host with no readable mount
// table is every path.
func backingMount(path string, mounts []Mount) (Mount, bool) {
	var best Mount
	found := false

	for _, m := range mounts {
		if !mountCovers(m.Target, path) {
			continue
		}
		if !found || len(m.Target) > len(best.Target) {
			best, found = m, true
		}
	}

	return best, found
}

// ephemeralWriteablePaths returns the subset of writeable whose backing filesystem cannot keep a
// write — memory-only or kernel-synthesised — preserving the order it was given.
//
// A path whose backing mount cannot be identified is NOT reported as ephemeral. The claim this
// finding makes is "a write here does not persist", and an unidentified mount does not support it;
// staying silent is the recoverable error, claiming ephemerality wrongly is not.
func ephemeralWriteablePaths(writeable []string, mounts []Mount) []string {
	out := make([]string, 0, len(writeable))

	for _, p := range writeable {
		m, ok := backingMount(p, mounts)
		if ok && nonPersistentFS(m.FSType) {
			out = append(out, p)
		}
	}

	return out
}
