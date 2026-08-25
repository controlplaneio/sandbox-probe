package tasks

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The mount table measured inside GitHub Copilot CLI's Linux sandbox (MXC's bubblewrap backend) on
// a GitHub runner, reduced to the entries that decide the answer. A tmpfs root with host content
// bind-mounted at the CHILDREN, and nothing mounted at /usr, /var or /opt themselves.
func copilotSandboxMounts() []Mount {
	return []Mount{
		{Source: "tmpfs", Target: "/", FSType: "tmpfs", Root: "/newroot"},
		{Source: "/dev/root", Target: "/usr/bin", FSType: "ext4", Root: "/usr/bin"},
		{Source: "/dev/root", Target: "/usr/lib", FSType: "ext4", Root: "/usr/lib"},
		{Source: "/dev/root", Target: "/opt/pipx_bin", FSType: "ext4", Root: "/opt/pipx_bin"},
		{Source: "tmpfs", Target: "/dev", FSType: "tmpfs", Root: "/"},
		{Source: "/dev/root", Target: "/home/runner/work", FSType: "ext4", Root: "/home/runner/work"},
	}
}

// The same runner unconfined: one real disk filesystem at the root.
func unconfinedMounts() []Mount {
	return []Mount{
		{Source: "/dev/root", Target: "/", FSType: "ext4", Root: "/"},
		{Source: "tmpfs", Target: "/dev/shm", FSType: "tmpfs", Root: "/"},
		{Source: "tmpfs", Target: "/run", FSType: "tmpfs", Root: "/"},
	}
}

// The finding exists because the raw count moves the WRONG WAY under a sandbox: confined reports
// four writeable paths against unconfined's one, which reads as the sandbox granting more write
// access than no sandbox at all. Splitting by backing filesystem puts it right — every one of the
// confined four is memory-backed and cannot persist, so persistent write access goes 1 -> 0.
//
// This is the regression guard for the whole change: if it fails, the published comparison has
// gone back to being misleading.
func TestSandboxWriteablePathsAreEphemeralAndUnconfinedOnesAreNot(t *testing.T) {
	confined := []string{"/usr", "/dev", "/var", "/opt"}
	unconfined := []string{"/opt"}

	gotConfined := ephemeralWriteablePaths(confined, copilotSandboxMounts())
	assert.Equal(t, confined, gotConfined,
		"every writeable path in the sandbox sits on its tmpfs root, so all four are ephemeral")

	gotUnconfined := ephemeralWriteablePaths(unconfined, unconfinedMounts())
	assert.Empty(t, gotUnconfined,
		"/opt on a real ext4 root persists and must never be reported as ephemeral")

	// The point of the split, stated as the assertion a reader cares about.
	persistent := func(all, ephemeral []string) int { return len(all) - len(ephemeral) }
	assert.Equal(t, 0, persistent(confined, gotConfined))
	assert.Equal(t, 1, persistent(unconfined, gotUnconfined))
}

// Depth decides, not order: /usr/bin is bound over a tmpfs root, and only the deeper mount
// describes what a write to /usr/bin actually touches.
func TestBackingMountIsTheDeepestCoveringMount(t *testing.T) {
	for _, tt := range []struct {
		path       string
		wantTarget string
		wantFS     string
	}{
		{"/usr/bin", "/usr/bin", "ext4"},
		{"/usr/bin/env", "/usr/bin", "ext4"},
		{"/usr", "/", "tmpfs"},
		{"/usr/share", "/", "tmpfs"},
		{"/opt", "/", "tmpfs"},
		{"/opt/pipx_bin", "/opt/pipx_bin", "ext4"},
		{"/dev", "/dev", "tmpfs"},
		{"/home/runner/work", "/home/runner/work", "ext4"},
	} {
		t.Run(tt.path, func(t *testing.T) {
			m, ok := backingMount(tt.path, copilotSandboxMounts())
			assert.True(t, ok, "the root mount covers everything, so a path must always resolve")
			assert.Equal(t, tt.wantTarget, m.Target)
			assert.Equal(t, tt.wantFS, m.FSType)
		})
	}
}

// The boundary a raw strings.HasPrefix gets wrong. "/usr" is not above "/usrlocal", and reading it
// as though it were would attribute that path to the wrong filesystem — silently, and only on
// hosts that happen to have such a directory, which is the worst way for a bug to behave.
func TestMountCoversComparesPathComponentsNotStringPrefixes(t *testing.T) {
	for _, tt := range []struct {
		mountPoint, path string
		want             bool
	}{
		{"/usr", "/usr", true},
		{"/usr", "/usr/bin", true},
		{"/usr", "/usrlocal", false},
		{"/usr", "/usr-backup", false},
		{"/", "/anything", true},
		{"/", "/", true},
		{"/usr/bin", "/usr", false},
		{"/var/log", "/var/logrotate", false},
	} {
		t.Run(fmt.Sprintf("%s_covers_%s", tt.mountPoint, tt.path), func(t *testing.T) {
			assert.Equal(t, tt.want, mountCovers(tt.mountPoint, tt.path))
		})
	}
}

// absent != empty. With no readable mount table nothing can be attributed to a filesystem, so the
// scan must record that it measured nothing — and baseline.go omits the finding entirely rather
// than publishing an empty list that reads as "measured, nothing ephemeral".
func TestEphemeralIsUnmeasuredWhenTheMountTableCannotBeRead(t *testing.T) {
	orig := mountTable
	t.Cleanup(func() { mountTable = orig })

	mountTable = func() ([]Mount, bool) { return nil, false }
	got := scanTargetedPathsForHome(t.TempDir())
	assert.False(t, got.MountsReadable,
		"an unreadable mount table must be recorded, so the caller can omit the finding")
	assert.Empty(t, got.EphemeralWritablePaths)

	mountTable = func() ([]Mount, bool) { return unconfinedMounts(), true }
	got = scanTargetedPathsForHome(t.TempDir())
	assert.True(t, got.MountsReadable,
		"a readable table means the answer is measured, even when nothing is ephemeral")
}

// A path no mount covers is not ephemeral. The finding claims "a write here does not persist", and
// an unidentified backing filesystem does not support that claim.
func TestUnattributablePathIsNotClaimedEphemeral(t *testing.T) {
	noRoot := []Mount{{Source: "tmpfs", Target: "/dev", FSType: "tmpfs", Root: "/"}}

	_, ok := backingMount("/opt", noRoot)
	assert.False(t, ok, "no mount covers /opt when the root itself is missing from the table")
	assert.Empty(t, ephemeralWriteablePaths([]string{"/opt"}, noRoot),
		"silence is recoverable; a wrong ephemerality claim is not")
}

// Disk-backed overlays are deliberately NOT ephemeral: an overlay's upper layer persists, and
// whether a container later discards it is a lifecycle fact this cannot attest.
func TestNonPersistentCoversMemoryAndSynthesisedFilesystems(t *testing.T) {
	for fs, want := range map[string]bool{
		// memory-backed: the write happens, and dies with the mount
		"tmpfs":    true,
		"ramfs":    true,
		"devtmpfs": true,
		// kernel-synthesised: access(2) can answer yes while a create cannot succeed at all
		"proc":   true,
		"sysfs":  true,
		"cgroup": true,
		"bpf":    true,
		// real stores, where a write persists
		"ext4":    false,
		"overlay": false,
		"xfs":     false,
		"btrfs":   false,
		"9p":      false,
	} {
		assert.Equal(t, want, nonPersistentFS(fs), "fstype %q", fs)
	}
}

// /proc is reported writeable when the scan runs as root, because isWritable trusts access(2) for
// directories and never attempts a create. It must not count as persistent write access: procfs
// has no backing store, so the create it implies would fail.
func TestProcIsNotPersistentWriteAccess(t *testing.T) {
	mounts := []Mount{
		{Source: "/dev/root", Target: "/", FSType: "ext4", Root: "/"},
		{Source: "proc", Target: "/proc", FSType: "proc", Root: "/"},
	}
	assert.Equal(t, []string{"/proc"}, ephemeralWriteablePaths([]string{"/etc", "/proc"}, mounts),
		"/etc persists and /proc cannot")
}
