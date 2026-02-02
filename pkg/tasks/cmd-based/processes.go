package tasks

import (
	"errors"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

var (
	unexpectedLineErr  = errors.New("Unexpected format in command output line")
	commandNotFoundErr = errors.New("Command not found")
)

type command struct {
	pid     int
	ppid    int
	command []string
}

type psAllRunningProcessesProbe struct {
	commands []command
}

// returns a new instance of psAllRunningProcessesProbe
// TODO: add configurable options opts...
func newPsAllRunningProcessesProbe() (*psAllRunningProcessesProbe, error) {
	return &psAllRunningProcessesProbe{}, nil
}

type psSingleRunningProcessProbe struct {
	pid     int
	command command
}

// returns a new instance of psSingleRunningProcessProbe
// TODO: add configurable options opts...
func newPsSingleRunningProcessProbe(pid int) (*psSingleRunningProcessProbe, error) {
	return &psSingleRunningProcessProbe{
		pid: pid,
	}, nil
}

type psParentRunningProcessProbe struct {
	pid     int
	command command
}

// returns a new instance of psParentRunningProcessProbe
// TODO: add configurable options opts...
func newpsParentRunningProcessProbe(pid int) (*psParentRunningProcessProbe, error) {
	return &psParentRunningProcessProbe{
		pid: pid,
	}, nil
}

func getPsAxCommand() ([]string, error) {
	return []string{"ps", "-ax", "-o", "pid=,ppid=,command="}, nil
}

// getCommand implements cmdProbe.getCommand
func (p *psAllRunningProcessesProbe) getCommand() ([]string, error) {
	return getPsAxCommand()
}

// getCommand implements cmdProbe.getCommand
func (p *psSingleRunningProcessProbe) getCommand() ([]string, error) {
	return getPsAxCommand()
}

// getCommand implements cmdProbe.getCommand
func (p *psParentRunningProcessProbe) getCommand() ([]string, error) {
	return getPsAxCommand()
}

// parsePSCommandLine parses a line from PS
func parsePSCommandLine(line string) (command, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return command{}, unexpectedLineErr
	}
	lineArr := strings.Fields(line)
	if len(lineArr) < 3 {
		return command{}, unexpectedLineErr
	}
	pid, err := strconv.Atoi(lineArr[0])
	if err != nil {
		return command{}, err
	}
	ppid, err := strconv.Atoi(lineArr[1])
	if err != nil {
		return command{}, err
	}
	return command{
		pid:     pid,
		ppid:    ppid,
		command: lineArr[2:],
	}, nil
}

// parseCommandOuput implements cmdProbe.parseCommandOuput
func (p *psAllRunningProcessesProbe) parseCommandOuput(out []byte) (*psAllRunningProcessesProbe, error) {
	var commands []command

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		command, err := parsePSCommandLine(line)
		if err != nil {
			if errors.Is(err, unexpectedLineErr) {
				log.Warn().Msgf("Error parsing the line: %s", err)
				continue
			} else {
				return nil, err
			}
		}
		commands = append(commands, command)
	}
	return &psAllRunningProcessesProbe{
		commands: commands,
	}, nil
}

// parseCommandOuput implements cmdProbe.parseCommandOuput
func (p *psSingleRunningProcessProbe) parseCommandOuput(out []byte) (*psSingleRunningProcessProbe, error) {
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		command, err := parsePSCommandLine(line)
		if err != nil {
			if errors.Is(err, unexpectedLineErr) {
				log.Warn().Msgf("Error parsing the line: %s", err)
				continue
			} else {
				return nil, err
			}
		}
		if command.pid == p.pid {
			p.command = command
			return p, nil
		}
	}
	return nil, commandNotFoundErr
}

// parseCommandOuput implements cmdProbe.parseCommandOuput
func (p *psParentRunningProcessProbe) parseCommandOuput(out []byte) (*psParentRunningProcessProbe, error) {
	lines := strings.Split(string(out), "\n")
	ppid := -1
	// iterate to find matching command
	for _, line := range lines {
		command, err := parsePSCommandLine(line)
		if err != nil {
			if errors.Is(err, unexpectedLineErr) {
				log.Warn().Msgf("Error parsing the line: %s", err)
				continue
			} else {
				return nil, err
			}
		}
		if command.pid == p.pid {
			ppid = command.ppid
			break
		}
	}
	if ppid < 0 {
		return nil, commandNotFoundErr
	}
	// iterate to find matching parent command
	for _, line := range lines {
		command, err := parsePSCommandLine(line)
		if err != nil {
			if errors.Is(err, unexpectedLineErr) {
				log.Warn().Msgf("Error parsing the line: %s", err)
				continue
			} else {
				return nil, err
			}
		}
		if command.pid == ppid {
			p.command = command
			return p, nil
		}
	}
	return nil, commandNotFoundErr
}
