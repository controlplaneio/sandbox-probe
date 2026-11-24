package tasks

import (
	"context"
	"testing"

	baselineTasks "github.com/controlplaneio/sandbox-probe/v6/pkg/tasks/baseline"
)

// TestContainerRuntimeNameOpenShell pins the report strings: a runtime without a case here is
// detected and then silently dropped from the finding.
func TestContainerRuntimeNameOpenShell(t *testing.T) {
	for rt, want := range map[baselineTasks.ContainerRuntime]string{
		baselineTasks.RuntimeOpenShell: "openshell",
		baselineTasks.RuntimeNemoClaw:  "nemoclaw",
	} {
		if got := containerRuntimeName(rt); got != want {
			t.Errorf("containerRuntimeName(%v) = %q, want %q", rt, got, want)
		}
	}
}

// TestSandboxTaskReportsOpenShell runs the real task with the supervisor's marker in the process
// environment and reads the sandbox_detection finding back, so the wiring from detector to
// report is exercised, not just the mapping. No root needed: the env check is the first to fire.
func TestSandboxTaskReportsOpenShell(t *testing.T) {
	t.Setenv("OPENSHELL_SANDBOX", "1")
	findings, err := NewSandboxTask().Run(context.Background(), Inputs{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []string
	for _, f := range findings {
		if f.GetFindingType() == SANDBOXDETECTION {
			got = append(got, f.GetValue().GetStringValue())
		}
	}
	if len(got) != 1 || got[0] != "openshell" {
		t.Fatalf("sandbox_detection findings = %q, want exactly [\"openshell\"]", got)
	}
}
