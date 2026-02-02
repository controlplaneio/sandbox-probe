package tasks

import (
	"fmt"
	"os"
	"strings"

	"github.com/controlplaneio/sandbox-probe/pkg/models"
	"github.com/prometheus/procfs"
	"github.com/rs/zerolog/log"
)

// fileExists checks if a file or directory exists
var fileExistsFunc = func(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func getCommand(proc procfs.Proc) (string, error) {
	cmdline, err := proc.CmdLine()
	if len(cmdline) < 1 || err != nil {
		return proc.Comm()
	}
	return strings.Join(cmdline, " "), nil
}

func GetRunningProcess(pid int) (*models.Process, error) {
	return getRunningProcessCommandLinux(pid)
}

func GetRunningParentProcess(pid int) (*models.Process, error) {
	return getRunningParentProcessLinux(pid)
}

func GetRunningProcesses() ([]*models.Process, error) {
	// ignore error
	return getRunningProcessesCommandsLinux()
}

func getRunningProcessLinux(pid int) (procfs.Proc, error) {
	fs, err := procfs.NewFS("/proc")
	if err != nil {
		log.Warn().Msgf("failed to access /proc: %s", err)
		return procfs.Proc{}, fmt.Errorf("failed to access to /proc: %w", err)
	}
	return fs.Proc(pid)
}

func getRunningProcessCommandLinux(pid int) (*models.Process, error) {
	proc, err := getRunningProcessLinux(pid)
	if err != nil {
		return nil, err
	}
	cmd, err := getCommand(proc)
	if err != nil {
		return nil, err
	}

	namespaces, ppid, err := getNamespacesAndParentProcess(proc)
	if err != nil {
		return nil, err
	}

	return &models.Process{
		Command:    cmd,
		PID:        pid,
		PPID:       ppid,
		Namespaces: namespaces,
	}, nil
}

func getNamespacesAndParentProcess(proc procfs.Proc) ([]*models.Namespace, int, error) {
	var namespaces []*models.Namespace
	procfsNamespaces, err := proc.Namespaces()
	if err != nil {
		log.Info().Err(err).Msgf("Couldn't get namespaces for process %d", proc.PID)
	} else {
		for _, ns := range procfsNamespaces {
			namespaces = append(namespaces, &models.Namespace{
				Type:  ns.Type,
				Inode: ns.Inode,
			})
		}
	}
	// don't return error if cannot find parent id
	status, err := proc.Stat()
	if err != nil {
		return namespaces, -1, nil
	}
	return namespaces, status.PPID, nil
}

func getRunningParentProcessLinux(pid int) (*models.Process, error) {
	proc, err := getRunningProcessLinux(pid)
	if err != nil {
		return nil, err
	}
	status, err := proc.Stat()
	if err != nil {
		return nil, err
	}

	return getRunningProcessCommandLinux(status.PPID)
}

func getRunningProcessesCommandsLinux() ([]*models.Process, error) {
	procs, err := getRunningProcessesLinux()
	if err != nil {
		log.Info().Err(err).Msg("Couldn't get running processes in linux")
		return nil, err
	}
	var processes []*models.Process

	for _, proc := range procs {
		cmd, err := getCommand(proc)
		if err != nil {
			log.Warn().Err(err).Msg("error getting process command line")
			continue
		}
		namespaces, ppid, err := getNamespacesAndParentProcess(proc)
		if err != nil {
			log.Warn().Err(err).Msg("error getting process namespaces")
		}
		processes = append(processes, &models.Process{
			Command:    cmd,
			PID:        proc.PID,
			PPID:       ppid,
			Namespaces: namespaces,
		})
	}

	return processes, nil
}

func getRunningProcessesLinux() ([]procfs.Proc, error) {
	fs, err := procfs.NewFS("/proc")
	if err != nil {
		log.Warn().Msgf("failed to access /proc: %s", err)
		return []procfs.Proc{}, fmt.Errorf("failed to access to /proc: %w", err)
	}
	return fs.AllProcs()
}
