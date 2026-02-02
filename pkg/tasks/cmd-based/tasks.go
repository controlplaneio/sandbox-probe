package tasks

import (
	"fmt"
	"os/exec"
)

type CmdTask[T any] interface {
	getCommand() ([]string, error)
	parseCommandOuput([]byte) (T, error)
}

var execCommand = func(cmd string, args ...string) ([]byte, error) {
	return exec.Command(cmd, args...).Output()
}

func runCmdTask[T any](probe CmdTask[T]) (T, error) {
	var zero T
	fullCmd, err := probe.getCommand()
	if err != nil {
		return zero, err
	}
	var cmd string
	var args []string
	if len(fullCmd) < 1 {
		return zero, fmt.Errorf("returned command length is 0")
	}
	if len(fullCmd) > 2 {
		args = fullCmd[1:]
	}
	cmd = fullCmd[0]
	out, err := execCommand(cmd, args...)
	if err != nil {
		return zero, err
	}

	return probe.parseCommandOuput(out)
}

func RunPSAllRunningProcessesCmd() (*PSAllRunningProcessesCmd, error) {
	p, err := newPsAllRunningProcessesCmd()
	if err != nil {
		return nil, err
	}
	return runCmdTask[*PSAllRunningProcessesCmd](p)
}

func RunPSSingleRunningProcessCmd(pid int) (*PSSingleRunningProcessCmd, error) {
	p, err := newPsSingleRunningProcessCmd(pid)
	if err != nil {
		return nil, err
	}
	return runCmdTask[*PSSingleRunningProcessCmd](p)
}

func RunPSParentRunningProcessCmd(pid int) (*PSParentRunningProcessCmd, error) {
	p, err := newPSParentRunningProcessCmd(pid)
	if err != nil {
		return nil, err
	}
	return runCmdTask[*PSParentRunningProcessCmd](p)
}
