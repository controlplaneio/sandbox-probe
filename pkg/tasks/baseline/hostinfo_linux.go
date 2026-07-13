package tasks

import (
	"os"
	"strings"
)

// hostKernelInfo reads the kernel release and full version string from procfs.
func hostKernelInfo() (release, version string) {
	return readTrimFile("/proc/sys/kernel/osrelease"), readTrimFile("/proc/version")
}

// hostOSRelease returns PRETTY_NAME from /etc/os-release, e.g. "Ubuntu 24.04.3 LTS".
func hostOSRelease() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if v, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
			return strings.Trim(v, `"`)
		}
	}
	return ""
}

func readTrimFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
