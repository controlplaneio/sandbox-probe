//go:build !windows
// +build !windows

package tasks

import (
	"os"

	"golang.org/x/sys/unix"
)

func isReadable(path string) bool {
	if err := unix.Access(path, unix.R_OK); err != nil {
		return false
	}
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func isWritable(path string) bool {
	if err := unix.Access(path, unix.W_OK); err != nil {
		return false
	}
	// Directories cannot be opened with O_WRONLY; unix.Access is sufficient.
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return true
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// platformDefaultHome returns the fallback home directory when os.UserHomeDir fails.
func platformDefaultHome() string {
	return "/root"
}

// platformSensitivePaths returns Unix-specific absolute paths to check.
// These paths don't exist on Windows and are meaningless there.
func platformSensitivePaths(_ string) []SensitivePath {
	return []SensitivePath{
		// ── User and group information ────────────────────────────────────
		sp("/etc/passwd"),
		sp("/etc/shadow"),
		sp("/etc/group"),
		sp("/etc/gshadow"),

		// ── System configuration ──────────────────────────────────────────
		sp("/etc/hostname"),
		sp("/etc/hosts"),
		sp("/etc/resolv.conf"),
		sp("/etc/ssh/sshd_config"),
		sp("/etc/sudoers"),

		// ── Process and container information ─────────────────────────────
		sp("/proc/self/cgroup"),
		sp("/proc/self/mountinfo"),
		sp("/proc/self/status"),
		sp("/proc/1/cgroup"),
		sp("/proc/1/environ"),

		// ── Container runtime indicators ──────────────────────────────────
		sp("/.dockerenv"),
		sp("/run/.containerenv"),
		sp("/var/run/docker.sock"),

		// ── Root account credentials ──────────────────────────────────────
		sp("/root"),
		sp("/root/.ssh"),
		sp("/root/.bash_history"),

		// ── System credentials and keys ───────────────────────────────────
		sp("/etc/ssl/private"),
		sp("/etc/pki/private"),
		sp("/var/lib/docker"),

		// ── Runtime secrets (Docker / Kubernetes) ─────────────────────────
		sp("/run/secrets"),
		sp("/var/run/secrets/kubernetes.io/serviceaccount/token"),
		sp("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"),
	}
}

// platformSystemWritePaths is the Unix list of system directories that
// should be read-only under normal operation.
var platformSystemWritePaths = []string{
	// Core system directories
	"/etc",
	"/boot",
	"/usr",
	"/usr/bin",
	"/usr/sbin",
	"/usr/lib",
	"/bin",
	"/sbin",
	"/lib",
	"/lib64",

	// System state
	"/sys",
	"/proc",
	"/dev",

	// Variable data (some writable, some not)
	"/var",
	"/var/log",
	"/var/lib",
	"/opt",
}
