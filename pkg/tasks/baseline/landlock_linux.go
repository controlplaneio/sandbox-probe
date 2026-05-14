//go:build linux
// +build linux

package tasks

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/unix"
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

func ProbeLandlockSelfDepth() (int, error) {
	// Required before landlock_restrict_self
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return 0, fmt.Errorf("prctl failed: %w", err)
	}

	// Define which access rights we want to handle (deny by default)
	handledAccess := uint64(unix.LANDLOCK_ACCESS_FS_EXECUTE |
		unix.LANDLOCK_ACCESS_FS_WRITE_FILE |
		unix.LANDLOCK_ACCESS_FS_READ_FILE |
		unix.LANDLOCK_ACCESS_FS_READ_DIR)

	attr := unix.LandlockRulesetAttr{
		Access_fs: handledAccess,
	}

	rulesetFD, _, errno := unix.Syscall(
		unix.SYS_LANDLOCK_CREATE_RULESET,
		uintptr(unsafe.Pointer(&attr)),
		uintptr(unsafe.Sizeof(attr)),
		0,
	)
	if errno != 0 {
		return 0, fmt.Errorf("create_ruleset failed: %v", errno)
	}
	defer unix.Close(int(rulesetFD))

	_, _, errno = unix.Syscall(
		unix.SYS_LANDLOCK_RESTRICT_SELF,
		rulesetFD,
		0,
		0,
	)
	if errno != 0 {
		return 0, fmt.Errorf("restrict_self failed: %v", errno)
	}

	for i := 1; i < LANDLOCK_MAX_DEPTH*3; i++ {
		err := landlock()
		if err != nil {
			return i, nil
		}
	}

	// should be unreachable
	return 0, errors.New("didn't reach an error with lockdown depth")
}

func landlock() error {
	handledAccess := uint64(unix.LANDLOCK_ACCESS_FS_EXECUTE |
		unix.LANDLOCK_ACCESS_FS_WRITE_FILE |
		unix.LANDLOCK_ACCESS_FS_READ_FILE |
		unix.LANDLOCK_ACCESS_FS_READ_DIR)

	attr := unix.LandlockRulesetAttr{
		Access_fs: handledAccess,
	}

	rulesetFD, _, errno := unix.Syscall(
		unix.SYS_LANDLOCK_CREATE_RULESET,
		uintptr(unsafe.Pointer(&attr)),
		uintptr(unsafe.Sizeof(attr)),
		0,
	)
	if errno != 0 {
		return fmt.Errorf("create_ruleset failed: %v", errno)
	}
	defer unix.Close(int(rulesetFD))

	_, _, errno = unix.Syscall(
		unix.SYS_LANDLOCK_RESTRICT_SELF,
		rulesetFD,
		0,
		0,
	)
	if errno != 0 {
		return fmt.Errorf("restrict_self failed: %v", errno)
	}

	return nil
}
