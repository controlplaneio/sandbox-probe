//go:build windows
// +build windows

package tasks

import (
	"net"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Winsock error numbers, not the syscall.E* constants.
//
// This file exists because of a trap that compiles cleanly and is always
// wrong: on Windows, syscall.ECONNREFUSED is defined as APPLICATION_ERROR+n, a
// Go-synthesised value the socket layer never returns. Winsock returns
// WSAECONNREFUSED (10061). So errors.Is(err, syscall.ECONNREFUSED) builds
// happily on Windows and silently never matches, which would classify every
// refusal as an unknown error.

func isRefused(e syscall.Errno) bool {
	// WSAECONNRESET is how a connected UDP socket surfaces an ICMP
	// port-unreachable, so it belongs here alongside the TCP refusal.
	return e == syscall.Errno(windows.WSAECONNREFUSED) ||
		e == syscall.Errno(windows.WSAECONNRESET)
}

func isPermission(e syscall.Errno) bool {
	return e == syscall.Errno(windows.WSAEACCES)
}

func isUnreachable(e syscall.Errno) bool {
	return e == syscall.Errno(windows.WSAENETUNREACH) ||
		e == syscall.Errno(windows.WSAEHOSTUNREACH) ||
		e == syscall.Errno(windows.WSAENETDOWN)
}

// isResourceExhaustion is the class the old scanner silently turned into a
// security finding. Running out of sockets is a fact about the probe, never
// about the sandbox.
func isResourceExhaustion(e syscall.Errno) bool {
	return e == syscall.Errno(windows.WSAEMFILE) ||
		e == syscall.Errno(windows.WSAENOBUFS) ||
		e == syscall.Errno(windows.WSAEADDRNOTAVAIL)
}

func isTimeout(e syscall.Errno) bool {
	return e == syscall.Errno(windows.WSAETIMEDOUT)
}

func errnoName(e syscall.Errno) string {
	switch e {
	case syscall.Errno(windows.WSAECONNREFUSED):
		return "WSAECONNREFUSED"
	case syscall.Errno(windows.WSAECONNRESET):
		return "WSAECONNRESET"
	case syscall.Errno(windows.WSAEACCES):
		return "WSAEACCES"
	case syscall.Errno(windows.WSAENETUNREACH):
		return "WSAENETUNREACH"
	case syscall.Errno(windows.WSAEHOSTUNREACH):
		return "WSAEHOSTUNREACH"
	case syscall.Errno(windows.WSAENETDOWN):
		return "WSAENETDOWN"
	case syscall.Errno(windows.WSAEMFILE):
		return "WSAEMFILE"
	case syscall.Errno(windows.WSAENOBUFS):
		return "WSAENOBUFS"
	case syscall.Errno(windows.WSAEADDRNOTAVAIL):
		return "WSAEADDRNOTAVAIL"
	case syscall.Errno(windows.WSAETIMEDOUT):
		return "WSAETIMEDOUT"
	}
	return e.Error()
}

// enableUDPErrorReporting re-enables ICMP error delivery on a connected UDP
// socket.
//
// Go's own net package disables it on every UDP socket it creates
// (net/fd_windows.go calls WSAIoctl with SIO_UDP_CONNRESET set to false),
// because a stray ICMP port-unreachable otherwise fails an unrelated recv on a
// shared socket. That is sensible for a general-purpose library and fatal for
// a probe: it suppresses the single signal that distinguishes "nothing is
// listening there" from "no answer came back", which is the whole basis of UDP
// reachability.
//
// Turning it back on is therefore deliberate and scoped to sockets this probe
// creates for one connected conversation each.
func enableUDPErrorReporting(c any) error {
	conn, ok := c.(*net.UDPConn)
	if !ok {
		return nil
	}
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var ioctlErr error
	if err := raw.Control(func(fd uintptr) {
		flag := uint32(1) // TRUE: report ICMP errors on this socket
		var ret uint32
		ioctlErr = windows.WSAIoctl(
			windows.Handle(fd),
			windows.SIO_UDP_CONNRESET,
			(*byte)(unsafe.Pointer(&flag)), uint32(unsafe.Sizeof(flag)),
			nil, 0,
			&ret, nil, 0,
		)
	}); err != nil {
		return err
	}
	return ioctlErr
}
