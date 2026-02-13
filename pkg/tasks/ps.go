package tasks

import (
	"context"
	"os"

	reportv1 "github.com/controlplaneio/sandbox-probe/api/gen/proto/report/v1"
	cmdBasedTasks "github.com/controlplaneio/sandbox-probe/pkg/tasks/cmd-based"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/structpb"
)

func cmdToFinding(cmd cmdBasedTasks.Command, Type, desc, task string) (*reportv1.Finding, error) {
	cmdMap := map[string]interface{}{
		"pid":     cmd.Pid,
		"ppid":    cmd.Ppid,
		"command": stringSliceToInterface(cmd.Command),
	}
	cmdValue, err := structpb.NewValue(cmdMap)
	if err != nil {
		log.Error().Err(err).Msg("Failed to convert cmd to protobuf value")
		return nil, err
	}
	return &reportv1.Finding{
		FindingType: Type,
		Description: desc,
		Task:        task,
		Value:       cmdValue,
	}, nil
}

type PSAllTask struct {
	baseTask
}

func NewPSAllTask() *PSAllTask {
	return &PSAllTask{
		baseTask: baseTask{
			name:        "ps_all",
			description: "Lists all running processes using ps command",
		},
	}
}

func (t *PSAllTask) Run(ctx context.Context) ([]*reportv1.Finding, error) {
	var findings []*reportv1.Finding
	result, err := cmdBasedTasks.RunPSAllRunningProcessesCmd()
	if err != nil {
		return []*reportv1.Finding{}, err
	}
	for _, cmd := range result.Commands {
		finding, err := cmdToFinding(cmd, PROCESSDETECTION, "process found", t.GetName())
		if err != nil {
			log.Warn().Msgf("Unexpected error parsing command: %s", err)
			continue
		}
		findings = append(findings, finding)
	}
	return findings, nil
}

type PSSingleTask struct {
	baseTask
	pid int
}

func NewPSSingleTask() *PSSingleTask {
	return &PSSingleTask{
		baseTask: baseTask{
			name:        "ps_single",
			description: "Gets information about the running process using ps command",
		},
		pid: os.Getpid(),
	}
}

func (t *PSSingleTask) Run(ctx context.Context) ([]*reportv1.Finding, error) {
	result, err := cmdBasedTasks.RunPSSingleRunningProcessCmd(t.pid)
	if err != nil {
		return []*reportv1.Finding{}, err
	}
	finding, err := cmdToFinding(result.Command, PROCESSDETECTION, "single process found", t.GetName())
	if err != nil {
		log.Warn().Msgf("Unexpected error parsing command: %s", err)
		return []*reportv1.Finding{}, err
	}
	return []*reportv1.Finding{finding}, nil
}

type PSParentTask struct {
	baseTask
	pid int
}

func NewPSParentTask() *PSParentTask {
	return &PSParentTask{
		baseTask: baseTask{
			name:        "ps_parent",
			description: "Gets parent process information using ps command",
		},
		pid: os.Getpid(),
	}
}

func (t *PSParentTask) Run(ctx context.Context) ([]*reportv1.Finding, error) {
	result, err := cmdBasedTasks.RunPSParentRunningProcessCmd(t.pid)
	if err != nil {
		return []*reportv1.Finding{}, err
	}
	finding, err := cmdToFinding(result.Command, PROCESSDETECTION, "parent process found", t.GetName())
	if err != nil {
		log.Warn().Msgf("Unexpected error parsing command: %s", err)
		return []*reportv1.Finding{}, err
	}
	return []*reportv1.Finding{finding}, nil
}

func GetPSTasks() []Task {
	return []Task{
		NewPSAllTask(),
		NewPSParentTask(),
		NewPSSingleTask(),
	}
}
