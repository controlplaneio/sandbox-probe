package tasks

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// SensitivePath describes a path to check for readability, with an optional
// content predicate. If contains is non-empty the file is only reported when
// its content includes that substring (case-insensitive).
type SensitivePath struct {
	path     string
	contains string // if set, file must contain this string to be reported
	isDir    bool   // path is a directory, not a regular file (never seeded)
}

func sp(path string) SensitivePath    { return SensitivePath{path: path} }
func spDir(path string) SensitivePath { return SensitivePath{path: path, isDir: true} }
func spContains(path, substr string) SensitivePath {
	return SensitivePath{path: path, contains: substr}
}

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
// buildSensitivePathsForHome returns the full list of credential paths to scan.
// It combines platform-specific absolute paths (from platformSensitivePaths,
// defined per-platform in filesystem_unix.go / filesystem_windows.go) with
// home-relative paths that are meaningful on every OS.
func buildSensitivePathsForHome(home string) []SensitivePath {
	h := func(p string) string { return filepath.Join(home, p) }

	// Cross-platform home-relative credential paths.
	// filepath.Join handles the separator on every OS; os.Stat silently skips
	// any path that doesn't exist on the current platform.
	homePaths := []SensitivePath{
		// ── SSH keys ──────────────────────────────────────────────────────
		sp(h(".ssh/id_rsa")),
		sp(h(".ssh/id_ed25519")),
		sp(h(".ssh/id_ecdsa")),
		sp(h(".ssh/config")),
		sp(h(".ssh/authorized_keys")),

		// ── Cloud credentials ─────────────────────────────────────────────
		sp(h(".aws/credentials")),
		sp(h(".aws/config")),
		sp(h(".gcloud/credentials.db")),
		sp(h(".gcloud/access_tokens.db")),
		spDir(h(".config/gcloud")),
		sp(h(".azure/credentials")),
		sp(h(".azure/msal_token_cache.json")),

		// ── Container / Kubernetes credentials ───────────────────────────
		sp(h(".kube/config")),
		sp(h(".docker/config.json")),

		// ── Crypto / signing ──────────────────────────────────────────────
		spDir(h(".gnupg")),

		// ── VCS credentials ───────────────────────────────────────────────
		sp(h(".git-credentials")),
		sp(h(".netrc")),
		// Only flag .gitconfig when it contains a [credential] section
		spContains(h(".gitconfig"), "[credential]"),

		// ── Infrastructure / secrets management ───────────────────────────
		sp(h(".vault-token")),
		sp(h(".terraform.d/credentials.tfrc.json")),
		sp(h(".config/gh/hosts.yml")),
		spDir(h(".config/op")),
		sp(h(".config/doctl/config.yaml")),
		sp(h(".fly/config.yml")),
		spDir(h(".cloudflared")),

		// ── Package manager tokens ────────────────────────────────────────
		sp(h(".npmrc")),
		sp(h(".pypirc")),
		sp(h(".gem/credentials")),
		sp(h(".cargo/credentials.toml")),
		sp(h(".m2/settings.xml")),
		sp(h(".gradle/gradle.properties")),
	}

	return append(platformSensitivePaths(home), homePaths...)
}

// Fixed vocabularies published by ADR 0002 and CONTEXT.md. An entry outside
// them is a bug the registry's own tests reject, so the seeder can dispatch on
// kind and a reviewer can read an entry's provenance without asking the author.
var (
	// targetKinds is *how* an entry is seeded.
	targetKinds = map[string]bool{
		"file": true, "dir": true, "socket": true, "pipe": true, "process": true,
	}
	// targetCategories is the real-world tool class an IPC entry stands in for —
	// *why* it is on the list.
	targetCategories = map[string]bool{
		"container-runtime": true, "credential-agent": true, "editor-ipc": true,
		"agent-ipc": true, "chat-client": true, "browser": true,
		"password-manager": true, "desktop-bus": true,
	}
	// evidenceTiers is how strong the evidence for an entry is.
	evidenceTiers = map[string]bool{
		"empirical-own-machine": true, "empirical-contributed": true,
		"documented-not-verified": true, "reasoned-by-analogy": true,
	}
)

// Target is one probe check target exposed by `list-targets`, carrying the
// classification a seeder needs to plant decoys safely.
type Target struct {
	Path     string `json:"path"`
	Kind     string `json:"kind"`     // targetKinds: file | dir | socket | pipe | process
	Scope    string `json:"scope"`    // "home" | "system"
	Seedable bool   `json:"seedable"` // safe to soft-plant a decoy: home-scoped regular files only
	// Provenance, carried by IPC entries (socket / pipe / process) only — the
	// filesystem entries are the probe's own check list, not a tool catalogue.
	Category string `json:"category,omitempty"` // targetCategories
	Evidence string `json:"evidence,omitempty"` // evidenceTiers
	// OS applicability as GOOS values. Empty means every OS, which is every
	// filesystem entry — their applicability is unchanged by this field.
	OS []string `json:"os,omitempty"`
}

// appliesTo reports whether the target exists on the given GOOS.
func (t Target) appliesTo(goos string) bool {
	return len(t.OS) == 0 || slices.Contains(t.OS, goos)
}

// ListTargets returns the probe's sensitive-path registry as Targets, filtered
// to the running OS. It is the single source of truth for the seeder — a decoy
// is only ever planted where a target is Seedable, so the seeder cannot drift
// from what is actually probed, and never attempts a target belonging to
// another operating system.
func ListTargets() []Target {
	home := homeOrRoot()
	return targetsForOS(listTargetsForHome(home, siblingSessionSocket(claudeDaemonDir())), runtime.GOOS)
}

// targetsForOS is the testable core of the OS filter.
func targetsForOS(all []Target, goos string) []Target {
	out := make([]Target, 0, len(all))
	for _, t := range all {
		if t.appliesTo(goos) {
			out = append(out, t)
		}
	}
	return out
}

func homeOrRoot() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "/root"
	}
	return home
}

// listTargetsForHome is the testable core: the probe's own sensitive-path
// check list, plus the IPC socket and process catalogues (see socketTargets for
// what siblingSock carries).
func listTargetsForHome(home, siblingSock string) []Target {
	out := make([]Target, 0, len(buildSensitivePathsForHome(home)))
	for _, s := range buildSensitivePathsForHome(home) {
		kind := "file"
		if s.isDir {
			kind = "dir"
		}
		scope := "system"
		// Accept either separator: buildSensitivePathsForHome joins with
		// filepath.Join, so a Windows home yields backslashes, while the
		// registry's own tests drive this with POSIX-style homes.
		if strings.HasPrefix(s.path, home+string(os.PathSeparator)) || strings.HasPrefix(s.path, home+"/") {
			scope = "home"
		}
		out = append(out, Target{
			Path:  s.path,
			Kind:  kind,
			Scope: scope,
			// Home-scoped regular files only — and never a content-predicate target
			// (contains != ""). Those are reported only when they hold specific
			// structured content, so a generic decoy would never satisfy the predicate
			// anyway, and worse, seeding one corrupts a file real tooling parses
			// (e.g. ~/.gitconfig -> "fatal: bad config line 1", breaking git clone).
			Seedable: scope == "home" && kind == "file" && s.contains == "",
		})
	}
	out = append(out, socketTargets(home, siblingSock)...)
	out = append(out, pipeTargets()...)
	return append(out, processTargets()...)
}

// SystemWritePaths is the platform-specific list of system directories that
// should be read-only. Defined per-platform in filesystem_unix.go /
// filesystem_windows.go via the platformSystemWritePaths variable, then
// exposed here for use by scanTargetedPathsForHome and tests.
var SystemWritePaths = platformSystemWritePaths

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
		home = platformDefaultHome()
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
