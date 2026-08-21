package tasks

import (
	"net"
	"runtime"
	"testing"
)

// Bind real sockets and require the kernel's own table to report them.
//
// This is the only check that can catch the macOS reader rotting: struct
// xinpcb_n is XNU-private, absent from the public SDK, and has grown across
// releases, so the field offsets are pinned against a live kernel here rather
// than against a captured byte dump. A dump would keep passing while the kernel
// moved underneath it.
func TestListLocalListenersSeesRealSockets(t *testing.T) {
	tcpLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("bind tcp: %v", err)
	}
	defer tcpLn.Close()
	udpConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("bind udp: %v", err)
	}
	defer udpConn.Close()

	wantTCP := tcpLn.Addr().(*net.TCPAddr).Port
	wantUDP := udpConn.LocalAddr().(*net.UDPAddr).Port

	got, err := ListLocalListeners()
	if err != nil {
		if ErrUnsupportedInventory(err) {
			t.Skipf("no socket-table reader on %s yet", runtime.GOOS)
		}
		t.Fatalf("ListLocalListeners: %v", err)
	}

	if !hasListener(got, "tcp", wantTCP) {
		t.Errorf("a TCP socket bound on 127.0.0.1:%d is missing from the kernel table", wantTCP)
	}
	if !hasListener(got, "udp", wantUDP) {
		// The UDP half is the whole point: this is the class of socket the
		// old port sweep could never see, because a bound UDP service is
		// under no obligation to answer an unsolicited datagram.
		t.Errorf("a UDP socket bound on 127.0.0.1:%d is missing from the kernel table", wantUDP)
	}
}

// A closed socket must leave the table. Without this, a stale inventory would
// send the reachability half chasing ports nothing is listening on, and every
// one of them would come back refused — which reads as a sandbox blocking
// things it never touched.
func TestListLocalListenersDropsClosedSockets(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	got, err := ListLocalListeners()
	if err != nil {
		if ErrUnsupportedInventory(err) {
			t.Skipf("no socket-table reader on %s yet", runtime.GOOS)
		}
		t.Fatalf("ListLocalListeners: %v", err)
	}
	if hasListener(got, "tcp", port) {
		t.Errorf("port %d was closed but is still in the table", port)
	}
}

// Connected sockets are conversations, not services, and must not appear. This
// is the filter that removes the run-to-run churn that made the published
// series unreadable: every Linux baseline reported a different random high UDP
// port each run (58026, 37631, 59801, 39154), all of them other processes'
// client sockets.
func TestListLocalListenersExcludesConnectedSockets(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	defer ln.Close()

	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()
	server, err := ln.Accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	defer server.Close()

	got, err := ListLocalListeners()
	if err != nil {
		if ErrUnsupportedInventory(err) {
			t.Skipf("no socket-table reader on %s yet", runtime.GOOS)
		}
		t.Fatalf("ListLocalListeners: %v", err)
	}
	ephemeral := client.LocalAddr().(*net.TCPAddr).Port
	if hasListener(got, "tcp", ephemeral) {
		t.Errorf("ephemeral client port %d is a connection, not a service, and must not be listed", ephemeral)
	}
	if !hasListener(got, "tcp", ln.Addr().(*net.TCPAddr).Port) {
		t.Error("the accepting listener must still be listed while it has a connection")
	}
}

// The output feeds a report that is diffed between runs, so an unstable order
// would render as a change that did not happen. The old scanner appended from
// racing goroutines and did exactly that.
func TestListenerOutputIsSortedAndDeduped(t *testing.T) {
	in := []Listener{
		{Proto: "udp", Addr: "", Port: 5353},
		{Proto: "tcp", Addr: "127.0.0.1", Port: 8080},
		{Proto: "tcp", Addr: "127.0.0.1", Port: 22},
		{Proto: "tcp", Addr: "127.0.0.1", Port: 8080}, // duplicate
		{Proto: "tcp", Addr: "", Port: 0},             // invalid, dropped
		{Proto: "tcp", Addr: "", Port: 70000},         // out of range, dropped
	}
	got := normalizeListeners(in)
	want := []Listener{
		{Proto: "tcp", Addr: "127.0.0.1", Port: 22},
		{Proto: "tcp", Addr: "127.0.0.1", Port: 8080},
		{Proto: "udp", Addr: "", Port: 5353},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d listeners, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
	if ports := ListenerPorts(got, "tcp"); len(ports) != 2 || ports[0] != 22 || ports[1] != 8080 {
		t.Errorf("ListenerPorts(tcp) = %v, want [22 8080]", ports)
	}
}

// The rendered form is what lands in the report, one line per entry, matching
// the house style of unix_socket_detection and named_pipe_detection.
func TestListenerString(t *testing.T) {
	cases := []struct {
		in   Listener
		want string
	}{
		{Listener{Proto: "tcp", Addr: "127.0.0.1", Port: 22}, "tcp 127.0.0.1:22"},
		{Listener{Proto: "udp", Addr: "", Port: 5353}, "udp *:5353"},
		{Listener{Proto: "tcp", Addr: "::1", Port: 631}, "tcp [::1]:631"},
	}
	for _, c := range cases {
		if got := c.in.String(); got != c.want {
			t.Errorf("String() = %q, want %q", got, c.want)
		}
	}
}

func hasListener(ls []Listener, proto string, port int) bool {
	for _, l := range ls {
		if l.Proto == proto && l.Port == port {
			return true
		}
	}
	return false
}
