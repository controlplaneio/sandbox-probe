package tasks

import (
	"testing"

	"github.com/stretchr/testify/assert"
)


func TestStringToContainerRuntime(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ContainerRuntime
	}{
		{
			name:     "docker keyword",
			input:    "1:name=systemd:/docker/abc123",
			expected: RuntimeDocker,
		},
		{
			name:     "podman keyword",
			input:    "1:name=systemd:/podman/xyz789",
			expected: RuntimePodman,
		},
		{
			name:     "lxc keyword",
			input:    "1:name=systemd:/lxc/container",
			expected: RuntimeLXC,
		},
		{
			name:     "firejail keyword",
			input:    "/usr/bin/firejail --profile=default",
			expected: RuntimeFirejail,
		},
		{
			name:     "no container",
			input:    "1:name=systemd:/user.slice/user-1000.slice",
			expected: RuntimeUnknown,
		},
		{
			name:     "empty string",
			input:    "",
			expected: RuntimeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringToContainerRuntime(tt.input)
			assert.Equal(t, tt.expected, result,
				"stringToContainerRuntime(%q) = %v, expected %v",
				tt.input, result, tt.expected)
		})
	}
}
