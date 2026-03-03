package tasks

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func isProcSelfSetNoNewPrivs() bool {
	r1, _, errno := unix.Syscall6(
		unix.SYS_PRCTL,
		uintptr(unix.PR_GET_NO_NEW_PRIVS),
		0, 0, 0, 0, 0,
	)

	if errno != 0 {
		panic(errno)
	}

	if r1 == 1 {
		fmt.Println("NoNewPrivs is enabled")
	} else {
		fmt.Println("NoNewPrivs is disabled")
	}
	return false
}
