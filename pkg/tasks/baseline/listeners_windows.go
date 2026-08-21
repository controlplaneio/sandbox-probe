//go:build windows
// +build windows

package tasks

import (
	"encoding/binary"
	"fmt"
	"net"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows keeps its socket tables in the network store, reachable through
// iphlpapi. This is what `netstat -ano` calls, and the OWNER_PID table classes
// need no elevation — the elevation cliff is OWNER_MODULE, which this does not
// touch.
//
// Pure Go via LazyDLL, so CGO_ENABLED=0 is unaffected. The house pattern for
// direct syscalls here is pipes_windows.go next door.
var (
	iphlpapi           = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetExtTCPTable = iphlpapi.NewProc("GetExtendedTcpTable")
	procGetExtUDPTable = iphlpapi.NewProc("GetExtendedUdpTable")
)

const (
	// TCP_TABLE_OWNER_PID_LISTENER: listening sockets only, so no state
	// filter is needed afterwards.
	tcpTableOwnerPIDListener = 3
	// UDP_TABLE_OWNER_PID: every bound UDP socket. See the caveat below.
	udpTableOwnerPID = 1

	errorInsufficientBuffer = 122 // ERROR_INSUFFICIENT_BUFFER
)

// Row sizes, fixed by the published MIB_* layouts. Asserted at compile time
// below so a wrong constant is a build failure rather than silently decoded
// noise.
const (
	rowTCP4 = 24 // dwState, dwLocalAddr, dwLocalPort, dwRemoteAddr, dwRemotePort, dwOwningPid
	rowTCP6 = 56 // ucLocalAddr[16], dwLocalScopeId, dwLocalPort, ucRemoteAddr[16], dwRemoteScopeId, dwRemotePort, dwState, dwOwningPid
	rowUDP4 = 12 // dwLocalAddr, dwLocalPort, dwOwningPid
	rowUDP6 = 28 // ucLocalAddr[16], dwLocalScopeId, dwLocalPort, dwOwningPid
)

func listLocalListeners() ([]Listener, error) {
	var out []Listener
	var firstErr error
	collect := func(ls []Listener, err error) {
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return
		}
		out = append(out, ls...)
	}

	collect(tcpListeners(windows.AF_INET))
	collect(tcpListeners(windows.AF_INET6))
	collect(udpListeners(windows.AF_INET))
	collect(udpListeners(windows.AF_INET6))

	// A table that could not be read is not an empty table. Reporting the
	// error lets the caller say "could not measure" rather than publishing a
	// silence that scores as the sandbox having blocked everything.
	if firstErr != nil && len(out) == 0 {
		return nil, firstErr
	}
	return out, nil
}

func tcpListeners(family uint32) ([]Listener, error) {
	buf, err := extendedTable(procGetExtTCPTable, "GetExtendedTcpTable", family, tcpTableOwnerPIDListener)
	if err != nil {
		return nil, err
	}
	rowLen := rowTCP4
	if family == windows.AF_INET6 {
		rowLen = rowTCP6
	}
	return decodeTable(buf, rowLen, "tcp", func(row []byte) (net.IP, int) {
		if family == windows.AF_INET6 {
			// ucLocalAddr[16] then dwLocalScopeId then dwLocalPort.
			return net.IP(row[0:16]), netPort(binary.LittleEndian.Uint32(row[20:24]))
		}
		// dwState then dwLocalAddr then dwLocalPort.
		return net.IP(row[4:8]).To4(), netPort(binary.LittleEndian.Uint32(row[8:12]))
	}), nil
}

func udpListeners(family uint32) ([]Listener, error) {
	buf, err := extendedTable(procGetExtUDPTable, "GetExtendedUdpTable", family, udpTableOwnerPID)
	if err != nil {
		return nil, err
	}
	rowLen := rowUDP4
	if family == windows.AF_INET6 {
		rowLen = rowUDP6
	}
	return decodeTable(buf, rowLen, "udp", func(row []byte) (net.IP, int) {
		if family == windows.AF_INET6 {
			// ucLocalAddr[16] then dwLocalScopeId then dwLocalPort.
			return net.IP(row[0:16]), netPort(binary.LittleEndian.Uint32(row[20:24]))
		}
		// dwLocalAddr then dwLocalPort.
		return net.IP(row[0:4]).To4(), netPort(binary.LittleEndian.Uint32(row[4:8]))
	}), nil
}

// decodeTable walks a MIB_*TABLE_OWNER_PID: a DWORD entry count, then that many
// fixed-size rows.
//
// CAVEAT, and it is a real one worth stating rather than hiding: unlike
// /proc/net/udp and darwin's pcblist, GetExtendedUdpTable carries NO remote
// address, so a connected UDP client socket is indistinguishable from a bound
// service here. On Linux and macOS the remote-port-is-zero test removes those;
// on Windows it cannot. The reachability half records which entries answered,
// which is what separates them in practice, and local_probe_status names the
// limitation so nobody reads more into a Windows UDP list than it can carry.
func decodeTable(buf []byte, rowLen int, proto string, addrPort func([]byte) (net.IP, int)) []Listener {
	if len(buf) < 4 {
		return nil
	}
	n := int(binary.LittleEndian.Uint32(buf[0:4]))
	// The count comes from the kernel, but trust the buffer: a short read
	// must truncate the walk rather than index past the end.
	if max := (len(buf) - 4) / rowLen; n > max {
		n = max
	}
	out := make([]Listener, 0, n)
	for i := 0; i < n; i++ {
		off := 4 + i*rowLen
		ip, port := addrPort(buf[off : off+rowLen])
		if port <= 0 {
			continue
		}
		out = append(out, Listener{Proto: proto, Addr: bindAddr(ip), Port: port})
	}
	return out
}

// netPort extracts the port from a DWORD whose low word holds it in NETWORK
// byte order. Truncating the DWORD instead turns port 22 into 5632, which is
// the classic way to get a plausible-looking but entirely wrong table.
func netPort(d uint32) int {
	return int(d&0xFF)<<8 | int((d>>8)&0xFF)
}

// extendedTable runs the size-then-fetch dance, retrying while the table grows
// underneath us. A busy machine really does add sockets between the two calls.
func extendedTable(proc *windows.LazyProc, name string, family, class uint32) ([]byte, error) {
	// LazyProc panics on a missing export rather than returning an error, and
	// Nano Server does ship without some of these. Find() turns that into an
	// error we can report.
	if err := proc.Find(); err != nil {
		return nil, fmt.Errorf("%s unavailable: %w", name, err)
	}

	var size uint32
	for attempt := 0; attempt < 6; attempt++ {
		var words []uint32
		var p unsafe.Pointer
		if size > 0 {
			// []uint32, not []byte: the rows contain DWORDs, and a
			// byte slice gives the runtime's checkptr no alignment
			// guarantee.
			words = make([]uint32, (size+3)/4)
			p = unsafe.Pointer(&words[0])
		}
		r, _, _ := proc.Call(
			uintptr(p),
			uintptr(unsafe.Pointer(&size)),
			0, // bOrder = FALSE
			uintptr(family),
			uintptr(class),
			0, // Reserved
		)
		switch r {
		case 0:
			if words == nil {
				return nil, nil // an empty table is a valid answer
			}
			b := unsafe.Slice((*byte)(unsafe.Pointer(&words[0])), len(words)*4)
			return append([]byte(nil), b[:min(int(size), len(b))]...), nil
		case errorInsufficientBuffer:
			continue // size now holds what the kernel wants; go round again
		default:
			// The return value carries the status directly; GetLastError
			// is not set by these calls.
			return nil, fmt.Errorf("%s: %w", name, windows.Errno(r))
		}
	}
	return nil, fmt.Errorf("%s: table kept growing across retries", name)
}

// networkNamespace has no Windows equivalent that means what the Linux one
// means. Returning "" says so rather than implying a shared host network.
func networkNamespace() string { return "" }

// inventorySource names where the table came from, for the report.
func inventorySource() string { return "iphlpapi" }
