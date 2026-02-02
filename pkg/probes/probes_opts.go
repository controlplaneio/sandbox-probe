package probes

import "github.com/controlplaneio/sandbox-probe/pkg/tasks"

type NewProbeOpt func(*Probe) error

func WithName(name string) NewProbeOpt {
	return func(p *Probe) error {
		p.Name = name
		return nil
	}
}

func WithTasks(tasks []tasks.Task) NewProbeOpt {
	return func(p *Probe) error {
		p.Tasks = append(p.Tasks, tasks...)
		return nil
	}
}
