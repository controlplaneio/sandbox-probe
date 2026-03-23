package tasks

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

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
