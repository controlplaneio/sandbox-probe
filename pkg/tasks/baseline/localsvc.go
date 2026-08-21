package tasks

import (
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	// probeDeadline is per attempt. Generous for loopback, where anything
	// alive answers in microseconds, and short enough that a whole pass
	// over a few dozen endpoints stays well under a second.
	probeDeadline = 500 * time.Millisecond
	// probeWorkers is deliberately small. The scanner this replaces used
	// 66,535 goroutines, each holding a file descriptor with a 3s deadline,
	// which is how a scan of localhost managed to exhaust the process's own
	// fd limit and then report the resulting failures as a sandbox blocking
	// things.
	probeWorkers = 16
)

// LocalServices is the whole local-services measurement: ask the kernel what
// is listening, prove the feedback channel works, then attempt each endpoint
// and classify what came back.
//
// The two halves answer different questions and must not be conflated. The
// inventory says what EXISTS in this network namespace. The attempts say what
// this process can REACH. Under a Seatbelt profile denying network the first is
// a full list and the second is empty; inside a network namespace the first is
// empty and nothing is denied at all. Reporting only one of them cannot tell
// those apart.
type LocalServices struct {
	Listeners []Listener
	Results   []PortResult
	Status    map[string]any
}

// MeasureLocalServices runs the measurement. It returns a result in every case,
// including total failure: a caller that emits nothing when it is unsure
// reproduces the bug this replaces, because a missing finding is scored as
// "the sandbox blocked it".
func MeasureLocalServices() LocalServices {
	out := LocalServices{Status: map[string]any{
		"method": "kernel-inventory-v1",
		"source": inventorySource(),
	}}
	if ns := networkNamespace(); ns != "" {
		out.Status["netns"] = ns
	}

	listeners, err := ListLocalListeners()
	switch {
	case err != nil && ErrUnsupportedInventory(err):
		out.Status["table"] = "unsupported"
		out.Status["error"] = err.Error()
		return out
	case err != nil:
		// Could not read the table. That is not an empty table, and the
		// difference is the whole point.
		out.Status["table"] = "denied"
		out.Status["error"] = err.Error()
		return out
	}
	out.Status["table"] = "read"
	out.Status["listeners_found"] = len(listeners)
	out.Listeners = listeners

	// Calibrate BEFORE probing anything real. A UDP timeout is meaningless
	// until the negative-feedback channel is known to work, and the control
	// must run first: probing real endpoints first would consume the ICMP
	// rate-limit budget and make the control look dead.
	//
	// The kernel inventory is what makes this trustworthy. The old sweep
	// fired at 65,535 closed ports and drained that budget every time, so a
	// control probe could never have been believed. Here the probe set is a
	// few dozen endpoints, so there is almost nothing to drain.
	cal := calibrate()
	out.Status["control_tcp"] = string(cal.tcp)
	out.Status["control_udp"] = string(cal.udp)
	out.Status["udp_feedback"] = cal.feedback()

	out.Results = probeAll(listeners)
	// structpb only accepts the basic Go kinds, so the per-port detail goes
	// in as plain maps rather than as []PortResult. Without this the whole
	// status finding fails to serialise at runtime while building fine.
	rows := make([]any, 0, len(out.Results))
	for _, r := range out.Results {
		row := map[string]any{
			"proto": r.Proto, "addr": r.Addr,
			"port": float64(r.Port), "outcome": string(r.Outcome),
		}
		if r.Syscall != "" {
			row["syscall"] = r.Syscall
		}
		if r.Errno != "" {
			row["errno"] = r.Errno
		}
		rows = append(rows, row)
	}
	out.Status["results"] = rows
	return out
}

// TCPPorts returns the TCP ports this process reached, and whether the
// measurement concluded at all. A false second return means the caller must
// emit no finding: the site reads an absent finding as unmeasured, and an
// empty one as a measured negative, so publishing [] here would claim a
// sandbox blocked something that was never tested.
func (s LocalServices) TCPPorts() ([]int, bool) {
	if s.Status["table"] != "read" {
		return nil, false
	}
	return reachablePorts(s.Results, "tcp"), true
}

// UDPPorts is the same for UDP, with one extra condition: silence only counts
// as reachable when the control proved the refusal channel works.
//
//	working     a bound port that stays quiet was reached; the service
//	            simply had nothing to say to an empty datagram
//	suppressed  ICMP is being dropped, so only an actual reply counts
//	unknown     nothing can be concluded from silence at all, so nothing
//	            is reported
//
// The "unknown" branch is the one that matters. A sandbox dropping everything
// uniformly gives silence on both controls; reporting [] there would read as
// "measured, nothing reachable", when in truth nothing was measured.
func (s LocalServices) UDPPorts() ([]int, bool) {
	if s.Status["table"] != "read" {
		return nil, false
	}
	switch s.Status["udp_feedback"] {
	case "working":
		seen := map[int]struct{}{}
		out := []int{}
		for _, r := range s.Results {
			if r.Proto != "udp" {
				continue
			}
			if r.Outcome != OutcomeReachable && r.Outcome != OutcomeSilent {
				continue
			}
			if _, dup := seen[r.Port]; dup {
				continue
			}
			seen[r.Port] = struct{}{}
			out = append(out, r.Port)
		}
		sortInts(out)
		return out, true
	case "suppressed":
		return reachablePorts(s.Results, "udp"), true
	default:
		return nil, false
	}
}

// control is what an endpoint that is known NOT to be listening returns, which
// is what makes every other outcome interpretable.
type control struct{ tcp, udp Outcome }

// feedback says whether a UDP silence can be read as "reachable but quiet".
//
//	working  a refusal came back, so the channel is live
//	suppressed  TCP refused but UDP did not, so ICMP is being dropped
//	unknown  neither refused: nothing can be concluded from UDP silence
//
// The "unknown" case is the one that matters and is easy to get wrong. A
// sandbox that drops everything uniformly gives silence on both controls.
// Reporting an empty port list there would say "measured nothing reachable",
// which reads as a blocked capability. Nothing was measured at all.
func (c control) feedback() string {
	switch {
	case c.udp == OutcomeRefused:
		return "working"
	case c.tcp == OutcomeRefused:
		return "suppressed"
	default:
		return "unknown"
	}
}

func calibrate() control {
	port, ok := freeLoopbackPort()
	if !ok {
		return control{tcp: OutcomeProbeError, udp: OutcomeProbeError}
	}
	t, _, _ := attempt("tcp", "127.0.0.1", port)
	u, _, _ := attempt("udp", "127.0.0.1", port)
	return control{tcp: t, udp: u}
}

// freeLoopbackPort asks the kernel for a port and immediately gives it back, so
// the number is known to be unbound. If binding is denied — which is exactly
// what a tight sandbox does — there is no control to run.
func freeLoopbackPort() (int, bool) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, false
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		return 0, false
	}
	return port, true
}

func probeAll(ls []Listener) []PortResult {
	jobs := make(chan Listener)
	results := make(chan PortResult, len(ls))
	var wg sync.WaitGroup
	for i := 0; i < probeWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for l := range jobs {
				o, call, errno := attempt(l.Proto, dialAddr(l), l.Port)
				results <- PortResult{
					Proto: l.Proto, Addr: dialAddr(l), Port: l.Port,
					Outcome: o, Syscall: call, Errno: errno,
				}
			}
		}()
	}
	go func() {
		for _, l := range ls {
			jobs <- l
		}
		close(jobs)
	}()
	go func() { wg.Wait(); close(results) }()

	out := make([]PortResult, 0, len(ls))
	for r := range results {
		out = append(out, r)
	}
	return out
}

// dialAddr picks what to actually connect to. A wildcard bind is reachable on
// loopback; a bind to one address has to be dialled at that address.
//
// Never the name "localhost": resolving it can yield ::1 or 127.0.0.1 in either
// order, and a probe that silently dialled a different family than the one the
// service is bound to is a strong candidate for the intermittent misses in the
// published series.
func dialAddr(l Listener) string {
	if l.Addr == "" {
		return "127.0.0.1"
	}
	return l.Addr
}

// attempt makes one connection and classifies the result.
//
// For UDP a connected socket is required: an unconnected one never surfaces
// ICMP errors at all. Connect, write, then read with a deadline — the refusal
// arrives on the read, not the write.
func attempt(proto, host string, port int) (Outcome, string, string) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout(proto, addr, probeDeadline)
	if err != nil {
		return classify(err)
	}
	defer conn.Close()

	if proto != "udp" {
		return OutcomeReachable, "", ""
	}

	// Windows suppresses ICMP errors on UDP sockets by default; ask for them
	// back. If that fails the socket still works, but its silence carries no
	// information, which the status already records.
	_ = enableUDPErrorReporting(conn)

	if _, err := conn.Write([]byte{}); err != nil {
		return classify(err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(probeDeadline)); err != nil {
		return classify(err)
	}
	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err != nil {
		o, call, errno := classify(err)
		if o == OutcomeSilent {
			// Silence on a socket the kernel says is bound. Whether
			// that means reachable depends on the control, which the
			// caller applies; on its own it is not evidence.
			return OutcomeSilent, call, errno
		}
		return o, call, errno
	}
	return OutcomeReachable, "", ""
}
