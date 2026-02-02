package tasks

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_getRunningProcess(t *testing.T) {
	// Test with current process
	pid := os.Getpid()
	cmdLine, err := GetRunningProcess(pid)

	require.NoError(t, err, "getRunningProcess should not return error for current process")
	assert.NotEmpty(t, cmdLine, "Command line should not be empty for current process")

	t.Logf("Current process (PID %d) command: %v", pid, cmdLine)
}

func Test_getRunningParentProcess(t *testing.T) {
	// Test with current process to get its parent
	pid := os.Getpid()
	parentCmd, err := GetRunningParentProcess(pid)

	require.NoError(t, err, "getRunningParentProcess should not return error")
	assert.NotEmpty(t, parentCmd, "Parent process command should not be empty")

	t.Logf("Parent process of PID %d: %v", pid, parentCmd)
}

func Test_getRunningProcesses(t *testing.T) {
	// Test getting all running processes
	processes, err := GetRunningProcesses()

	require.NoError(t, err, "getRunningProcesses should not return error")
	assert.NotEmpty(t, processes, "Should return at least some processes")

	t.Logf("Found %d running processes", len(processes))

	// Log first few processes
	maxLog := 5
	if len(processes) < maxLog {
		maxLog = len(processes)
	}
	for i := 0; i < maxLog; i++ {
		if len(processes[i]) > 0 {
			t.Logf("Process %d: %v", i, processes[i])
		}
	}
}
func Test_getRunningParentProcessLinux(t *testing.T) {
	// Test getting parent process using ps command
	pid := os.Getpid()
	parentCmd, _, err := getRunningParentProcessLinux(pid)

	require.NoError(t, err, "getRunningParentProcessLinux should not return error")
	assert.NotEmpty(t, parentCmd, "Parent process command should not be empty")

	t.Logf("Parent of PID %d using ps: %v", pid, parentCmd)
}

func TestStringToContainerRuntime(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ContainerRuntime
	}{
		{
			name:     "docker keyword",
			input:    "1:name=systemd:/docker/abc123",
			expected: RuntimeDocker,
		},
		{
			name:     "podman keyword",
			input:    "1:name=systemd:/podman/xyz789",
			expected: RuntimePodman,
		},
		{
			name:     "lxc keyword",
			input:    "1:name=systemd:/lxc/container",
			expected: RuntimeLXC,
		},
		{
			name:     "firejail keyword",
			input:    "/usr/bin/firejail --profile=default",
			expected: RuntimeFirejail,
		},
		{
			name:     "no container",
			input:    "1:name=systemd:/user.slice/user-1000.slice",
			expected: RuntimeNotFound,
		},
		{
			name:     "empty string",
			input:    "",
			expected: RuntimeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringToContainerRuntime(tt.input)
			assert.Equal(t, tt.expected, result,
				"stringToContainerRuntime(%q) = %v, expected %v",
				tt.input, result, tt.expected)
		})
	}
}
