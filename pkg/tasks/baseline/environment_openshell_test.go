package tasks

import (
	"fmt"
	"slices"
	"testing"
)

// TestGetContainerRuntimeOpenShell pins the NVIDIA OpenShell / NemoClaw fingerprints and their
// precedence. An OpenShell sandbox always sits inside a docker/podman container or a k3s pod, so
// its evidence must outrank the cgroup and the /.dockerenv and /run/.containerenv markers; and a
// NemoClaw marker directory only names NemoClaw when OpenShell itself is proven.
func TestGetContainerRuntimeOpenShell(t *testing.T) {
	origRead, origAttr, origExists, origLandlock, origChroot, origEnv := readFile, readProcAttr, fileExistsFunc, probeForLandlock, isChroot, lookupEnv
	t.Cleanup(func() {
		readFile, readProcAttr, fileExistsFunc, probeForLandlock, isChroot, lookupEnv = origRead, origAttr, origExists, origLandlock, origChroot, origEnv
	})
	readProcAttr = func(string) string { return "" }
	probeForLandlock = func() (bool, error) { return false, nil }
	isChroot = func() bool { return false }

	for _, tt := range []struct {
		name   string
		env    map[string]string
		files  []string
		cgroup string
		want   ContainerRuntime
	}{
		{name: "env only", env: map[string]string{"OPENSHELL_SANDBOX": "1"}, want: RuntimeOpenShell},
		{name: "env holding a sandbox name (older releases)", env: map[string]string{"OPENSHELL_SANDBOX": "my-sandbox"}, want: RuntimeOpenShell},
		{name: "env set but empty is absent", env: map[string]string{"OPENSHELL_SANDBOX": ""}, want: RuntimeNotFound},
		{name: "CA file only", files: []string{openShellCAFiles[0]}, want: RuntimeOpenShell},
		{name: "v0.1.4 supervisor CA only", files: []string{"/run/openshell-supervisor-ca/material/ca.crt"}, want: RuntimeOpenShell},
		{name: "env beats docker cgroup and /.dockerenv", env: map[string]string{"OPENSHELL_SANDBOX": "1"}, files: []string{"/.dockerenv"}, cgroup: "0::/docker/abc123", want: RuntimeOpenShell},
		{name: "CA file beats /run/.containerenv", files: []string{openShellCAFiles[0], "/run/.containerenv"}, want: RuntimeOpenShell},
		{name: "env plus NemoClaw lib dir", env: map[string]string{"OPENSHELL_SANDBOX": "1"}, files: []string{"/usr/local/lib/nemoclaw"}, want: RuntimeNemoClaw},
		{name: "CA file plus /opt/nemoclaw inside docker", files: []string{openShellCAFiles[0], "/opt/nemoclaw", "/.dockerenv"}, cgroup: "0::/docker/abc123", want: RuntimeNemoClaw},
		{name: "NemoClaw dir alone is not NemoClaw", files: []string{"/usr/local/lib/nemoclaw"}, want: RuntimeNotFound},
		{name: "NemoClaw dir in docker stays docker", files: []string{"/usr/local/lib/nemoclaw", "/.dockerenv"}, want: RuntimeDocker},
		{name: "nothing", want: RuntimeNotFound},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lookupEnv = func(k string) (string, bool) { v, ok := tt.env[k]; return v, ok }
			fileExistsFunc = func(p string) bool { return slices.Contains(tt.files, p) }
			readFile = func(p string) ([]byte, error) {
				if p == "/proc/self/cgroup" && tt.cgroup != "" {
					return []byte(tt.cgroup), nil
				}
				return nil, fmt.Errorf("file not found")
			}
			if got := GetContainerRuntime(0, 0); got != tt.want {
				t.Fatalf("GetContainerRuntime() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestStringToContainerRuntimeOpenShell covers the ancestor-walk fingerprint: the supervisor's
// binary name in a command line, matched ahead of the generic keywords it may share a line with.
func TestStringToContainerRuntimeOpenShell(t *testing.T) {
	for _, s := range []string{
		"/usr/local/bin/openshell-sandbox --policy /etc/openshell/policy.yaml",
		"openshell-supervisor",
		"openshell-sandbox --driver docker",
	} {
		if got := stringToContainerRuntime(s); got != RuntimeOpenShell {
			t.Errorf("stringToContainerRuntime(%q) = %v, want RuntimeOpenShell", s, got)
		}
	}
	if got := stringToContainerRuntime("openshell"); got == RuntimeOpenShell {
		t.Errorf("bare CLI name %q must not match the supervisor fingerprint", "openshell")
	}
}
