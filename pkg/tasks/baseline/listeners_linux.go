//go:build linux
// +build linux

package tasks

import (
	"fmt"
	"net"
	"os"

	"github.com/prometheus/procfs"
)

// Socket states as the kernel writes them into /proc/net/*, hex in the "st"
// column. Only two matter here.
const (
	// tcpListen is TCP_LISTEN. A TCP socket in any other state is a
	// connection, not a service.
	tcpListen = 0x0A
	// tcpClose is TCP_CLOSE, which is what every UDP socket reports because
	// UDP has no state machine. It is necessary but not sufficient: an
	// unconnected UDP socket and a connected one both sit here, so the
	// remote port is what separates them.
	tcpClose = 0x07
)

// listLocalListeners reads the kernel's own socket tables.
//
// /proc/net is a view of the CURRENT NETWORK NAMESPACE, not of the host. That
// is the single most useful property here: a sandbox that puts the process in
// its own namespace produces an empty table, honestly, with nothing denied and
// no error — while a sandbox that shares the host's namespace but blocks
// connect() produces a full table the process cannot use. Those are different
// findings and the old port sweep could not tell them apart. The namespace
// identity is recorded separately by the caller so a report can say which of
// the two it is.
func listLocalListeners() ([]Listener, error) {
	// procfs.NewFS does not fail on a /proc that is absent, masked or
	// synthetic — it only stats the directory — so a successful call here
	// says nothing. Read failure below is the real readability signal.
	fs, err := procfs.NewFS("/proc")
	if err != nil {
		return nil, fmt.Errorf("open /proc: %w", err)
	}

	var out []Listener
	var firstErr error
	add := func(proto string, rows procfs.NetTCP, err error, keep func(st, remPort uint64) bool) {
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return
		}
		for _, r := range rows {
			if r == nil || !keep(r.St, r.RemPort) {
				continue
			}
			out = append(out, Listener{Proto: proto, Addr: bindAddr(r.LocalAddr), Port: int(r.LocalPort)})
		}
	}

	isTCPService := func(st, _ uint64) bool { return st == tcpListen }
	// The remote-port test is what removes the run-to-run churn that made
	// this measurement unreadable: every Linux baseline reported a different
	// random high UDP port each run (58026, 37631, 59801, 39154...). Those
	// were connected client sockets belonging to other processes, which
	// carry a peer. A service does not.
	isUDPService := func(st, remPort uint64) bool { return st == tcpClose && remPort == 0 }

	if err := readTables(fs, add, isTCPService, isUDPService); err != nil {
		return nil, err
	}
	// A table that could not be read is not an empty table. Surfacing the
	// error lets the caller report "could not measure" instead of publishing
	// a silence that scores as "the sandbox blocked everything".
	if firstErr != nil && len(out) == 0 {
		return nil, firstErr
	}
	return out, nil
}

// readTables walks the four socket files, guarding the one place upstream can
// panic on us.
//
// ponytail: recover() rather than our own parser. procfs's
// parseNetIPSocketLine validates len(fields) < 10 and then indexes fields[12]
// for UDP, so a /proc/net/udp with 10 to 12 columns panics instead of
// erroring. Synthetic and emulated procfs implementations do vary here, and
// there is no recover() anywhere else in this repository, so an unguarded
// panic would lose the whole report rather than this one finding. Three lines
// against roughly twenty-five for a tolerant reimplementation; write the
// parser only if a real report shows this firing.
func readTables(
	fs procfs.FS,
	add func(string, procfs.NetTCP, error, func(uint64, uint64) bool),
	isTCPService, isUDPService func(uint64, uint64) bool,
) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("procfs socket table parser panicked: %v", r)
		}
	}()
	t4, e := fs.NetTCP()
	add("tcp", t4, e, isTCPService)
	t6, e := fs.NetTCP6()
	add("tcp", procfs.NetTCP(t6), e, isTCPService)
	u4, e := fs.NetUDP()
	add("udp", procfs.NetTCP(u4), e, isUDPService)
	u6, e := fs.NetUDP6()
	add("udp", procfs.NetTCP(u6), e, isUDPService)
	return nil
}

// bindAddr renders the bound address, collapsing the two wildcard forms to ""
// so a caller can tell "bound everywhere" from "bound to one interface"
// without parsing IPs again.
func bindAddr(ip net.IP) string {
	if ip == nil || ip.IsUnspecified() {
		return ""
	}
	return ip.String()
}

// networkNamespace returns the current network namespace's identity, e.g.
// "net:[4026531840]". Two reports carrying different values are looking at
// different networks, which is what makes an empty socket table interpretable:
// isolated, rather than denied or broken.
func networkNamespace() string {
	ns, err := os.Readlink("/proc/self/ns/net")
	if err != nil {
		return ""
	}
	return ns
}
