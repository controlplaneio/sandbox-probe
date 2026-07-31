//go:build linux
// +build linux

package tasks

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getHostMounts(t *testing.T) {
	mounts, err := GetHostMounts()
	t.Logf("Mounts found: %v", mounts)
	assert.NoError(t, err)
}

// TestGetContainerRuntimeUIDMap drives the /proc/self/uid_map fallback through the same readFile
// indirection as the AppArmor and systemd-container marker tables above it. Any map other than the
// init namespace's identity map of the whole uid range is a restricted user namespace; the last
// cases pin the ordering, where a more specific runtime's evidence must still outrank it.
func TestGetContainerRuntimeUIDMap(t *testing.T) {
	origRead, origAttr, origExists, origLandlock, origChroot := readFile, readProcAttr, fileExistsFunc, probeForLandlock, isChroot
	t.Cleanup(func() {
		readFile, readProcAttr, fileExistsFunc, probeForLandlock, isChroot = origRead, origAttr, origExists, origLandlock, origChroot
	})
	readProcAttr = func(string) string { return "" }
	fileExistsFunc = func(string) bool { return false }
	probeForLandlock = func() (bool, error) { return false, nil }
	isChroot = func() bool { return false }

	for _, tt := range []struct {
		name        string
		uidMap      string // /proc/self/uid_map contents ("" = absent/unreadable)
		cgroup      string // /proc/self/cgroup contents ("" = unreadable)
		marker      string // /run/systemd/container contents (defaults to an unnamed marker)
		env         string // the "container" environment variable
		wantRuntime ContainerRuntime
	}{
		// Identity map for the full range: the init namespace, so no user namespace and the
		// output is whatever an unconfined baseline reports.
		{name: "identity map", uidMap: "0 0 4294967295", wantRuntime: RuntimeUnknown},
		// The exact shape that caused the reported defect: bwrap invoked without remapping the
		// inner user to root keeps the invoking uid.
		{name: "map keeping invoking uid", uidMap: "1001 1001 1", wantRuntime: RuntimeBubblewrap},
		// Inner user remapped to root: detected before this rule was broadened, and still is.
		{name: "map remapping inner user to root", uidMap: "0 1001 1", wantRuntime: RuntimeBubblewrap},
		// Identity mapping of a truncated range is still a restricted namespace.
		{name: "identity map of a truncated range", uidMap: "0 0 1000", wantRuntime: RuntimeBubblewrap},
		{name: "absent or unreadable map", uidMap: "", wantRuntime: RuntimeUnknown},
		// Ordering: a specific runtime's evidence alongside a restricted map still wins, so the
		// broadened rule cannot relabel a rootless container or an nspawn run as bubblewrap.
		{name: "cgroup string outranks restricted map", uidMap: "1001 1001 1", cgroup: "0::/docker/abc", wantRuntime: RuntimeDocker},
		{name: "container marker outranks restricted map", uidMap: "1001 1001 1", marker: "systemd-nspawn", wantRuntime: RuntimeNspawn},
		{name: "container env var outranks restricted map", uidMap: "1001 1001 1", env: "podman", wantRuntime: RuntimePodman},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				t.Setenv("container", tt.env)
			}
			readFile = func(path string) ([]byte, error) {
				switch path {
				case "/proc/self/uid_map":
					if tt.uidMap == "" {
						return nil, fmt.Errorf("file not found")
					}
					return []byte(tt.uidMap), nil
				case "/proc/self/cgroup":
					if tt.cgroup == "" {
						return nil, fmt.Errorf("file not found")
					}
					return []byte(tt.cgroup), nil
				case "/run/systemd/container":
					// An unnamed marker forces identifiedRuntime to RuntimeUnknown so the final
					// no-new-privs fallback is deterministic regardless of the real process's
					// NoNewPrivs bit on the machine running the test.
					if tt.marker == "" {
						return []byte("some-manager\n"), nil
					}
					return []byte(tt.marker + "\n"), nil
				default:
					return nil, fmt.Errorf("file not found")
				}
			}
			if got := GetContainerRuntime(0, 0); got != tt.wantRuntime {
				t.Errorf("uid_map=%q: got %v, want %v", tt.uidMap, got, tt.wantRuntime)
			}
		})
	}
}

// TestActiveMechanismsUserNamespace pins that "user-namespace" appears in the mechanism list exactly
// when the uid_map is non-identity, regardless of whether the wrapper name resolved to bubblewrap or
// stayed unknown — the mechanism is kernel-attested and separate from the wrapper inference.
func TestActiveMechanismsUserNamespace(t *testing.T) {
	origRead := readFile
	t.Cleanup(func() { readFile = origRead })

	for _, tt := range []struct {
		name   string
		uidMap string // "" = absent/unreadable
		want   bool
	}{
		{name: "identity map: no user namespace", uidMap: "0 0 4294967295", want: false},
		{name: "map keeping invoking uid: user namespace", uidMap: "1001 1001 1", want: true},
		{name: "map remapping inner user to root: user namespace", uidMap: "0 1001 1", want: true},
		{name: "absent or unreadable map: no user namespace", uidMap: "", want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			readFile = func(path string) ([]byte, error) {
				if path == "/proc/self/uid_map" {
					if tt.uidMap == "" {
						return nil, fmt.Errorf("file not found")
					}
					return []byte(tt.uidMap), nil
				}
				return nil, fmt.Errorf("file not found")
			}

			mechanisms := ActiveMechanisms()
			found := false
			for _, m := range mechanisms {
				if m == "user-namespace" {
					found = true
				}
			}
			if found != tt.want {
				t.Errorf("uid_map=%q: user-namespace present=%v, want %v (mechanisms=%v)", tt.uidMap, found, tt.want, mechanisms)
			}
		})
	}
}
