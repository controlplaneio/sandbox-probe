package tasks

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	FAKE_API_KEY_PREFIX = "sk-ant-api03-"
)

func Test_detectSensitiveEnvVars(t *testing.T) {
	UNSAFEVARKEY := "test_detect_sensitive_env_vars_sensitive"
	SAFEVARKEY := "test_detect_sensitive_env_vars_safe"

	require.NoError(t, os.Setenv(UNSAFEVARKEY, fmt.Sprintf("%s%sAA", FAKE_API_KEY_PREFIX, strings.Repeat("X", 93))))
	require.NoError(t, os.Setenv(SAFEVARKEY, "unharmful_string"))

	defer func() {
		require.NoError(t, os.Unsetenv(UNSAFEVARKEY))
		require.NoError(t, os.Unsetenv(SAFEVARKEY))
	}()

	findings, err := detectSensitiveEnvVars()
	require.NoError(t, err)

	found_sensitive := false
	found_safe := false
	for _, finding := range findings {
		if finding.EnvKey == UNSAFEVARKEY {
			found_sensitive = true
		}
		if finding.EnvKey == SAFEVARKEY {
			found_safe = true
		}
	}

	assert.False(t, found_safe, "Safe environment variable reported when it should not be")
	assert.True(t, found_sensitive, "Sensitive environment variable not reported")
}

func Test_getUserGroupInfo(t *testing.T) {
	identity, err := GetUserGroupInfo()

	if runtime.GOOS == "windows" {
		// On Windows, should return error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not supported on Windows")
		assert.Nil(t, identity)
		return
	}

	require.NoError(t, err)
	require.NotNil(t, identity)

	assert.GreaterOrEqual(t, identity.UID, 0, "UID should be non-negative")
	assert.GreaterOrEqual(t, identity.GID, 0, "GID should be non-negative")
	assert.GreaterOrEqual(t, identity.EUID, 0, "EUID should be non-negative")
	assert.GreaterOrEqual(t, identity.EGID, 0, "EGID should be non-negative")
	assert.NotNil(t, identity.Groups, "Groups should not be nil")
}

func Test_getBubbleWrap(t *testing.T) {
	hasBubbleWrap, err := GetBubbleWrap(os.Getpid())
	require.NoError(t, err)

	t.Logf("BubbleWrap detected: %v", hasBubbleWrap)
}

func Test_getContainerRuntime(t *testing.T) {
	type args struct {
		runtimeStr string
	}
	type want struct {
		runtime ContainerRuntime
	}
	for _, tt := range []struct {
		name string
		args args
		want want
	}{
		{
			name: "docker test",
			args: args{
				runtimeStr: "docker",
			},
			want: want{
				runtime: RuntimeDocker,
			},
		},
		{
			name: "podman test",
			args: args{
				runtimeStr: "podman",
			},
			want: want{
				runtime: RuntimePodman,
			},
		},
		{
			name: "lxc test",
			args: args{
				runtimeStr: "lxc",
			},
			want: want{
				runtime: RuntimeLXC,
			},
		},
		{
			name: "firejail test",
			args: args{
				runtimeStr: "firejail",
			},
			want: want{
				runtime: RuntimeFirejail,
			},
		},
		{
			name: "systemd-nspawn test",
			args: args{
				runtimeStr: "systemd-nspawn",
			},
			want: want{
				runtime: RuntimeNspawn,
			},
		},
		{
			name: "unknown test",
			args: args{
				runtimeStr: "unknown",
			},
			want: want{
				runtime: RuntimeUnknown,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			readFile = func(path string) ([]byte, error) {
				switch path {
				case "/proc/self/cgroup":
					return []byte(tt.args.runtimeStr), nil
				case "/proc/1/cmdline":
					return []byte(""), nil
				default:
					return nil, fmt.Errorf("file not found")
				}
			}
			fileExistsFunc = func(path string) bool { return false }
			probeForLandlock = func() (bool, error) { return false, nil }
			origChroot := isChroot
			isChroot = func() bool { return false }
			t.Cleanup(func() { probeForLandlock = probeForLandlockImpl; isChroot = origChroot })

			runtime := GetContainerRuntime(0, 0)
			if runtime != tt.want.runtime {
				t.Errorf("expected RuntimeDocker, got %v", runtime)
			}
		})
	}
}

func TestGetContainerRuntimeAppArmor(t *testing.T) {
	origRead, origAttr, origExists, origLandlock, origChroot := readFile, readProcAttr, fileExistsFunc, probeForLandlock, isChroot
	t.Cleanup(func() {
		readFile, readProcAttr, fileExistsFunc, probeForLandlock, isChroot = origRead, origAttr, origExists, origLandlock, origChroot
	})
	readFile = func(string) ([]byte, error) { return nil, fmt.Errorf("file not found") }
	probeForLandlock = func() (bool, error) { return false, nil }
	isChroot = func() bool { return false }

	for _, tt := range []struct {
		name         string
		apparmorAttr string // /proc/self/attr/apparmor/current (LSM-specific)
		legacyAttr   string // /proc/self/attr/current (LSM-agnostic)
		apparmorLSM  bool   // /sys/module/apparmor present
		wantApparmor bool
	}{
		// LSM-specific path is unambiguous — trusted regardless of the module check.
		{"apparmor-specific profile", "sandbox-probe (complain)", "", false, true},
		{"unconfined", "unconfined", "unconfined", true, false},
		// Legacy path only trusted when AppArmor is the active LSM (old kernels).
		{"legacy profile, apparmor LSM", "", "sandbox-probe (enforce)", true, true},
		// SELinux writes its context to the same legacy path — must NOT be read as apparmor.
		{"selinux context, no apparmor LSM", "", "system_u:system_r:httpd_t:s0", false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fileExistsFunc = func(p string) bool { return tt.apparmorLSM && p == "/sys/module/apparmor" }
			readProcAttr = func(path string) string {
				switch path {
				case "/proc/self/attr/apparmor/current":
					return tt.apparmorAttr
				case "/proc/self/attr/current":
					return tt.legacyAttr
				}
				return ""
			}
			if got := GetContainerRuntime(0, 0) == RuntimeAppArmor; got != tt.wantApparmor {
				t.Errorf("apparmor=%v, want %v", got, tt.wantApparmor)
			}
		})
	}
}

// The /run/systemd/container marker must name nspawn but must NOT mask a lower detector when the
// marker is empty or unrecognised (the bug fixed by returning early only on a *named* runtime).
func TestGetContainerRuntimeSystemdContainerMarker(t *testing.T) {
	origRead, origAttr, origExists, origLandlock, origChroot := readFile, readProcAttr, fileExistsFunc, probeForLandlock, isChroot
	t.Cleanup(func() {
		readFile, readProcAttr, fileExistsFunc, probeForLandlock, isChroot = origRead, origAttr, origExists, origLandlock, origChroot
	})
	fileExistsFunc = func(string) bool { return false }
	probeForLandlock = func() (bool, error) { return false, nil }

	for _, tt := range []struct {
		name        string
		marker      string // /run/systemd/container contents ("" = file absent)
		apparmor    bool   // whether a named AppArmor profile is present
		wantRuntime ContainerRuntime
	}{
		{"nspawn marker names nspawn", "systemd-nspawn\n", false, RuntimeNspawn},
		{"empty marker does not mask apparmor", "", true, RuntimeAppArmor},
		{"unknown marker does not mask apparmor", "some-manager\n", true, RuntimeAppArmor},
		// An unrecognised marker (proot/rkt/…) still means "containerised" — preserve the signal.
		{"unknown marker still reports unknown", "proot\n", false, RuntimeUnknown},
	} {
		t.Run(tt.name, func(t *testing.T) {
			isChroot = func() bool { return false }
			readFile = func(path string) ([]byte, error) {
				if path == "/run/systemd/container" && tt.marker != "" {
					return []byte(tt.marker), nil
				}
				return nil, fmt.Errorf("file not found")
			}
			readProcAttr = func(string) string {
				if tt.apparmor {
					return "sandbox-probe (complain)"
				}
				return ""
			}
			if got := GetContainerRuntime(0, 0); got != tt.wantRuntime {
				t.Errorf("marker=%q apparmor=%v: got %v, want %v", tt.marker, tt.apparmor, got, tt.wantRuntime)
			}
		})
	}
}
