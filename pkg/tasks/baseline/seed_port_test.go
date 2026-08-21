package tasks

import (
	"net"
	"os"
	"strconv"
	"testing"
	"time"
)

// The decoy has to be listening by the time seedPort returns, and it has to be
// visible to the kernel inventory, or the scan that follows records a decoy
// that was never planted as a sandbox blocking it.
//
// This is the property serve-pipe does not need and serve-port does:
// exec.Cmd.Start returns as soon as the process exists, which is before it has
// bound anything.
func TestServePortIsListeningBeforeSeedReturns(t *testing.T) {
	d, err := seedPort()
	if err != nil {
		t.Skipf("could not plant a port decoy here: %v", err)
	}
	defer stopDecoy(t, d.PID)

	if d.Port <= 0 {
		t.Fatalf("seeded port is %d", d.Port)
	}
	if d.PID <= 0 {
		t.Fatalf("seeded pid is %d", d.PID)
	}

	// Reachable immediately, with no retry: that is the whole point.
	c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(d.Port)), time.Second)
	if err != nil {
		t.Fatalf("decoy port %d not reachable the instant seeding returned: %v", d.Port, err)
	}
	c.Close()

	// And the kernel inventory must see it, since that is what selects it as
	// a probe target.
	ls, err := ListLocalListeners()
	if err != nil {
		if ErrUnsupportedInventory(err) {
			t.Skip("no socket-table reader on this platform")
		}
		t.Fatalf("inventory: %v", err)
	}
	found := false
	for _, l := range ls {
		if l.Proto == "tcp" && l.Port == d.Port {
			found = true
		}
	}
	if !found {
		t.Errorf("decoy port %d is listening but absent from the kernel inventory", d.Port)
	}
}

// It binds the wildcard address, not loopback. That is what lets the probe
// attempt both 127.0.0.1 and the host's own address, which is the pair that
// separates Docker's loopback isolation from a Seatbelt allow-localhost policy.
func TestServePortBindsWildcardNotJustLoopback(t *testing.T) {
	d, err := seedPort()
	if err != nil {
		t.Skipf("could not plant a port decoy here: %v", err)
	}
	defer stopDecoy(t, d.PID)

	ls, err := ListLocalListeners()
	if err != nil {
		if ErrUnsupportedInventory(err) {
			t.Skip("no socket-table reader on this platform")
		}
		t.Fatalf("inventory: %v", err)
	}
	for _, l := range ls {
		if l.Proto == "tcp" && l.Port == d.Port {
			if l.Addr != "" {
				t.Errorf("decoy bound to %q, want the wildcard: a loopback-only decoy "+
					"cannot distinguish container loopback isolation from a policy block", l.Addr)
			}
			return
		}
	}
	t.Skip("decoy not visible in the inventory on this platform")
}

// Soft-plant discipline: the port must be one the kernel says is free, so a
// decoy never displaces something real.
func TestFreeDecoyPortIsActuallyFree(t *testing.T) {
	port, err := freeDecoyPort()
	if err != nil {
		t.Skipf("cannot obtain a port here: %v", err)
	}
	ln, err := net.Listen("tcp", net.JoinHostPort("0.0.0.0", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("port %d was reported free but could not be bound: %v", port, err)
	}
	ln.Close()
}

// waitForPort must fail rather than hang when nothing ever binds, so a failed
// plant is an error the caller can act on instead of a phantom decoy.
func TestWaitForPortTimesOutOnNothing(t *testing.T) {
	port, err := freeDecoyPort()
	if err != nil {
		t.Skipf("cannot obtain a port here: %v", err)
	}
	start := time.Now()
	if err := waitForPort(port, 300*time.Millisecond); err == nil {
		t.Error("waiting on a port nothing binds must fail")
	}
	if el := time.Since(start); el > 3*time.Second {
		t.Errorf("waited %s, far past the timeout", el)
	}
}

// hostAddresses is the second probe target. It may legitimately be empty in a
// fully isolated container, which the caller reports rather than papering over.
func TestHostAddressesAreNonLoopbackIPv4(t *testing.T) {
	for _, a := range hostAddresses() {
		ip := net.ParseIP(a)
		if ip == nil {
			t.Errorf("%q is not an IP", a)
			continue
		}
		if ip.IsLoopback() {
			t.Errorf("%s is loopback; the whole point is to dial a non-loopback address", a)
		}
		if ip.To4() == nil {
			t.Errorf("%s is not IPv4", a)
		}
	}
}

func stopDecoy(t *testing.T, pid int) {
	t.Helper()
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = p.Kill()
}
