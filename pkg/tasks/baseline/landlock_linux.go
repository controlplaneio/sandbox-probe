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

	// fmt.Println("Child: creating ruleset...")

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

	// fmt.Println("Child: restricting self...")

	_, _, errno = unix.Syscall(
		unix.SYS_LANDLOCK_RESTRICT_SELF,
		rulesetFD,
		0,
		0,
	)
	if errno != 0 {
		return 0, fmt.Errorf("restrict_self failed: %v", errno)
	}

	// if !isProcSelfSetNoNewPrivs() {
	// 	fmt.Println("Child: sandbox not active.")
	// 	os.Exit(1)
	// }
	// fmt.Println("Child: sandbox active.")

	// Test access
	// fmt.Println("Child: attempting to read /proc/self/attr/current...")
	// _, err := os.ReadFile("/proc/self/attr/current")
	// if err != nil {
	// 	fmt.Println("Access denied as expected:", err)
	// } else {
	// 	fmt.Println("Unexpectedly succeeded.")
	// }

	for i := 1; i < LANDLOCK_MAX_DEPTH*3; i++ {
		err := lockdown(i)
		if err != nil {
			return i, nil
		}
	}

	// should be unreachable
	return 0, errors.New("didn't reach an error with lockdown depth")
}

func lockdown(depth int) error {
	// fmt.Printf("Child: sandbox starting. %d\n", depth)
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
		return fmt.Errorf("create_ruleset failed: %v", errno)
	}
	defer unix.Close(int(rulesetFD))

	// r1, r2, errno := unix.Syscall(
	_, _, errno = unix.Syscall(
		unix.SYS_LANDLOCK_RESTRICT_SELF,
		rulesetFD,
		0,
		0,
	)
	if errno == 7 {
		// fmt.Printf("failed depth %v\n", errno)
	}
	if errno != 0 {
		// fmt.Printf("failed %v\n", errno)
		return fmt.Errorf("restrict_self failed: %v", errno)
	}

	// fmt.Printf("Child: sandbox active. %d, %x, %x, %x\n", depth, r1, r2, errno)
	return nil
}
