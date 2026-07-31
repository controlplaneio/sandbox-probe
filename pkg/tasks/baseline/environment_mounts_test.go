package tasks

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mountFixtures are every captured mount table under testdata/mountinfo.
var mountFixtures = []string{"gvisor-bind", "docker-bind", "unconfined-host"}

// stubMountTable points readFile at content of the test's choosing, the same indirection the
// runtime-detection chain uses.
func stubMountTable(t *testing.T, read func() ([]byte, error)) {
	t.Helper()
	orig := readFile
	readFile = func(path string) ([]byte, error) {
		require.Equal(t, procMountInfo, path)
		return read()
	}
	t.Cleanup(func() { readFile = orig })
}

// readMountFixture points readFile at a mount table captured from a real runtime, so the
// enumerator is exercised against that capture rather than the host the test runs on.
// See testdata/mountinfo/README.md for how each fixture was captured.
func readMountFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "mountinfo", name+".mountinfo"))
	require.NoError(t, err)

	stubMountTable(t, func() ([]byte, error) { return data, nil })

	return data
}

func Test_GetHostMounts(t *testing.T) {
	for _, tt := range []struct {
		name    string
		fixture string
		want    []Mount
	}{
		{
			// The case that caused the bug: gVisor names every mount's source "none", so a filter
			// keyed on the source's shape dropped the /hostbind bind of a host directory along with
			// everything else. Nothing here looks at the source, so the bind is reported.
			name:    "gvisor sandbox with a bind mount",
			fixture: "gvisor-bind",
			want: []Mount{
				{Source: "none", Target: "/", FSType: "9p"},
				{Source: "none", Target: "/dev", FSType: "dev"},
				{Source: "none", Target: "/hostbind", FSType: "9p"},
				{Source: "none", Target: "/tmp", FSType: "tmpfs"},
			},
		},
		{
			// The explicit bind at /mnt/hostbind is reported, and so are the container's own overlay
			// root and its tmpfs mounts — neither is a kernel pseudo-filesystem, and treating them
			// as the sandbox's own storage is a guess about intent rather than a fact.
			name:    "docker container with an explicit bind",
			fixture: "docker-bind",
			want: []Mount{
				{Source: "overlay", Target: "/", FSType: "overlay"},
				{Source: "tmpfs", Target: "/dev", FSType: "tmpfs"},
				{Source: "shm", Target: "/dev/shm", FSType: "tmpfs"},
				{Source: "/run/host_mark/private", Target: "/mnt/hostbind", FSType: "fakeowner"},
				{Source: "/dev/vda1", Target: "/etc/resolv.conf", FSType: "ext4"},
				{Source: "/dev/vda1", Target: "/etc/hostname", FSType: "ext4"},
				{Source: "/dev/vda1", Target: "/etc/hosts", FSType: "ext4"},
				{Source: "tmpfs", Target: "/proc/interrupts", FSType: "tmpfs"},
				{Source: "tmpfs", Target: "/proc/kcore", FSType: "tmpfs"},
				{Source: "tmpfs", Target: "/proc/keys", FSType: "tmpfs"},
				{Source: "tmpfs", Target: "/proc/scsi", FSType: "tmpfs"},
				{Source: "tmpfs", Target: "/proc/timer_list", FSType: "tmpfs"},
				{Source: "tmpfs", Target: "/sys/firmware", FSType: "tmpfs"},
			},
		},
		{
			// Unconfined host: the virtiofs shares of the macOS host are reported despite sources
			// ("virtiofs0"…) that are not spelled as paths, which the previous filter required.
			name:    "unconfined host",
			fixture: "unconfined-host",
			want: []Mount{
				{Source: "/dev/root", Target: "/oldroot", FSType: "erofs"},
				{Source: "devtmpfs", Target: "/oldroot/dev", FSType: "devtmpfs"},
				{Source: "tmpfs", Target: "/oldroot/run", FSType: "tmpfs"},
				{Source: "overlay", Target: "/", FSType: "overlay"},
				{Source: "devtmpfs", Target: "/dev", FSType: "devtmpfs"},
				{Source: "tmpfs", Target: "/run", FSType: "tmpfs"},
				{Source: "tmpfs", Target: "/var", FSType: "tmpfs"},
				{Source: "tmpfs", Target: "/host_mnt", FSType: "tmpfs"},
				{Source: "tmpfs", Target: "/tmp", FSType: "tmpfs"},
				{Source: "shm", Target: "/dev/shm", FSType: "tmpfs"},
				{Source: "fusectl", Target: "/sys/fs/fuse/connections", FSType: "fusectl"},
				{Source: "pstore", Target: "/sys/fs/pstore", FSType: "pstore"},
				{Source: "rpc_pipefs", Target: "/run/rpc_pipefs", FSType: "rpc_pipefs"},
				{Source: "binfmt_misc", Target: "/proc/sys/fs/binfmt_misc", FSType: "binfmt_misc"},
				{Source: "virtiofs0", Target: "/run/host_virtiofs/Users", FSType: "virtiofs.virtiofs0"},
				{Source: "/run/host_virtiofs/Users", Target: "/run/host_mark/Users", FSType: "fakeowner"},
				{Source: "/run/host_mark/Users", Target: "/host_mnt/Users", FSType: "fakeowner"},
				{Source: "virtiofs1", Target: "/run/host_virtiofs/Volumes", FSType: "virtiofs.virtiofs1"},
				{Source: "/run/host_virtiofs/Volumes", Target: "/run/host_mark/Volumes", FSType: "fakeowner"},
				{Source: "/run/host_mark/Volumes", Target: "/host_mnt/Volumes", FSType: "fakeowner"},
				{Source: "virtiofs2", Target: "/run/host_virtiofs/private", FSType: "virtiofs.virtiofs2"},
				{Source: "/run/host_virtiofs/private", Target: "/run/host_mark/private", FSType: "fakeowner"},
				{Source: "/run/host_mark/private", Target: "/host_mnt/private", FSType: "fakeowner"},
				{Source: "virtiofs3", Target: "/run/host_virtiofs/tmp", FSType: "virtiofs.virtiofs3"},
				{Source: "/run/host_virtiofs/tmp", Target: "/run/host_mark/tmp", FSType: "fakeowner"},
				{Source: "/run/host_mark/tmp", Target: "/host_mnt/tmp", FSType: "fakeowner"},
				{Source: "virtiofs4", Target: "/run/host_virtiofs/var/folders", FSType: "virtiofs.virtiofs4"},
				{Source: "/run/host_virtiofs/var/folders", Target: "/run/host_mark/var/folders", FSType: "fakeowner"},
				{Source: "/run/host_mark/var/folders", Target: "/host_mnt/var/folders", FSType: "fakeowner"},
				{Source: "ROSETTA", Target: "/run/rosetta.orig", FSType: "virtiofs"},
				{Source: "/dev/vda1", Target: "/var/lib", FSType: "ext4"},
				{Source: "rawBridge", Target: "/run/jfs", FSType: "fuse.rawBridge"},
				{Source: "/var/lib/mutagen/file-shares", Target: "/run/mutagen-file-shares-mark", FSType: "selfowner"},
				{Source: "/run/mutagen-file-shares-mark", Target: "/run/mutagen-file-shares", FSType: "selfowner"},
				{Source: "rosetta-mount", Target: "/run/rosetta", FSType: "fuse.rosetta-mount"},
				{Source: "/dev/vda1", Target: "/var/lib/docker", FSType: "ext4"},
				{Source: "overlay", Target: "/var/lib/docker/rootfs/overlayfs/62899e34e243848ccf609e29a8167f7f7582c8d560a1d321e6245169da79e92d", FSType: "overlay"},
				{Source: "overlay", Target: "/var/lib/docker/rootfs/overlayfs/bb4dac5d8a88ecd7964095b8d82bf4becbe689d27518393f0678bac55532742e", FSType: "overlay"},
				{Source: "overlay", Target: "/var/lib/docker/rootfs/overlayfs/6bacb8798411738535f8e306ce4cd6031cdef95280be837216076aaf766ca4e4", FSType: "overlay"},
				{Source: "overlay", Target: "/var/lib/docker/rootfs/overlayfs/971156221d61a0eaa614e1bcdafd3c6f33fc3ab54b3a6394addd0c47e07e2b33", FSType: "overlay"},
				{Source: "overlay", Target: "/var/lib/docker/rootfs/overlayfs/a434543d72b0c1811856a600dc1179beeb3186dc7a999fd4962034647e4191a2", FSType: "overlay"},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			readMountFixture(t, tt.fixture)

			got, err := GetHostMounts()
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// The regression this fixes: a bind of a host directory into a gVisor sandbox is reachable from
// inside and was not enumerated, so the Host mounts cell read blocked while the host filesystem was
// exposed. gVisor spells the source of every mount "none", including this one.
func Test_GetHostMountsGvisorBindReported(t *testing.T) {
	readMountFixture(t, "gvisor-bind")

	got, err := GetHostMounts()
	require.NoError(t, err)
	assert.Contains(t, got, Mount{Source: "none", Target: "/hostbind", FSType: "9p"})
}

// No fixture reports a kernel pseudo-filesystem, whichever runtime produced it.
func Test_GetHostMountsExcludesPseudoFilesystems(t *testing.T) {
	for _, fixture := range mountFixtures {
		t.Run(fixture, func(t *testing.T) {
			readMountFixture(t, fixture)

			got, err := GetHostMounts()
			require.NoError(t, err)
			require.NotEmpty(t, got)
			for _, m := range got {
				assert.False(t, isKernelPseudoFilesystem(m.FSType), "%s is a pseudo-filesystem", m.Target)
			}
		})
	}
}

// A restricted environment must not raise a spurious finding: no mounts and no error, so the task
// reports an empty list rather than failing.
func Test_GetHostMountsUnreadableTable(t *testing.T) {
	for _, tt := range []struct {
		name string
		read func() ([]byte, error)
	}{
		{"empty", func() ([]byte, error) { return nil, nil }},
		{"unreadable", func() ([]byte, error) { return nil, errors.New("permission denied") }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stubMountTable(t, tt.read)

			got, err := GetHostMounts()
			assert.NoError(t, err)
			assert.Empty(t, got)
		})
	}
}
