package tasks

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
)

// A Listener is one socket the kernel says is bound and offering a service, as
// reported by the operating system's own socket table rather than by dialling
// anything.
//
// This exists because the alternative — sweeping every port and calling
// whatever answers "open" — cannot work, and provably did not. A UDP service
// that never replies to an unsolicited datagram (mDNS on 5353 is the obvious
// one) is invisible to a sweep at any concurrency with any timeout, while a
// sweep that treats silence as "open" reports thousands of ports that were
// never bound at all. The kernel already knows the answer exactly; asking it
// costs one read and no packets.
//
// Reading the table answers "what is listening". It does NOT answer "can this
// confined process reach it", and the two diverge under exactly the sandboxes
// this probe exists to measure: a macOS Seatbelt profile that denies network
// leaves the socket table byte-identical while connect() returns EPERM, and a
// Linux network namespace empties the table while nothing is denied at all.
// So an inventory is target selection. It is never the scored result.
type Listener struct {
	// Proto is "tcp" or "udp". Address family is folded in: a dual-stack
	// listener appears once per family it is actually bound on.
	Proto string
	// Addr is the bound address as the kernel reports it — "127.0.0.1",
	// "::1", or "" for a wildcard bind. Kept because it decides what a
	// reachability probe should dial: a wildcard bind is reachable on
	// loopback, a bind to a specific interface address may not be.
	Addr string
	Port int
}

// String renders one entry for the local_listeners finding, matching the
// house style of unix_socket_detection and named_pipe_detection: one legible
// line per entry, so the site's existing renderer needs no change.
func (l Listener) String() string {
	addr := l.Addr
	if addr == "" {
		addr = "*"
	}
	if ip := net.ParseIP(addr); ip != nil && ip.To4() == nil {
		addr = "[" + addr + "]"
	}
	return l.Proto + " " + addr + ":" + strconv.Itoa(l.Port)
}

// errUnsupportedInventory is what platforms without an implemented socket-table
// reader return. It is deliberately distinct from "the table was empty": an
// empty table is a measurement, and a missing implementation is not. Reporting
// them the same way is the bug this whole seam exists to remove.
var errUnsupportedInventory = errors.New("kernel socket table enumeration is not implemented on this platform")

// ErrUnsupportedInventory reports whether err means the platform has no
// implementation, as opposed to a table that could not be read.
func ErrUnsupportedInventory(err error) bool { return errors.Is(err, errUnsupportedInventory) }

// ListLocalListeners returns every bound TCP and UDP socket the running
// process's kernel view contains, sorted and deduplicated.
//
// "The running process's view" is load-bearing on Linux: /proc/net is
// namespace-scoped, so inside a network namespace this legitimately returns
// nothing. That is a real measurement, not a failure, and telling the two
// apart is why the caller also records the namespace identity.
func ListLocalListeners() ([]Listener, error) {
	ls, err := listLocalListeners()
	if err != nil {
		return nil, err
	}
	return normalizeListeners(ls), nil
}

// normalizeListeners sorts and dedupes. Sorting matters beyond tidiness: the
// previous scanner appended results from 66,535 racing goroutines, so the port
// order varied between runs of an identical scan and any consumer diffing two
// reports saw a change that had not happened.
func normalizeListeners(ls []Listener) []Listener {
	seen := make(map[string]struct{}, len(ls))
	out := make([]Listener, 0, len(ls))
	for _, l := range ls {
		if l.Port <= 0 || l.Port > 65535 {
			continue // a parse that produced a nonsense port is dropped, not reported
		}
		k := fmt.Sprintf("%s|%s|%d", l.Proto, l.Addr, l.Port)
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Proto != out[j].Proto {
			return out[i].Proto < out[j].Proto
		}
		if out[i].Port != out[j].Port {
			return out[i].Port < out[j].Port
		}
		return out[i].Addr < out[j].Addr
	})
	return out
}

// ListenerPorts returns the distinct ports for one protocol, which is the shape
// the tcp_ports_open / udp_ports_open findings have always carried.
func ListenerPorts(ls []Listener, proto string) []int {
	seen := map[int]struct{}{}
	out := []int{}
	for _, l := range ls {
		if l.Proto != proto {
			continue
		}
		if _, dup := seen[l.Port]; dup {
			continue
		}
		seen[l.Port] = struct{}{}
		out = append(out, l.Port)
	}
	sort.Ints(out)
	return out
}
