package tasks

/*
#include <unistd.h>

// A non-variadic wrapper for the private Apple API
int check_my_sandbox(pid_t pid) {
    // We call the real function with NULL for operation and 0 for type
    // This is the standard way to check general process confinement
    extern int sandbox_check(pid_t, const char *, int, ...);
    return sandbox_check(pid, NULL, 0);
}
*/
import "C"

import (
	"fmt"
	"os"
)

func isSeatbelt() bool {
	// Call our C wrapper
	ret := C.check_my_sandbox(C.pid_t(os.Getpid()))

	if ret == 1 {
		fmt.Println("Result: [SANDBOXED] - Seatbelt is active.")
		return true
	} else {
		fmt.Println("Result: [FREE] - No Seatbelt detected.")
	}
	return false
}
