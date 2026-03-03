package tasks

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

const (
	LANDLOCK_MAX_DEPTH = 16
)

func ProbeForLandlock() (bool, error) {
	self, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("failed to get self executable location: %w", err)
	}

	cmd := exec.Command(self, "probe")
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}

	var pd ProbeSubCmdData
	err = json.Unmarshal(out, &pd)
	if err != nil {
		return false, err
	}

	if pd.LockdownDepth > LANDLOCK_MAX_DEPTH {
		return false, fmt.Errorf("lockdown depth isn't expected to ever go beyond %d", LANDLOCK_MAX_DEPTH)
	}

	if pd.LockdownDepth == LANDLOCK_MAX_DEPTH {
		return false, nil
	}

	return true, nil
}
