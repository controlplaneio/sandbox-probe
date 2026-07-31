package tasks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prometheus/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readMountFixture points readFile at a mount table captured from a real runtime, so the
// enumerator is exercised against that capture rather than the host the test runs on.
// See testdata/mountinfo/README.md for how each fixture was captured.
func readMountFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "mountinfo", name+".mountinfo"))
	require.NoError(t, err)

	orig := readFile
	readFile = func(path string) ([]byte, error) {
		require.Equal(t, procMountInfo, path)
		return data, nil
	}
	t.Cleanup(func() { readFile = orig })

	return data
}

func Test_GetHostMounts(t *testing.T) {
	for _, tt := range []struct {
		name    string
		fixture string
		want    []Mount
	}{
		{
			// gVisor names every mount's source "none", so nothing survives the filter — not even
			// the /hostbind bind of a host directory. See Test_GetHostMountsGvisorBindRejected.
			name:    "gvisor sandbox with a bind mount reports nothing",
			fixture: "gvisor-bind",
			want:    nil,
		},
		{
			// The bind is reported because Docker Desktop names its source as a path; the
			// container's own overlay root and the pseudo-filesystems are dropped.
			name:    "docker container with an explicit bind",
			fixture: "docker-bind",
			want: []Mount{
				{Source: "/run/host_mark/private", Target: "/mnt/hostbind", FSType: "fakeowner"},
				{Source: "/dev/vda1", Target: "/etc/resolv.conf", FSType: "ext4"},
				{Source: "/dev/vda1", Target: "/etc/hostname", FSType: "ext4"},
				{Source: "/dev/vda1", Target: "/etc/hosts", FSType: "ext4"},
			},
		},
		{
			// Unconfined host: ordinary filesystems are reported, pseudo-filesystems are not. The
			// virtiofs shares of the macOS host (sources "virtiofs0"…) are dropped for the same
			// reason the gVisor bind is — their source is not spelled as a path.
			name:    "unconfined host",
			fixture: "unconfined-host",
			want: []Mount{
				{Source: "/dev/root", Target: "/oldroot", FSType: "erofs"},
				{Source: "fusectl", Target: "/sys/fs/fuse/connections", FSType: "fusectl"},
				{Source: "/run/host_virtiofs/Users", Target: "/run/host_mark/Users", FSType: "fakeowner"},
				{Source: "/run/host_mark/Users", Target: "/host_mnt/Users", FSType: "fakeowner"},
				{Source: "/run/host_virtiofs/Volumes", Target: "/run/host_mark/Volumes", FSType: "fakeowner"},
				{Source: "/run/host_mark/Volumes", Target: "/host_mnt/Volumes", FSType: "fakeowner"},
				{Source: "/run/host_virtiofs/private", Target: "/run/host_mark/private", FSType: "fakeowner"},
				{Source: "/run/host_mark/private", Target: "/host_mnt/private", FSType: "fakeowner"},
				{Source: "/run/host_virtiofs/tmp", Target: "/run/host_mark/tmp", FSType: "fakeowner"},
				{Source: "/run/host_mark/tmp", Target: "/host_mnt/tmp", FSType: "fakeowner"},
				{Source: "/run/host_virtiofs/var/folders", Target: "/run/host_mark/var/folders", FSType: "fakeowner"},
				{Source: "/run/host_mark/var/folders", Target: "/host_mnt/var/folders", FSType: "fakeowner"},
				{Source: "/dev/vda1", Target: "/var/lib", FSType: "ext4"},
				{Source: "rawBridge", Target: "/run/jfs", FSType: "fuse.rawBridge"},
				{Source: "/var/lib/mutagen/file-shares", Target: "/run/mutagen-file-shares-mark", FSType: "selfowner"},
				{Source: "/run/mutagen-file-shares-mark", Target: "/run/mutagen-file-shares", FSType: "selfowner"},
				{Source: "rosetta-mount", Target: "/run/rosetta", FSType: "fuse.rosetta-mount"},
				{Source: "/dev/vda1", Target: "/var/lib/docker", FSType: "ext4"},
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

// The bug this fixture pins: a bind of a host directory into a gVisor sandbox is reachable from
// inside and is not enumerated. This records which clause of the current filter rejects it, rather
// than guessing — it is the source-shape heuristic, not the pseudo-filesystem exclusion.
func Test_GetHostMountsGvisorBindRejected(t *testing.T) {
	data := readMountFixture(t, "gvisor-bind")

	var bind *procfs.MountInfo
	for _, m := range parseMountInfo(data) {
		if m.MountPoint == "/hostbind" {
			bind = m
		}
	}
	require.NotNil(t, bind, "fixture must contain the known bind mount")
	assert.False(t, isLikelyHostMount(bind), "the bind is dropped by the filter")

	// It is not the fstype switch: 9p is not one of the excluded pseudo-filesystems.
	assert.Equal(t, "9p", bind.FSType)
	// It is the source-shape clause: gVisor reports "none" as the source of every mount, so
	// neither "source starts with /" nor "fstype contains fuse" matches.
	assert.Equal(t, "none", bind.Source)
	assert.False(t, strings.HasPrefix(bind.Source, "/"))
	assert.NotContains(t, bind.FSType, "fuse")

	mounts, err := GetHostMounts()
	assert.NoError(t, err)
	assert.NotContains(t, mounts, Mount{Source: bind.Source, Target: "/hostbind", FSType: bind.FSType})
}
