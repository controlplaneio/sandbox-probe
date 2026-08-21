//go:build darwin
// +build darwin

package tasks

import (
	"encoding/binary"
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// macOS has no /proc, so the socket table comes from the same sysctl the
// system's own netstat reads. Verified against `netstat -an` on macOS 15
// (arm64) as an ordinary user: the listening set matches exactly, port for
// port, and it includes services a port sweep can never find — mDNS on UDP
// 5353 is bound and listening and answers nothing unsolicited.
//
// It needs no privilege and no cgo, unlike seatbelt_darwin.go next door, so it
// works under CGO_ENABLED=0 too.
const (
	oidTCPPcbList = "net.inet.tcp.pcblist_n"
	oidUDPPcbList = "net.inet.udp.pcblist_n"
)

// The pcblist_n stream is self-describing: a 24-byte xinpgen header, then
// records each prefixed with {u32 length, u32 kind}. Several kinds describe the
// same socket in turn, so a walk keeps the one carrying addresses and skips the
// rest without needing to know their layouts.
const (
	xinpgenHeaderLen = 24
	xsoINPCB         = 0x010 // struct xinpcb_n: the addresses and ports

	// Offsets within an xinpcb_n record, confirmed byte by byte against a
	// live kernel rather than a header file: struct xinpcb_n is XNU-private
	// and absent from the public SDK.
	offInpFport = 16 // u16, network byte order
	offInpLport = 18 // u16, network byte order
	offInpVflag = 44 // u8
	offInpLaddr = 64 // 16-byte union; IPv4 occupies the last 4 bytes

	inpIPv4 = 0x1
	inpIPv6 = 0x2

	// Everything up to and including the local address union must be present
	// before any field is read.
	minINPCBLen = offInpLaddr + 16
)

func listLocalListeners() ([]Listener, error) {
	tcp, errT := pcbListeners(oidTCPPcbList, "tcp")
	udp, errU := pcbListeners(oidUDPPcbList, "udp")
	// One protocol failing must not hide the other. Both failing is a real
	// "could not measure", which the caller reports rather than publishing
	// an empty list that would score as a sandbox blocking everything.
	if errT != nil && errU != nil {
		return nil, fmt.Errorf("tcp: %w; udp: %v", errT, errU)
	}
	return append(tcp, udp...), nil
}

func pcbListeners(oid, proto string) ([]Listener, error) {
	b, err := unix.SysctlRaw(oid)
	if err != nil {
		// A Seatbelt profile can deny these oids by name, which is a
		// measurement in itself and must not be flattened to "nothing
		// was listening".
		return nil, fmt.Errorf("sysctl %s: %w", oid, err)
	}
	if len(b) < xinpgenHeaderLen {
		return nil, fmt.Errorf("sysctl %s: %d bytes, shorter than the xinpgen header", oid, len(b))
	}

	var out []Listener
	for off := xinpgenHeaderLen; off+8 <= len(b); {
		rlen := int(binary.LittleEndian.Uint32(b[off : off+4]))
		kind := binary.LittleEndian.Uint32(b[off+4 : off+8])
		if rlen < 8 || off+rlen > len(b) {
			// The stream ends with a trailing xinpgen that does not
			// parse as a record. Stopping here is the normal exit,
			// not an error.
			break
		}
		if kind == xsoINPCB && rlen >= minINPCBLen {
			if l, ok := inpcbListener(b[off:off+rlen], proto); ok {
				out = append(out, l)
			}
		}
		// Records are 8-byte aligned and the length does NOT include the
		// padding: xtcpcb_n is 204 bytes and occupies 208. Advancing by
		// the raw length desynchronises the walk a few records in and
		// then decodes noise as ports.
		off += (rlen + 7) &^ 7
	}
	return out, nil
}

func inpcbListener(rec []byte, proto string) (Listener, bool) {
	// A socket with a foreign port is a conversation, not a service. This one
	// test removes every connected client socket, which is what made the old
	// scan report a different random high port on every run.
	if binary.BigEndian.Uint16(rec[offInpFport:offInpFport+2]) != 0 {
		return Listener{}, false
	}
	lport := binary.BigEndian.Uint16(rec[offInpLport : offInpLport+2])
	if lport == 0 {
		return Listener{}, false
	}

	var ip net.IP
	switch vflag := rec[offInpVflag]; {
	case vflag&inpIPv4 != 0:
		// in_addr_4in6 keeps the v4 address in the last 4 bytes of the
		// 16-byte union.
		ip = net.IP(rec[offInpLaddr+12 : offInpLaddr+16]).To4()
	case vflag&inpIPv6 != 0:
		ip = net.IP(rec[offInpLaddr : offInpLaddr+16])
	default:
		// An address family we do not recognise: report the port, which
		// is what the finding carries, and leave the address blank
		// rather than inventing one.
		return Listener{Proto: proto, Port: int(lport)}, true
	}
	return Listener{Proto: proto, Addr: bindAddr(ip), Port: int(lport)}, true
}

// networkNamespace has no macOS equivalent. Returning "" says so, rather than
// implying an unnamespaced host.
func networkNamespace() string { return "" }

// inventorySource names where the table came from, for the report.
func inventorySource() string { return "sysctl-pcblist" }
