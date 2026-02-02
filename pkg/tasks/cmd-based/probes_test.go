package tasks

import (
	"reflect"
	"testing"
)

// Generic test helper function that maintains type safety for each probe type
func testProbe[T any](t *testing.T, name string, probe cmdProbe[T], mockExecCommand func(string, ...string) ([]byte, error), expected T) {
	t.Run(name, func(t *testing.T) {
		// Replace execCommand with mock
		originalExecCommand := execCommand
		execCommand = mockExecCommand
		defer func() { execCommand = originalExecCommand }()

		// Run the probe
		result, err := runCmdProbe(probe)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Compare result with expected
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("got %+v, want %+v", result, expected)
		}
	})
}

func TestProbes(t *testing.T) {
	psOutput := `
      1       0 /usr/lib/systemd/systemd --system --deserialize=123 splash
      2       0 [kthreadd]
      3       2 [pool_workqueue_release]
	`

	// Test psAllRunningProcessesProbe with proper type
	testProbe[*psAllRunningProcessesProbe](
		t,
		"psAllRunningProcessesProbe",
		&psAllRunningProcessesProbe{},
		func(_ string, _ ...string) ([]byte, error) {
			return []byte(psOutput), nil
		},
		&psAllRunningProcessesProbe{
			commands: []command{
				{
					pid:     1,
					ppid:    0,
					command: []string{"/usr/lib/systemd/systemd", "--system", "--deserialize=123", "splash"},
				},
				{
					pid:     2,
					ppid:    0,
					command: []string{"[kthreadd]"},
				},
				{
					pid:     3,
					ppid:    2,
					command: []string{"[pool_workqueue_release]"},
				},
			},
		},
	)

	// Test psSingleRunningProcessProbe with proper type
	testProbe[*psSingleRunningProcessProbe](
		t,
		"psSingleRunningProcessProbe",
		&psSingleRunningProcessProbe{pid: 1},
		func(_ string, _ ...string) ([]byte, error) {
			return []byte(psOutput), nil
		},
		&psSingleRunningProcessProbe{
			pid: 1,
			command: command{
				pid:     1,
				ppid:    0,
				command: []string{"/usr/lib/systemd/systemd", "--system", "--deserialize=123", "splash"},
			},
		},
	)

	// Test psParentRunningProcessProbe with proper type
	testProbe[*psParentRunningProcessProbe](
		t,
		"psParentRunningProcessProbe",
		&psParentRunningProcessProbe{pid: 3},
		func(_ string, _ ...string) ([]byte, error) {
			return []byte(psOutput), nil
		},
		&psParentRunningProcessProbe{
			pid: 3,
			command: command{
				pid:     2,
				ppid:    0,
				command: []string{"[kthreadd]"},
			},
		},
	)
}
