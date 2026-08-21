package tasks

import (
	"errors"
	"net"
	"os"
	"runtime"
	"syscall"
	"testing"
)

// A real listener is reachable; a port nothing holds is refused. These are the
// two anchors every other outcome is read against.
func TestAttemptReachableAndRefused(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	if o, _, e := attempt("tcp", "127.0.0.1", port); o != OutcomeReachable {
		t.Errorf("live listener: got %s (%s), want reachable", o, e)
	}

	free, ok := freeLoopbackPort()
	if !ok {
		t.Skip("could not obtain a known-free port")
	}
	o, _, e := attempt("tcp", "127.0.0.1", free)
	if o != OutcomeRefused {
		t.Errorf("unbound port: got %s (%s), want refused", o, e)
	}
}

// The distinction the old scanner could not make. Both of these produce no
// connection, and treating them the same is what turned a probe running out of
// file descriptors into a report that a sandbox had blocked something.
func TestClassifySeparatesPolicyFromResourceFromSilence(t *testing.T) {
	wrap := func(e error) error {
		return &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", e)}
	}
	cases := []struct {
		name string
		err  error
		want Outcome
	}{
		{"nil is the only proof of reach", nil, OutcomeReachable},
		{"resource exhaustion is never a verdict", wrap(syscall.EMFILE), OutcomeProbeError},
		{"deadline is ambiguous, not open", wrap(os.ErrDeadlineExceeded), OutcomeSilent},
	}
	for _, c := range cases {
		if got, _, _ := classify(c.err); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}

	// Platform-specific errnos, checked through the same helpers production
	// uses. On Windows the syscall.E* constants are Go-synthesised values
	// Winsock never returns, so comparing against them directly would
	// compile and silently never match.
	if runtime.GOOS != "windows" {
		if got, _, _ := classify(wrap(syscall.EPERM)); got != OutcomeBlocked {
			t.Errorf("EPERM: got %s, want blocked", got)
		}
		if got, _, _ := classify(wrap(syscall.ENETUNREACH)); got != OutcomeUnreachable {
			t.Errorf("ENETUNREACH: got %s, want unreachable", got)
		}
		if got, _, _ := classify(wrap(syscall.ECONNREFUSED)); got != OutcomeRefused {
			t.Errorf("ECONNREFUSED: got %s, want refused", got)
		}
	}
}

// classify must see through wrapping. net returns *net.OpError around
// *os.SyscallError around syscall.Errno, and a type assertion at any one layer
// breaks the moment another is added.
func TestClassifyUnwrapsNestedErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX errno values are synthesised on Windows")
	}
	err := &net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}
	o, call, name := classify(err)
	if o != OutcomeRefused {
		t.Errorf("got %s, want refused", o)
	}
	if call != "connect" {
		t.Errorf("syscall = %q, want connect — it says how far the attempt got", call)
	}
	if name != "ECONNREFUSED" {
		t.Errorf("errno name = %q, want ECONNREFUSED", name)
	}
	if !errors.Is(err, syscall.ECONNREFUSED) {
		t.Error("sanity: the wrapped error should still match with errors.Is")
	}
}

// Only a completed connection counts as exposure.
//
// A refusal proves something answered, but not that policy allowed it: an
// iptables REJECT forges a TCP reset, and a seccomp-notify supervisor can
// return any errno it chooses for connect(). Counting refusals as reach would
// let a sandbox that blocks everything by forging refusals read as wide open.
func TestOnlyReachableIsScored(t *testing.T) {
	rs := []PortResult{
		{Proto: "tcp", Port: 22, Outcome: OutcomeReachable},
		{Proto: "tcp", Port: 80, Outcome: OutcomeRefused},
		{Proto: "tcp", Port: 443, Outcome: OutcomeBlocked},
		{Proto: "tcp", Port: 8080, Outcome: OutcomeSilent},
		{Proto: "tcp", Port: 9090, Outcome: OutcomeProbeError},
		{Proto: "udp", Port: 53, Outcome: OutcomeReachable},
	}
	got := reachablePorts(rs, "tcp")
	if len(got) != 1 || got[0] != 22 {
		t.Errorf("scored ports = %v, want [22]: only a completed connection is evidence", got)
	}
}

// The branch that decides whether a UDP silence means anything.
func TestUDPSilenceNeedsAProvenFeedbackChannel(t *testing.T) {
	results := []PortResult{
		{Proto: "udp", Port: 5353, Outcome: OutcomeSilent},
		{Proto: "udp", Port: 123, Outcome: OutcomeReachable},
	}
	mk := func(feedback string) LocalServices {
		return LocalServices{
			Results: results,
			Status:  map[string]any{"table": "read", "udp_feedback": feedback},
		}
	}

	// Control refused: the channel is live, so a bound port that stays quiet
	// was reached — the service just had nothing to say.
	got, ok := mk("working").UDPPorts()
	if !ok || len(got) != 2 {
		t.Errorf("feedback working: got %v ok=%v, want both ports", got, ok)
	}

	// ICMP dropped: only an actual reply counts.
	got, ok = mk("suppressed").UDPPorts()
	if !ok || len(got) != 1 || got[0] != 123 {
		t.Errorf("feedback suppressed: got %v ok=%v, want [123]", got, ok)
	}

	// Neither control refused. Nothing can be concluded, so nothing is
	// reported — an empty list here would claim a measured negative that
	// was never measured.
	got, ok = mk("unknown").UDPPorts()
	if ok {
		t.Errorf("feedback unknown: got %v, want no finding emitted at all", got)
	}
}

// A table that could not be read must not produce a finding either.
func TestUnreadableTableEmitsNoPorts(t *testing.T) {
	s := LocalServices{Status: map[string]any{"table": "denied", "udp_feedback": "working"}}
	if _, ok := s.TCPPorts(); ok {
		t.Error("a denied socket table must not yield a tcp finding")
	}
	if _, ok := s.UDPPorts(); ok {
		t.Error("a denied socket table must not yield a udp finding")
	}
}

// The control's whole job is to be interpretable.
func TestControlFeedbackRules(t *testing.T) {
	cases := []struct {
		c    control
		want string
	}{
		{control{tcp: OutcomeRefused, udp: OutcomeRefused}, "working"},
		{control{tcp: OutcomeRefused, udp: OutcomeSilent}, "suppressed"},
		{control{tcp: OutcomeSilent, udp: OutcomeSilent}, "unknown"},
		{control{tcp: OutcomeBlocked, udp: OutcomeBlocked}, "unknown"},
	}
	for _, c := range cases {
		if got := c.c.feedback(); got != c.want {
			t.Errorf("control{tcp:%s udp:%s} = %q, want %q", c.c.tcp, c.c.udp, got, c.want)
		}
	}
}

// End to end against this machine's own kernel: a listener we just bound must
// be inventoried and then reached.
func TestMeasureLocalServicesFindsAndReachesOurListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	s := MeasureLocalServices()
	if s.Status["table"] == "unsupported" {
		t.Skipf("no socket-table reader on %s", runtime.GOOS)
	}
	if s.Status["table"] != "read" {
		t.Fatalf("table not read: %v", s.Status)
	}

	ports, ok := s.TCPPorts()
	if !ok {
		t.Fatal("a readable table must yield a tcp finding")
	}
	found := false
	for _, p := range ports {
		if p == port {
			found = true
		}
	}
	if !found {
		t.Errorf("our own listener on %d was inventoried but not reached; ports=%v", port, ports)
	}
	if s.Status["method"] != "kernel-inventory-v1" {
		t.Errorf("method = %v, want the cutover marker", s.Status["method"])
	}
}
