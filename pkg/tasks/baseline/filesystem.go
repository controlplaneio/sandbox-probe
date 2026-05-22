package tasks

import (
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// SensitivePath describes a path to check for readability, with an optional
// content predicate. If contains is non-empty the file is only reported when
// its content includes that substring (case-insensitive).
type SensitivePath struct {
	path     string
	contains string // if set, file must contain this string to be reported
}

func sp(path string) SensitivePath                  { return SensitivePath{path: path} }
func spContains(path, substr string) SensitivePath  { return SensitivePath{path: path, contains: substr} }

// sensitivePaths is populated at runtime (requires home dir expansion).
// See buildSensitivePaths().
var sensitivePaths []SensitivePath

// buildSensitivePaths returns the full list of paths to check, expanding
// user-home entries against the real home directory.
func buildSensitivePaths() []SensitivePath {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/root"
	}
	return buildSensitivePathsForHome(home)
}

// buildSensitivePathsForHome is the testable core: it builds the path list
// using the provided home directory instead of calling os.UserHomeDir().
func buildSensitivePathsForHome(home string) []SensitivePath {
	h := func(p string) string { return home + p }

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

		// ── SSH keys (current user) ───────────────────────────────────────
		sp(h("/.ssh/id_rsa")),
		sp(h("/.ssh/id_ed25519")),
		sp(h("/.ssh/id_ecdsa")),
		sp(h("/.ssh/config")),
		sp(h("/.ssh/authorized_keys")),

		// ── Cloud credentials ─────────────────────────────────────────────
		sp(h("/.aws/credentials")),
		sp(h("/.aws/config")),
		sp(h("/.gcloud/credentials.db")),
		sp(h("/.gcloud/access_tokens.db")),
		sp(h("/.config/gcloud")),
		sp(h("/.azure/credentials")),
		sp(h("/.azure/msal_token_cache.json")),

		// ── Container / Kubernetes credentials ───────────────────────────
		sp(h("/.kube/config")),
		sp(h("/.docker/config.json")),

		// ── Crypto / signing ──────────────────────────────────────────────
		sp(h("/.gnupg")),

		// ── VCS credentials ───────────────────────────────────────────────
		sp(h("/.git-credentials")),
		sp(h("/.netrc")),
		// Only flag .gitconfig when it contains a [credential] section
		spContains(h("/.gitconfig"), "[credential]"),

		// ── Infrastructure / secrets management ───────────────────────────
		sp(h("/.vault-token")),
		sp(h("/.terraform.d/credentials.tfrc.json")),
		sp(h("/.config/gh/hosts.yml")),
		sp(h("/.config/op")),
		sp(h("/.config/doctl/config.yaml")),
		sp(h("/.fly/config.yml")),
		sp(h("/.cloudflared")),

		// ── Package manager tokens ────────────────────────────────────────
		sp(h("/.npmrc")),
		sp(h("/.pypirc")),
		sp(h("/.gem/credentials")),
		sp(h("/.cargo/credentials.toml")),
		sp(h("/.m2/settings.xml")),
		sp(h("/.gradle/gradle.properties")),
	}
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
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/root"
	}
	return scanTargetedPathsForHome(home)
}

// scanTargetedPathsForHome is the testable core: it runs the full scan using
// the provided home directory instead of calling os.UserHomeDir().
func scanTargetedPathsForHome(home string) *PathPermissions {
	result := &PathPermissions{
		WritablePaths: make([]string, 0),
		ReadablePaths: make([]string, 0),
	}

	// Check sensitive paths for read access
	for _, sp := range buildSensitivePathsForHome(home) {
		if _, err := os.Stat(sp.path); err != nil {
			continue
		}
		if !isReadable(sp.path) {
			continue
		}
		if sp.contains != "" {
			data, err := os.ReadFile(sp.path)
			if err != nil || !strings.Contains(strings.ToLower(string(data)), strings.ToLower(sp.contains)) {
				continue
			}
		}
		result.ReadablePaths = append(result.ReadablePaths, sp.path)
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
// First we check POSIX access, if no access is reported, we return false.
// For regular files we verify by opening O_WRONLY; directories cannot be
// opened with O_WRONLY on Linux so we trust the Access result directly.
func isWritable(path string) bool {
	if err := unix.Access(path, unix.W_OK); err != nil {
		return false
	}
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
