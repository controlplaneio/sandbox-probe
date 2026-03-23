package tasks

import (
	"context"
	"testing"

	cmdBasedTasks "github.com/controlplaneio/sandbox-probe/pkg/tasks/cmd-based"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCmdToFinding(t *testing.T) {
	tests := []struct {
		name        string
		cmd         cmdBasedTasks.Command
		findingType string
		desc        string
		task        string
		wantErr     bool
	}{
		{
			name: "valid command conversion",
			cmd: cmdBasedTasks.Command{
				Pid:     1234,
				Ppid:    1,
				Command: []string{"/bin/bash", "-c", "echo test"},
			},
			findingType: PROCESSDETECTION,
			desc:        "process found",
			task:        "ps_all",
			wantErr:     false,
		},
		{
			name: "command with single argument",
			cmd: cmdBasedTasks.Command{
				Pid:     5678,
				Ppid:    1234,
				Command: []string{"/usr/bin/python3"},
			},
			findingType: PARENTPROCESSDETECTION,
			desc:        "parent process found",
			task:        "ps_parent",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			finding, err := cmdToFinding(tt.cmd, tt.findingType, tt.desc, tt.task)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify finding structure
			assert.Equal(t, tt.findingType, finding.FindingType)
			assert.Equal(t, tt.desc, finding.Description)
			assert.Equal(t, tt.task, finding.Task)

			// Verify the value can be converted to a map
			require.NotNil(t, finding.Value)

			valueMap := finding.Value.AsInterface().(map[string]interface{})
			assert.Equal(t, float64(tt.cmd.Pid), valueMap["pid"])
			assert.Equal(t, float64(tt.cmd.Ppid), valueMap["ppid"])
		})
	}
}

func TestNewPSAllTask(t *testing.T) {
	task := NewPSAllTask()
	require.NotNil(t, task)
	assert.Equal(t, "ps_all", task.name)
}

func TestPSAllTask_GetName(t *testing.T) {
	task := NewPSAllTask()
	assert.Equal(t, "ps_all", task.GetName())
}

func TestPSAllTask_Run(t *testing.T) {
	task := NewPSAllTask()
	ctx := context.Background()

	findings, err := task.Run(ctx, Inputs{})
	require.NoError(t, err)

	// Verify findings structure (should have at least one process)
	assert.NotEmpty(t, findings, "Run() returned no findings, expected at least one process")

	// Verify first finding has correct structure
	if len(findings) > 0 {
		finding := findings[0]
		assert.Equal(t, PROCESSDETECTION, finding.FindingType)
		assert.Equal(t, "process found", finding.Description)
		assert.Equal(t, "ps_all", finding.Task)
		assert.NotNil(t, finding.Value)
	}
}

func TestNewPSSingleTask(t *testing.T) {
	task := NewPSSingleTask()
	if task == nil {
		t.Fatal("NewPSSingleTask() returned nil")
	}
	if task.name != "ps_single" {
		t.Errorf("task.name = %v, want %v", task.name, "ps_single")
	}
}

func TestPSSingleTask_GetName(t *testing.T) {
	task := NewPSSingleTask()
	if got := task.GetName(); got != "ps_single" {
		t.Errorf("GetName() = %v, want %v", got, "ps_single")
	}
}

func TestPSSingleTask_Run(t *testing.T) {
	// Use PID 1 which should always exist (init/systemd)
	task := NewPSSingleTask()
	ctx := context.Background()

	findings, err := task.Run(ctx, Inputs{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Should return exactly one finding
	assert.Equal(t, len(findings), 1)
}

func TestNewPSParentTask(t *testing.T) {
	task := NewPSParentTask()
	if task == nil {
		t.Fatal("NewPSParentTask() returned nil")
	}
	if task.name != "ps_parent" {
		t.Errorf("task.name = %v, want %v", task.name, "ps_parent")
	}
}

func TestPSParentTask_GetName(t *testing.T) {
	task := NewPSParentTask()
	if got := task.GetName(); got != "ps_parent" {
		t.Errorf("GetName() = %v, want %v", got, "ps_parent")
	}
}

func TestCmdToFinding_InvalidCommand(t *testing.T) {
	// Test with a command that has an invalid structure that might cause issues
	// when converting to structpb.Value
	cmd := cmdBasedTasks.Command{
		Pid:     1,
		Ppid:    0,
		Command: []string{},
	}

	finding, err := cmdToFinding(cmd, PROCESSDETECTION, "test", "test_task")
	if err != nil {
		t.Errorf("cmdToFinding() unexpected error = %v", err)
		return
	}

	// Should still create a valid finding
	if finding == nil {
		t.Error("cmdToFinding() returned nil finding")
		return
	}

	// Verify the value is properly structured
	if finding.Value == nil {
		t.Error("finding.Value is nil")
		return
	}

	valueMap := finding.Value.AsInterface().(map[string]interface{})
	if _, ok := valueMap["command"]; !ok {
		t.Error("finding.Value missing 'command' field")
	}
}

func TestCmdToFinding_StructpbConversion(t *testing.T) {
	// Test that the conversion to structpb works correctly with various command structures
	testCases := []struct {
		name    string
		command []string
	}{
		{"empty command", []string{}},
		{"single arg", []string{"/bin/sh"}},
		{"multiple args", []string{"/bin/sh", "-c", "echo 'test'"}},
		{"long command", []string{"/usr/bin/python3", "-m", "http.server", "8000", "--bind", "0.0.0.0"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := cmdBasedTasks.Command{
				Pid:     100,
				Ppid:    1,
				Command: tc.command,
			}

			finding, err := cmdToFinding(cmd, PROCESSDETECTION, "test", "test")
			if err != nil {
				t.Errorf("cmdToFinding() error = %v", err)
				return
			}

			// Verify we can convert back from structpb
			valueMap := finding.Value.AsInterface().(map[string]interface{})

			// Verify command is present and is a slice
			cmdInterface, ok := valueMap["command"]
			if !ok {
				t.Error("command field missing from value")
				return
			}

			// The command should be convertible to a slice
			if cmdSlice, ok := cmdInterface.([]interface{}); ok {
				if len(cmdSlice) != len(tc.command) {
					t.Errorf("command length = %d, want %d", len(cmdSlice), len(tc.command))
				}
			} else {
				t.Errorf("command is not a slice, got type %T", cmdInterface)
			}
		})
	}
}

// Test that findings can be properly validated
func TestFindingValidation(t *testing.T) {
	cmd := cmdBasedTasks.Command{
		Pid:     1234,
		Ppid:    1,
		Command: []string{"/bin/test"},
	}

	finding, err := cmdToFinding(cmd, PROCESSDETECTION, "test", "test_task")
	require.NoError(t, err)

	// The finding should have all required fields
	if finding.FindingType == "" {
		t.Error("finding.FindingType is empty")
	}
	if finding.Description == "" {
		t.Error("finding.Description is empty")
	}
	if finding.Task == "" {
		t.Error("finding.Task is empty")
	}
	if finding.Value == nil {
		t.Error("finding.Value is nil")
	}

	// Verify Value is a valid structpb.Value
	if _, ok := finding.Value.Kind.(*structpb.Value_StructValue); !ok {
		t.Errorf("finding.Value.Kind is not *structpb.Value_StructValue, got %T", finding.Value.Kind)
	}
}
