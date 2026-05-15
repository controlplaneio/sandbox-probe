package tasks

import (
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// Sensitive paths to check for readability (information disclosure)
var SensitiveReadPaths = []string{
	// User and group information
	"/etc/passwd",
	"/etc/shadow",
	"/etc/group",
	"/etc/gshadow",

	// System configuration
	"/etc/hostname",
	"/etc/hosts",
	"/etc/resolv.conf",
	"/etc/ssh/sshd_config",
	"/etc/sudoers",

	// Process and container information
	"/proc/self/cgroup",
	"/proc/self/mountinfo",
	"/proc/self/status",
	"/proc/1/cgroup",
	"/proc/1/environ",

	// Container runtime indicators
	"/.dockerenv",
	"/run/.containerenv",
	"/var/run/docker.sock",

	// Root directory
	"/root",
	"/root/.ssh",
	"/root/.bash_history",

	// System credentials and keys
	"/etc/ssl/private",
	"/etc/pki/private",
	"/var/lib/docker",
}

// System directories to check for write permissions (should typically be read-only)
var SystemWritePaths = []string{
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

// PathPermissions holds lists of writable and readable paths
type PathPermissions struct {
	WritablePaths []string
	ReadablePaths []string
}

// ScanTargetedPaths performs targeted security enumeration by checking
// specific sensitive paths instead of walking the entire filesystem.
// Returns separate lists for readable sensitive paths and writable system paths.
func ScanTargetedPaths() *PathPermissions {
	result := &PathPermissions{
		WritablePaths: make([]string, 0),
		ReadablePaths: make([]string, 0),
	}

	// Check sensitive paths for read access
	for _, path := range SensitiveReadPaths {
		if _, err := os.Stat(path); err == nil {
			if isReadable(path) {
				result.ReadablePaths = append(result.ReadablePaths, path)
			}
		}
	}

	// Check system paths for write access
	for _, path := range SystemWritePaths {
		if _, err := os.Stat(path); err == nil {
			if isWritable(path) {
				result.WritablePaths = append(result.WritablePaths, path)
			}
		}
	}

	return result
}

// // ScanFilesystemPermissions scans the filesystem starting from rootPath
// // up to the specified maxDepth and returns lists of writable and readable paths.
// // maxDepth of 0 means only check the root path, 1 means root + immediate children, etc.
// // A negative maxDepth means unlimited depth.
// func ScanFilesystemPermissions(rootPath string, maxDepth int) (*PathPermissions, error) {
// 	result := &PathPermissions{
// 		WritablePaths: make([]string, 0),
// 		ReadablePaths: make([]string, 0),
// 	}

// 	// Clean the root path
// 	rootPath = filepath.Clean(rootPath)
// 	rootDepth := strings.Count(rootPath, string(os.PathSeparator))

// 	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
// 		// If there was an error accessing this path, skip it
// 		if err != nil {
// 			return nil // Continue walking other paths
// 		}

// 		// Skip pseudo-filesystems for efficiency
// 		if isPseudoFilesystem(path) {
// 			if d.IsDir() {
// 				return fs.SkipDir
// 			}
// 			return nil
// 		}

// 		// Calculate current depth relative to root
// 		currentDepth := strings.Count(path, string(os.PathSeparator)) - rootDepth

// 		// If we've exceeded max depth, skip this directory
// 		if maxDepth >= 0 && currentDepth > maxDepth {
// 			if d.IsDir() {
// 				return fs.SkipDir
// 			}
// 			return nil
// 		}

// 		// Check if path is readable
// 		if isReadable(path) {
// 			result.ReadablePaths = append(result.ReadablePaths, path)
// 		}

// 		// Check if path is writable
// 		if isWritable(path) {
// 			result.WritablePaths = append(result.WritablePaths, path)
// 		}

// 		return nil
// 	})

// 	// Ignore permission denied errors at the top level
// 	if err != nil && !os.IsPermission(err) {
// 		return result, err
// 	}

// 	return result, nil
// }

// isReadable checks if the current process can read the given path.
// First we check POSIX access, if no access is reported, we return false
// If access through POSIX is reported, we verify it trying opening the file
func isReadable(path string) bool {
	err := unix.Access(path, unix.R_OK)
	if err != nil {
		return false
	}
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// isWritable checks if the current process can write to the given path.
// First we check POSIX access, if no access is reported, we return false
// If access through POSIX is reported, we verify it trying opening the file
func isWritable(path string) bool {
	err := unix.Access(path, unix.W_OK)
	if err != nil {
		return false
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// isPseudoFilesystem returns true if the path is in a pseudo-filesystem
// that should be skipped during security enumeration
func isPseudoFilesystem(path string) bool {
	pseudoFS := []string{"/proc", "/sys", "/dev"}
	for _, prefix := range pseudoFS {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
