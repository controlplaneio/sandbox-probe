package probes

import (
	"context"
	"errors"

	reportv1 "github.com/controlplaneio/sandbox-probe/api/gen/proto/report/v1"
	"github.com/controlplaneio/sandbox-probe/pkg/tasks"
	"github.com/rs/zerolog/log"
)

type ProbeType int

var noTasksErr = errors.New("no tasks in probe")

type Probe struct {
	Name     string
	Tasks    []tasks.Task
	Findings []*reportv1.Finding
	// fast mode
	Fast bool
}

func (p *Probe) Run() error {
	if len(p.Tasks) < 1 {
		return noTasksErr
	}

	for idx, task := range p.Tasks {
		log.Info().Msgf("Running task (%d/%d) %s", idx, len(p.Tasks), task.GetName())
		findings, err := task.Run(context.TODO(), tasks.Inputs{
			Fast: p.Fast,
		})
		if err != nil {
			log.Warn().Msgf("Error running task '%s': %s", task.GetName(), err)
		} else {
			p.Findings = append(p.Findings, findings...)
		}
	}

	return nil
}

func NewProbe(opts ...NewProbeOpt) (*Probe, error) {
	probe := &Probe{}

	for _, opt := range opts {
		if err := opt(probe); err != nil {
			return nil, err
		}
	}

	return probe, nil
}
