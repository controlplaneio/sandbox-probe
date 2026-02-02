package tasks

import (
	"fmt"
	"os/exec"
)

type cmdProbe[T any] interface {
	getCommand() ([]string, error)
	parseCommandOuput([]byte) (T, error)
}

var execCommand = func(cmd string, args ...string) ([]byte, error) {
	return exec.Command(cmd, args...).Output()
}

func runCmdProbe[T any](probe cmdProbe[T]) (T, error) {
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
