package tasks

import (
	"errors"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

var (
	errUnexpectedLine  = errors.New("unexpected format in command output line")
	errCommandNotFound = errors.New("command not found")
)

type Command struct {
	Pid     int
	Ppid    int
	Command []string
}

type PSAllRunningProcessesCmd struct {
	Commands []Command
}

// returns a new instance of PSAllRunningProcessesCmd
// TODO: add configurable options opts...
func newPsAllRunningProcessesCmd() (*PSAllRunningProcessesCmd, error) {
	return &PSAllRunningProcessesCmd{}, nil
}

type PSSingleRunningProcessCmd struct {
	Pid     int
	Command Command
}

// returns a new instance of PSSingleRunningProcessCmd
// TODO: add configurable options opts...
func newPsSingleRunningProcessCmd(pid int) (*PSSingleRunningProcessCmd, error) {
	return &PSSingleRunningProcessCmd{
		Pid: pid,
	}, nil
}

type PSParentRunningProcessCmd struct {
	Pid     int
	Command Command
}

// returns a new instance of PSParentRunningProcessCmd
// TODO: add configurable options opts...
func newPSParentRunningProcessCmd(pid int) (*PSParentRunningProcessCmd, error) {
	return &PSParentRunningProcessCmd{
		Pid: pid,
	}, nil
}

func getPsAxCommand() ([]string, error) {
	return []string{"ps", "-ax", "-o", "pid=,ppid=,command="}, nil
}

// getCommand implements CmdTask.getCommand
func (p *PSAllRunningProcessesCmd) getCommand() ([]string, error) {
	return getPsAxCommand()
}

// getCommand implements CmdTask.getCommand
func (p *PSSingleRunningProcessCmd) getCommand() ([]string, error) {
	return getPsAxCommand()
}

// getCommand implements CmdTask.getCommand
func (p *PSParentRunningProcessCmd) getCommand() ([]string, error) {
	return getPsAxCommand()
}

// parsePSCommandLine parses a line from PS
func parsePSCommandLine(line string) (Command, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Command{}, errUnexpectedLine
	}
	lineArr := strings.Fields(line)
	if len(lineArr) < 3 {
		return Command{}, errUnexpectedLine
	}
	pid, err := strconv.Atoi(lineArr[0])
	if err != nil {
		return Command{}, err
	}
	ppid, err := strconv.Atoi(lineArr[1])
	if err != nil {
		return Command{}, err
	}
	return Command{
		Pid:     pid,
		Ppid:    ppid,
		Command: lineArr[2:],
	}, nil
}

// parseCommandOuput implements CmdTask.parseCommandOuput
func (p *PSAllRunningProcessesCmd) parseCommandOuput(out []byte) (*PSAllRunningProcessesCmd, error) {
	var commands []Command

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		command, err := parsePSCommandLine(line)
		if err != nil {
			if errors.Is(err, errUnexpectedLine) {
				log.Warn().Msgf("Error parsing the line: %s", err)
				continue
			} else {
				return nil, err
			}
		}
		commands = append(commands, command)
	}
	return &PSAllRunningProcessesCmd{
		Commands: commands,
	}, nil
}

// parseCommandOuput implements CmdTask.parseCommandOuput
func (p *PSSingleRunningProcessCmd) parseCommandOuput(out []byte) (*PSSingleRunningProcessCmd, error) {
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		command, err := parsePSCommandLine(line)
		if err != nil {
			if errors.Is(err, errUnexpectedLine) {
				log.Warn().Msgf("Error parsing the line: %s", err)
				continue
			} else {
				return nil, err
			}
		}
		if command.Pid == p.Pid {
			p.Command = command
			return p, nil
		}
	}
	return nil, errCommandNotFound
}

// parseCommandOuput implements CmdTask.parseCommandOuput
func (p *PSParentRunningProcessCmd) parseCommandOuput(out []byte) (*PSParentRunningProcessCmd, error) {
	lines := strings.Split(string(out), "\n")
	ppid := -1
	// iterate to find matching command
	for _, line := range lines {
		command, err := parsePSCommandLine(line)
		if err != nil {
			if errors.Is(err, errUnexpectedLine) {
				log.Warn().Msgf("Error parsing the line: %s", err)
				continue
			} else {
				return nil, err
			}
		}
		if command.Pid == p.Pid {
			ppid = command.Ppid
			break
		}
	}
	if ppid < 0 {
		return nil, errCommandNotFound
	}
	// iterate to find matching parent command
	for _, line := range lines {
		command, err := parsePSCommandLine(line)
		if err != nil {
			if errors.Is(err, errUnexpectedLine) {
				log.Warn().Msgf("Error parsing the line: %s", err)
				continue
			} else {
				return nil, err
			}
		}
		if command.Pid == ppid {
			p.Command = command
			return p, nil
		}
	}
	return nil, errCommandNotFound
}
