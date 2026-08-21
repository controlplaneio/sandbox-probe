package tasks

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// A port decoy is a listening TCP socket planted on the host before a scan, so
// that "this sandbox cannot reach local services" becomes provable rather than
// assumed.
//
// Why it is needed at all: the only thing the matrix could previously compare
// against was whatever the runner happened to be running. On GitHub's images
// that is sshd on 22, which is an accident of the image rather than something
// anyone controls — the Windows runner's set churns completely between runs,
// macOS harness rows saw port 22 anywhere from one run in five to five in
// five, and on a developer's laptop there is no sshd at all because Remote
// Login is off by default. A decoy we plant ourselves is the same target on
// every OS, in every row, on every machine. That is what parity means.
//
// It is bound on 0.0.0.0 rather than loopback deliberately. The probe then
// attempts BOTH 127.0.0.1 and the host's own address, and the pair separates
// two cases a single target cannot:
//
//	Docker's default bridge   refused on loopback, reachable on the host
//	                          address — the container has its own loopback, so
//	                          this is the classic loopback-isolation bypass
//	Seatbelt allow-localhost  the exact inverse
//
// No third-party machine is contacted. The listener is ours, on this host.
// decoyPortLifetime matches the process-decoy belt: the server exits on its own
// regardless of whether cleanup runs, so a crashed run cannot leave a listener
// behind indefinitely.
var decoyPortLifetime = decoyProcessLifetime

const (
	// portServerStartTimeout is how long seeding waits for the spawned
	// server to actually be listening. This is the one genuinely new piece
	// of engineering next to serve-pipe: exec.Cmd.Start returns as soon as
	// the process exists, which is BEFORE it has bound anything. A scan that
	// raced ahead would find the port refused and record a decoy that was
	// never planted as a sandbox blocking it — manufacturing exactly the
	// false negative this work exists to remove.
	portServerStartTimeout = 15 * time.Second
)

// seededPort is the cleanup record. The pid and its creation time together are
// what make removal safe on a reused runner: a pid alone can have been recycled
// onto somebody else's process by the time cleanup runs.
type seededPort struct {
	Port    int   `json:"port"`
	PID     int   `json:"pid"`
	Created int64 `json:"created"`
}

var errPortDecoyUnavailable = errors.New("no free port available for a decoy")

// ServePort binds port on all interfaces and holds it for d, then exits.
//
// It accepts and immediately closes connections rather than leaving them
// queued, so a probe's connect succeeds cleanly instead of sitting in the
// backlog, and so a scan cannot exhaust the listen queue and see later
// attempts fail for a reason that has nothing to do with policy.
func ServePort(port int, d time.Duration) error {
	ln, err := net.Listen("tcp", net.JoinHostPort("0.0.0.0", strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("bind decoy port %d: %w", port, err)
	}
	defer ln.Close()

	done := time.After(d)
	go func() {
		<-done
		ln.Close() // unblocks Accept below
	}()
	for {
		c, err := ln.Accept()
		if err != nil {
			return nil // listener closed: lifetime expired
		}
		c.Close()
	}
}

// seedPort plants one decoy and returns its record.
//
// Soft-plant discipline, the same rule every other decoy kind follows: the port
// is one the kernel has just confirmed is free, so nothing real is displaced. If
// it cannot get one, that is one skipped entry rather than a failed run.
func seedPort() (seededPort, error) {
	port, err := freeDecoyPort()
	if err != nil {
		return seededPort{}, err
	}
	exe, err := os.Executable()
	if err != nil {
		return seededPort{}, err
	}
	cmd := exec.Command(exe, "serve-port", strconv.Itoa(port), decoyPortLifetime.String())
	if err := cmd.Start(); err != nil {
		return seededPort{}, fmt.Errorf("spawn decoy port server: %w", err)
	}
	pid := cmd.Process.Pid

	// Wait for the bind BEFORE releasing the handle: a server that never came
	// up has to be killed, and the handle is what kills it portably.
	if err := waitForPort(port, portServerStartTimeout); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return seededPort{}, err
	}

	// Now detach. The server must outlive the seeding process, the same
	// lifecycle the pipe and process decoys use, and Release rather than Wait
	// leaves no zombie behind.
	created, cErr := processCreated(pid)
	if cErr != nil {
		created = 0 // cleanup falls back to matching on the pid alone
	}
	_ = cmd.Process.Release()
	return seededPort{Port: port, PID: pid, Created: created}, nil
}

// freeDecoyPort asks the kernel for an unused port and hands it straight back,
// so the number is known free at the moment of asking. There is an unavoidable
// window between releasing it and the server binding it; waitForPort is what
// turns losing that race into an error rather than a phantom decoy.
func freeDecoyPort() (int, error) {
	l, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		return 0, errPortDecoyUnavailable
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		return 0, errPortDecoyUnavailable
	}
	return port, nil
}

// waitForPort blocks until something answers on the port, or the timeout. A
// decoy the scan cannot see yet is a decoy that was never planted.
func waitForPort(port int, timeout time.Duration) error {
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			c.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("decoy port %d did not start listening within %s", port, timeout)
}

// hostAddresses returns this host's non-loopback IPv4 addresses, which is what
// the probe dials as the second target. Empty is a legitimate answer — a fully
// isolated container has no such address — and the caller reports that rather
// than inventing one.
func hostAddresses() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var out []string
	for _, a := range addrs {
		n, ok := a.(*net.IPNet)
		if !ok || n.IP.IsLoopback() || n.IP.To4() == nil {
			continue
		}
		out = append(out, n.IP.String())
	}
	return out
}
