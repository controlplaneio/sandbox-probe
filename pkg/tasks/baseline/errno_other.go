//go:build !windows
// +build !windows

package tasks

import "syscall"

// POSIX errno predicates. These exist as a platform-split pair with
// errno_windows.go for one reason: Windows returns Winsock numbers, and the
// syscall.E* constants there are Go-synthesised values the socket layer never
// produces. Keeping the comparison behind these helpers is what stops shared
// code compiling cleanly and being wrong at runtime on one platform.

func isRefused(e syscall.Errno) bool { return e == syscall.ECONNREFUSED }

func isPermission(e syscall.Errno) bool {
	return e == syscall.EPERM || e == syscall.EACCES
}

func isUnreachable(e syscall.Errno) bool {
	return e == syscall.ENETUNREACH || e == syscall.EHOSTUNREACH ||
		e == syscall.ENETDOWN || e == syscall.EHOSTDOWN
}

// isResourceExhaustion is the class the old scanner silently turned into a
// security finding. Running out of file descriptors is a fact about the probe,
// never about the sandbox.
func isResourceExhaustion(e syscall.Errno) bool {
	return e == syscall.EMFILE || e == syscall.ENFILE ||
		e == syscall.ENOBUFS || e == syscall.ENOMEM ||
		e == syscall.EADDRNOTAVAIL
}

func isTimeout(e syscall.Errno) bool { return e == syscall.ETIMEDOUT }

func errnoName(e syscall.Errno) string {
	switch e {
	case syscall.ECONNREFUSED:
		return "ECONNREFUSED"
	case syscall.EPERM:
		return "EPERM"
	case syscall.EACCES:
		return "EACCES"
	case syscall.ENETUNREACH:
		return "ENETUNREACH"
	case syscall.EHOSTUNREACH:
		return "EHOSTUNREACH"
	case syscall.ENETDOWN:
		return "ENETDOWN"
	case syscall.EHOSTDOWN:
		return "EHOSTDOWN"
	case syscall.EMFILE:
		return "EMFILE"
	case syscall.ENFILE:
		return "ENFILE"
	case syscall.ENOBUFS:
		return "ENOBUFS"
	case syscall.ENOMEM:
		return "ENOMEM"
	case syscall.EADDRNOTAVAIL:
		return "EADDRNOTAVAIL"
	case syscall.ETIMEDOUT:
		return "ETIMEDOUT"
	case syscall.ECONNRESET:
		return "ECONNRESET"
	}
	return e.Error()
}

// enableUDPErrorReporting is a no-op off Windows. A connected UDP socket
// surfaces ICMP port-unreachable as ECONNREFUSED on the next operation without
// anything being asked of it.
func enableUDPErrorReporting(any) error { return nil }
