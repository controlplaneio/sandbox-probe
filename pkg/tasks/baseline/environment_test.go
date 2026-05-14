package tasks

import (
	"fmt"
	"os"
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

			runtime := GetContainerRuntime(0, 0)
			if runtime != tt.want.runtime {
				t.Errorf("expected RuntimeDocker, got %v", runtime)
			}
		})
	}
}


