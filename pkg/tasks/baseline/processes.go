package tasks

import (
	"os"

	"github.com/controlplaneio/sandbox-probe/pkg/models"
)

// fileExistsFunc checks if a file or directory exists.
// Defined as a var to allow overriding in tests.
var fileExistsFunc = func(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func GetRunningProcess(pid int) (*models.Process, error) {
	return getRunningProcessCommandLinux(pid)
}

func GetRunningParentProcess(pid int) (*models.Process, error) {
	return getRunningParentProcessLinux(pid)
}

func GetRunningProcesses() ([]*models.Process, error) {
	return getRunningProcessesCommandsLinux()
}
