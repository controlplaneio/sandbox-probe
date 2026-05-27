//go:build linux
// +build linux

package tasks

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_getRunningProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping process tests in short mode")
	}

	pid := os.Getpid()
	proc, err := GetRunningProcess(pid)

	require.NoError(t, err, "getRunningProcess should not return error for current process")
	require.NotNil(t, proc, "Process should not be nil")
	assert.NotEmpty(t, proc.Command, "Command line should not be empty for current process")

	t.Logf("Current process (PID %d) command: %v", pid, proc.Command)
}

func Test_getRunningParentProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping process tests in short mode")
	}

	pid := os.Getpid()
	parentProc, err := GetRunningParentProcess(pid)

	require.NoError(t, err, "getRunningParentProcess should not return error")
	require.NotNil(t, parentProc, "Parent process should not be nil")
	assert.NotEmpty(t, parentProc.Command, "Parent process command should not be empty")

	t.Logf("Parent process of PID %d: %v", pid, parentProc.Command)
}

func Test_getRunningProcesses(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping process tests in short mode")
	}

	processes, err := GetRunningProcesses()

	require.NoError(t, err, "getRunningProcesses should not return error")
	assert.NotEmpty(t, processes, "Should return at least some processes")

	t.Logf("Found %d running processes", len(processes))

	maxLog := 5
	if len(processes) < maxLog {
		maxLog = len(processes)
	}
	for i := 0; i < maxLog; i++ {
		t.Logf("Process %d (PID %d): %v", i, processes[i].PID, processes[i].Command)
	}
}

func Test_getRunningParentProcessLinux(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping process tests in short mode")
	}

	pid := os.Getpid()
	parentProc, err := getRunningParentProcessLinux(pid)

	require.NoError(t, err, "getRunningParentProcessLinux should not return error")
	require.NotNil(t, parentProc, "Parent process should not be nil")
	assert.NotEmpty(t, parentProc.Command, "Parent process command should not be empty")

	t.Logf("Parent of PID %d: %v", pid, parentProc.Command)
}
