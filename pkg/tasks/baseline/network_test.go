package tasks

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDnsQuery(t *testing.T) {
	tests := []struct {
		name        string
		hostname    string
		expectError bool
		expectIPs   bool
	}{
		{
			name:        "valid hostname localhost",
			hostname:    "localhost",
			expectError: false,
			expectIPs:   true,
		},
		{
			name:        "valid hostname google.com",
			hostname:    "google.com",
			expectError: false,
			expectIPs:   true,
		},
		{
			name:        "invalid hostname non-existent domain",
			hostname:    "this-domain-definitely-does-not-exist-12345.com",
			expectError: true,
			expectIPs:   false,
		},
		{
			name:        "empty hostname",
			hostname:    "",
			expectError: true,
			expectIPs:   false,
		},
		{
			name:        "valid hostname dns.google",
			hostname:    "dns.google",
			expectError: false,
			expectIPs:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ips, err := DnsQuery(tt.hostname)

			if tt.expectError {
				assert.Error(t, err, "dnsQuery(%s) expected error but got none", tt.hostname)
				return
			}

			assert.Greater(t, len(ips), 0, "dnsQuery(%s) returned an empty array of ips", tt.hostname)
		})
	}
}

func TestGetProxy(t *testing.T) {
	// getProxy should always return a result without error
	// It may return empty proxy settings if none are configured
	cfg, err := GetProxy()

	// Should not return an error
	assert.NoError(t, err, "getProxy() should not return an error")

	// Should return a non-nil config
	assert.NotNil(t, cfg, "getProxy() should return a non-nil tasks.models.ProxyConfig")

	// Log the proxy configuration for debugging
	t.Logf("Proxy configuration:")
	t.Logf("  HTTP_PROXY:  %s", cfg.HTTPProxy)
	t.Logf("  HTTPS_PROXY: %s", cfg.HTTPSProxy)
	t.Logf("  ALL_PROXY:   %s", cfg.ALLProxy)
	t.Logf("  NO_PROXY:    %s", cfg.NoProxy)
	t.Logf("  SOCKS_PROXY: %s", cfg.SOCKSProxy)
	t.Logf("  PAC_URL:     %s", cfg.PACURL)
}

func Test_getSockets(t *testing.T) {
	// Test getSockets function

	t.Log("Starting get sockets scan in /var/run/")

	sockets, err := GetSockets("/var/run/", false)
	require.NoError(t, err)
	for _, socket := range sockets {
		t.Logf("found socket: %s", socket)
	}
}

func Test_ScanSocketRoots_dedupAndNesting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix domain sockets and the /tmp layout are POSIX-only")
	}
	// Short /tmp base: macOS caps Unix socket bind paths at ~104 chars, which t.TempDir() blows.
	dir, err := os.MkdirTemp("/tmp", "spr")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	sub := filepath.Join(dir, "s")
	require.NoError(t, os.Mkdir(sub, 0o755))

	// A real bound Unix socket in the nested subdir.
	sockPath := filepath.Join(sub, "x.sock")
	l, err := net.Listen("unix", sockPath)
	require.NoError(t, err)
	defer l.Close()

	// A symlink aliasing the parent dir.
	alias := filepath.Join(dir, "alias")
	require.NoError(t, os.Symlink(dir, alias))

	// Pass overlapping roots: the parent, its nested subdir, the alias, and a non-existent path.
	// Nesting/symlink resolution must yield the socket exactly once, and the missing root is skipped.
	got, err := ScanSocketRoots([]string{dir, sub, alias, "/no/such/path/xyzzy"}, false)
	require.NoError(t, err)

	count := 0
	realSock, _ := filepath.EvalSymlinks(sockPath)
	for _, s := range got {
		if s == sockPath || s == realSock {
			count++
		}
	}
	assert.Equal(t, 1, count, "socket should be reported exactly once despite overlapping/aliased roots; got %v", got)
}

// TCPReachable must answer from real connects, not from name resolution: a
// listener on loopback is reachable, the same port once closed is not. This
// exercises the production dialer, not a fake.
func TestTCPReachableRealSocket(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	dial := (&net.Dialer{}).DialContext
	ips := []net.IP{net.ParseIP("127.0.0.1")}

	assert.True(t, TCPReachable(context.Background(), dial, ips, []int{port}, time.Second), "open listener on %d should be reachable", port)

	require.NoError(t, l.Close())
	assert.False(t, TCPReachable(context.Background(), dial, ips, []int{port}, time.Second), "closed port %d should not be reachable", port)
}

// Every attempt is bounded by the per-dial timeout, so a blackholed network
// (a dialer that never answers) costs at most attempts x timeout, never a hang.
func TestTCPReachableHonoursTimeout(t *testing.T) {
	var calls int
	blackhole := func(ctx context.Context, _, _ string) (net.Conn, error) {
		calls++
		<-ctx.Done()
		return nil, ctx.Err()
	}
	ips := []net.IP{net.ParseIP("192.0.2.1"), net.ParseIP("2001:db8::1")}

	done := make(chan bool, 1)
	start := time.Now()
	go func() {
		done <- TCPReachable(context.Background(), blackhole, ips, []int{443, 80}, 20*time.Millisecond)
	}()
	select {
	case got := <-done:
		assert.False(t, got, "a dialer that never connects must not count as reachable")
		assert.Less(t, time.Since(start), time.Second, "four 20ms attempts should finish well inside a second")
		assert.Equal(t, 4, calls, "each ip/port pair should be tried once")
	case <-time.After(2 * time.Second):
		t.Fatal("TCPReachable did not honour the per-attempt timeout")
	}
}

// Addresses go through JoinHostPort, so IPv6 literals are bracketed, and the
// first success stops further attempts.
func TestTCPReachableAddressesAndShortCircuit(t *testing.T) {
	var addrs []string
	dial := func(_ context.Context, network, addr string) (net.Conn, error) {
		assert.Equal(t, "tcp", network)
		addrs = append(addrs, addr)
		if addr == "[2001:db8::1]:443" {
			c, s := net.Pipe()
			_ = s.Close()
			return c, nil
		}
		return nil, errors.New("connection refused")
	}
	ips := []net.IP{net.ParseIP("192.0.2.1"), net.ParseIP("2001:db8::1"), net.ParseIP("192.0.2.9")}

	assert.True(t, TCPReachable(context.Background(), dial, ips, []int{443, 80}, time.Second))
	assert.Equal(t, []string{"192.0.2.1:443", "192.0.2.1:80", "[2001:db8::1]:443"}, addrs)
}
