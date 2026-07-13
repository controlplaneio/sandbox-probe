package tasks

import "golang.org/x/sys/unix"

// hostKernelInfo reads the Darwin kernel release and version via sysctl.
func hostKernelInfo() (release, version string) {
	release, _ = unix.Sysctl("kern.osrelease")
	version, _ = unix.Sysctl("kern.version")
	return release, version
}

// hostOSRelease returns the marketing OS version, e.g. "macOS 15.5".
func hostOSRelease() string {
	v, err := unix.Sysctl("kern.osproductversion")
	if err != nil || v == "" {
		return ""
	}
	return "macOS " + v
}
