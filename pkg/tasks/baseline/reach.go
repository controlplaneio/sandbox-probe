package tasks

import (
	"errors"
	"net"
	"os"
	"syscall"
)

// Outcome is what a single connection attempt established. The old scanner had
// two states, open and not-open, and inferred "not-open" from silence — which
// is how a scan that timed out under load became a report that a sandbox had
// blocked something.
//
// These are outcomes of a MEASUREMENT, not verdicts about a sandbox. Only
// OutcomeReachable is evidence of exposure. See scoring note below.
type Outcome string

const (
	// OutcomeReachable: the connection completed. This is the only outcome
	// that proves the process can reach the service.
	OutcomeReachable Outcome = "reachable"

	// OutcomeRefused: something answered with a refusal — a TCP RST, or an
	// ICMP port-unreachable surfaced on a connected UDP socket.
	//
	// It is tempting to read this as "the packet got there, so nothing
	// blocked it". Do not. A refusal is forgeable by the very thing being
	// measured: iptables -j REJECT --reject-with tcp-reset synthesises an
	// RST, and a seccomp-notify supervisor (which is how nono works) can
	// return any errno it likes for connect(), including this one. So a
	// refusal means "something refused", which may be the service being
	// absent or may be policy wearing a disguise. Diagnostic only.
	OutcomeRefused Outcome = "refused"

	// OutcomeBlocked: policy rejected the attempt locally, before anything
	// left. Seatbelt's (deny network*) and an iptables OUTPUT owner rule
	// both surface here.
	OutcomeBlocked Outcome = "blocked"

	// OutcomeUnreachable: no route. Typical of a process in its own network
	// namespace with no path to the target.
	OutcomeUnreachable Outcome = "unreachable"

	// OutcomeSilent: nothing came back before the deadline. Genuinely
	// ambiguous — dropped, filtered, rate-limited, or merely slow. The old
	// scanner recorded this as "open", which is backwards.
	OutcomeSilent Outcome = "silent"

	// OutcomeProbeError: the probe itself failed — out of file descriptors,
	// no ephemeral ports left, out of buffers. Never a statement about the
	// sandbox. The old scanner swallowed these, which is why the published
	// corpus contains file-descriptor exhaustion rendered as "blocked".
	OutcomeProbeError Outcome = "probe_error"
)

// PortResult is one attempt against one inventoried endpoint.
type PortResult struct {
	Proto   string  `json:"proto"`
	Addr    string  `json:"addr"`
	Port    int     `json:"port"`
	Outcome Outcome `json:"outcome"`
	// Syscall is which call failed — "connect" means nothing left this
	// process, "read" means a datagram left and an answer came back. Free
	// depth information that the errno alone does not carry.
	Syscall string `json:"syscall,omitempty"`
	Errno   string `json:"errno,omitempty"`
}

// classify turns a dial or read error into an outcome.
//
// Order matters: a timeout is the ABSENCE of an errno, so every specific errno
// must be tested before falling through to the deadline case, or a refusal
// carrying a deadline-ish wrapper would be misread as silence.
func classify(err error) (Outcome, string, string) {
	if err == nil {
		return OutcomeReachable, "", ""
	}

	var se *os.SyscallError
	call := ""
	if errors.As(err, &se) {
		call = se.Syscall
	}

	var errno syscall.Errno
	if errors.As(err, &errno) {
		name := errnoName(errno)
		switch {
		case isRefused(errno):
			return OutcomeRefused, call, name
		case isPermission(errno):
			return OutcomeBlocked, call, name
		case isUnreachable(errno):
			return OutcomeUnreachable, call, name
		case isResourceExhaustion(errno):
			return OutcomeProbeError, call, name
		case isTimeout(errno):
			return OutcomeSilent, call, name
		}
		// An errno we have no opinion on is not silence and is not a
		// verdict. Say so rather than guessing.
		return OutcomeProbeError, call, name
	}

	// No errno: a deadline, or a wrapped error with nothing underneath.
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return OutcomeSilent, call, "deadline"
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return OutcomeSilent, call, "timeout"
	}
	return OutcomeProbeError, call, ""
}

// reachablePorts is what the tcp_ports_open / udp_ports_open findings carry:
// ports this process actually reached.
//
// ONLY OutcomeReachable counts. A refusal is excluded deliberately, even
// though it proves a packet was answered, because the answer is forgeable by a
// REJECT firewall or a seccomp-notify supervisor — so counting it would let a
// sandbox that blocks everything by forging refusals read as exposure. The
// refusals are still reported, in the per-port results, where they inform a
// reader without scoring anything.
func reachablePorts(rs []PortResult, proto string) []int {
	seen := map[int]struct{}{}
	out := []int{}
	for _, r := range rs {
		if r.Proto != proto || r.Outcome != OutcomeReachable {
			continue
		}
		if _, dup := seen[r.Port]; dup {
			continue
		}
		seen[r.Port] = struct{}{}
		out = append(out, r.Port)
	}
	sortInts(out)
	return out
}

func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}
