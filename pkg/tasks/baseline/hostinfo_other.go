//go:build !linux && !darwin

package tasks

// hostKernelInfo / hostOSRelease are best-effort no-ops where there's no uniform kernel/OS query
// (e.g. Windows); the report's probe_binary.os/arch still record the platform.
func hostKernelInfo() (release, version string) { return "", "" }

func hostOSRelease() string { return "" }
