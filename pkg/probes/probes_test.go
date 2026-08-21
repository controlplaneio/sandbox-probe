package probes

import (
	"context"
	"errors"
	"testing"

	reportv1 "github.com/controlplaneio/sandbox-probe/api/gen/proto/report/v1"
	"github.com/controlplaneio/sandbox-probe/pkg/tasks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingTask struct{}

func (failingTask) GetName() string        { return "failing" }
func (failingTask) GetDescription() string { return "returns an error" }
func (failingTask) Run(context.Context, tasks.Inputs) ([]*reportv1.Finding, error) {
	return []*reportv1.Finding{{FindingType: "test"}}, tasks.NewScanFailure(errors.New("expected failure"))
}

func TestProbeRunPropagatesTaskErrors(t *testing.T) {
	probe, err := NewProbe(WithTasks([]tasks.Task{failingTask{}}))
	require.NoError(t, err)

	assert.Error(t, probe.Run())
	assert.Len(t, probe.Findings, 1)
}

type optionalFailingTask struct{}

func (optionalFailingTask) GetName() string        { return "optional-failing" }
func (optionalFailingTask) GetDescription() string { return "returns a non-fatal error" }
func (optionalFailingTask) Run(context.Context, tasks.Inputs) ([]*reportv1.Finding, error) {
	return nil, errors.New("expected optional failure")
}

func TestProbeRunDoesNotPropagateNonFatalTaskErrors(t *testing.T) {
	probe, err := NewProbe(WithTasks([]tasks.Task{optionalFailingTask{}}))
	require.NoError(t, err)

	assert.NoError(t, probe.Run())
}
