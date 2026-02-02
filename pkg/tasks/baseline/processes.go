package tasks

import (
	"fmt"
	"os"

	"github.com/prometheus/procfs"
	"github.com/rs/zerolog/log"
)

// fileExists checks if a file or directory exists
var fileExistsFunc = func(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func GetRunningProcess(pid int) ([]string, error) {
	return getRunningProcessCommandLinux(pid)
}

func GetRunningParentProcess(pid int) ([]string, error) {
	ppLinux, _, err := getRunningParentProcessLinux(pid)
	if err != nil {
		return []string{}, err
	}
	return ppLinux, nil
}

func GetRunningProcesses() ([][]string, error) {
	// ignore error
	cmdLinux, _ := getRunningProcessesCommandsLinux()

	return cmdLinux, nil
}

func getRunningProcessLinux(pid int) (procfs.Proc, error) {
	fs, err := procfs.NewFS("/proc")
	if err != nil {
		log.Warn().Msgf("failed to access /proc: %s", err)
		return procfs.Proc{}, fmt.Errorf("failed to access to /proc: %w", err)
	}
	return fs.Proc(pid)
}

func getRunningProcessCommandLinux(pid int) ([]string, error) {
	proc, err := getRunningProcessLinux(pid)
	if err != nil {
		return []string{}, nil
	}
	return proc.CmdLine()
}

// -1 stands for not found
func getRunningParentProcessLinux(pid int) ([]string, int, error) {
	proc, err := getRunningProcessLinux(pid)
	if err != nil {
		return []string{}, -1, nil
	}
	status, err := proc.Stat()
	if err != nil {
		return []string{}, -1, err
	}

	cmd, err := getRunningProcessCommandLinux(status.PPID)
	return cmd, status.PPID, err
}

func getRunningProcessesCommandsLinux() ([][]string, error) {
	procs, err := getRunningProcessesLinux()
	if err != nil {
		return [][]string{}, nil
	}
	var commands [][]string

	for _, proc := range procs {
		cmd, err := proc.CmdLine()
		if err != nil {
			return [][]string{}, err
		}
		commands = append(commands, cmd)
	}

	return commands, nil
}

func getRunningProcessesLinux() ([]procfs.Proc, error) {
	fs, err := procfs.NewFS("/proc")
	if err != nil {
		log.Warn().Msgf("failed to access /proc: %s", err)
		return []procfs.Proc{}, fmt.Errorf("failed to access to /proc: %w", err)
	}
	return fs.AllProcs()
}
