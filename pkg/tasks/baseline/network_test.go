package tasks

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"

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

func Test_scanTCP(t *testing.T) {
	// Test TCP port scanning on localhost
	// Note: This scans all ports 1-65535 and may take some time
	// In most environments, some ports will be open on localhost

	t.Log("Starting TCP port scan on localhost (this may take a moment)...")
	openPorts := ScanTCP("127.0.0.1")

	// Should return a slice (even if empty)
	assert.NotNil(t, openPorts, "scanTCP should return a non-nil slice")

	// Log results
	t.Logf("Found %d open TCP ports on localhost", len(openPorts))

	if len(openPorts) > 0 {
		// Log first few open ports for debugging
		maxLog := 10
		if len(openPorts) < maxLog {
			maxLog = len(openPorts)
		}

		// Verify all ports are in valid range
		for _, port := range openPorts {
			assert.GreaterOrEqual(t, port, 1, "Port should be >= 1")
			assert.LessOrEqual(t, port, 65535, "Port should be <= 65535")
		}
	} else {
		t.Log("No open TCP ports found (this may indicate network isolation)")
	}
}

func Test_scanUDP(t *testing.T) {
	// Test UDP port scanning on localhost
	// Note: UDP scanning is inherently unreliable as it depends on responses
	// Many UDP services don't respond to empty packets, so results may vary

	t.Log("Starting UDP port scan on localhost (this may take a moment)...")
	openPorts := ScanUDP("127.0.0.1")

	// Should return a slice (even if empty)
	assert.NotNil(t, openPorts, "scanUDP should return a non-nil slice")

	// Log results
	t.Logf("Found %d open/filtered UDP ports on localhost", len(openPorts))

	if len(openPorts) > 0 {
		// Log first few open ports for debugging
		maxLog := 10
		if len(openPorts) < maxLog {
			maxLog = len(openPorts)
		}

		// Verify all ports are in valid range
		for _, port := range openPorts {
			assert.GreaterOrEqual(t, port, 1, "Port should be >= 1")
			assert.LessOrEqual(t, port, 65535, "Port should be <= 65535")
		}
	} else {
		t.Log("No open/filtered UDP ports found (this is common as UDP scanning is unreliable)")
	}
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
