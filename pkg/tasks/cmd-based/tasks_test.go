package tasks

import (
	"os"
	"reflect"
	"testing"

	"github.com/rs/zerolog"
)

func TestMain(m *testing.M) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Exit(m.Run())
}

// Generic test helper function that maintains type safety for each probe type
func testProbe[T any](t *testing.T, name string, probe CmdTask[T], mockExecCommand func(string, ...string) ([]byte, error), expected T) {
	t.Run(name, func(t *testing.T) {
		// Replace execCommand with mock
		originalExecCommand := execCommand
		execCommand = mockExecCommand
		defer func() { execCommand = originalExecCommand }()

		// Run the probe
		result, err := runCmdTask(probe)
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
	psOutput := `      1       0 /usr/lib/systemd/systemd --system --deserialize=123 splash
      2       0 [kthreadd]
      3       2 [pool_workqueue_release]`

	// Test PSAllRunningProcessesCmd with proper type
	testProbe[*PSAllRunningProcessesCmd](
		t,
		"PSAllRunningProcessesCmd",
		&PSAllRunningProcessesCmd{},
		func(_ string, _ ...string) ([]byte, error) {
			return []byte(psOutput), nil
		},
		&PSAllRunningProcessesCmd{
			Commands: []Command{
				{
					Pid:     1,
					Ppid:    0,
					Command: []string{"/usr/lib/systemd/systemd", "--system", "--deserialize=123", "splash"},
				},
				{
					Pid:     2,
					Ppid:    0,
					Command: []string{"[kthreadd]"},
				},
				{
					Pid:     3,
					Ppid:    2,
					Command: []string{"[pool_workqueue_release]"},
				},
			},
		},
	)

	// Test PSSingleRunningProcessCmd with proper type
	testProbe[*PSSingleRunningProcessCmd](
		t,
		"PSSingleRunningProcessCmd",
		&PSSingleRunningProcessCmd{Pid: 1},
		func(_ string, _ ...string) ([]byte, error) {
			return []byte(psOutput), nil
		},
		&PSSingleRunningProcessCmd{
			Pid: 1,
			Command: Command{
				Pid:     1,
				Ppid:    0,
				Command: []string{"/usr/lib/systemd/systemd", "--system", "--deserialize=123", "splash"},
			},
		},
	)

	// Test PSParentRunningProcessCmd with proper type
	testProbe[*PSParentRunningProcessCmd](
		t,
		"PSParentRunningProcessCmd",
		&PSParentRunningProcessCmd{Pid: 3},
		func(_ string, _ ...string) ([]byte, error) {
			return []byte(psOutput), nil
		},
		&PSParentRunningProcessCmd{
			Pid: 3,
			Command: Command{
				Pid:     2,
				Ppid:    0,
				Command: []string{"[kthreadd]"},
			},
		},
	)
}
