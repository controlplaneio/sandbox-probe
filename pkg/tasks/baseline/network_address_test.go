package tasks

import "testing"

func TestNetworkAddress(t *testing.T) {
	tests := []struct {
		name string
		host string
		port int
		want string
	}{
		{name: "hostname", host: "localhost", port: 443, want: "localhost:443"},
		{name: "IPv4", host: "127.0.0.1", port: 53, want: "127.0.0.1:53"},
		{name: "IPv6", host: "::1", port: 443, want: "[::1]:443"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := networkAddress(tt.host, tt.port); got != tt.want {
				t.Fatalf("networkAddress(%q, %d) = %q, want %q", tt.host, tt.port, got, tt.want)
			}
		})
	}
}
