package tasks

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTestFile creates parent dirs and writes content to path.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

// makeUnreadable chmod 000s a file and restores permissions on cleanup.
func makeUnreadable(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.Chmod(path, 0o000))
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
}

// inReadable reports whether path appears in result.ReadablePaths.
func inReadable(result *PathPermissions, path string) bool {
	return slices.Contains(result.ReadablePaths, path)
}

// ── SensitivePath constructors ─────────────────────────────────────────────

func TestSp(t *testing.T) {
	p := sp("/etc/passwd")
	assert.Equal(t, "/etc/passwd", p.path)
	assert.Empty(t, p.contains, "sp() should not set a content predicate")
}

func TestSpContains(t *testing.T) {
	p := spContains("/home/user/.gitconfig", "[credential]")
	assert.Equal(t, "/home/user/.gitconfig", p.path)
	assert.Equal(t, "[credential]", p.contains)
}

// ── buildSensitivePathsForHome ─────────────────────────────────────────────

func TestBuildSensitivePathsForHome_expandsHome(t *testing.T) {
	home := "/fake/home/testuser"
	paths := buildSensitivePathsForHome(home)

	got := make([]string, len(paths))
	for i, p := range paths {
		got[i] = p.path
	}

	homeRelative := []string{
		filepath.Join(home, ".ssh", "id_rsa"),
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "id_ecdsa"),
		filepath.Join(home, ".ssh", "config"),
		filepath.Join(home, ".ssh", "authorized_keys"),
		filepath.Join(home, ".aws", "credentials"),
		filepath.Join(home, ".aws", "config"),
		filepath.Join(home, ".gcloud", "credentials.db"),
		filepath.Join(home, ".gcloud", "access_tokens.db"),
		filepath.Join(home, ".config", "gcloud"),
		filepath.Join(home, ".azure", "credentials"),
		filepath.Join(home, ".azure", "msal_token_cache.json"),
		filepath.Join(home, ".kube", "config"),
		filepath.Join(home, ".docker", "config.json"),
		filepath.Join(home, ".gnupg"),
		filepath.Join(home, ".git-credentials"),
		filepath.Join(home, ".netrc"),
		filepath.Join(home, ".gitconfig"),
		filepath.Join(home, ".vault-token"),
		filepath.Join(home, ".terraform.d", "credentials.tfrc.json"),
		filepath.Join(home, ".config", "gh", "hosts.yml"),
		filepath.Join(home, ".config", "op"),
		filepath.Join(home, ".config", "doctl", "config.yaml"),
		filepath.Join(home, ".fly", "config.yml"),
		filepath.Join(home, ".cloudflared"),
		filepath.Join(home, ".npmrc"),
		filepath.Join(home, ".pypirc"),
		filepath.Join(home, ".gem", "credentials"),
		filepath.Join(home, ".cargo", "credentials.toml"),
		filepath.Join(home, ".m2", "settings.xml"),
		filepath.Join(home, ".gradle", "gradle.properties"),
	}

	for _, want := range homeRelative {
		assert.Contains(t, got, want, "path %q should be in sensitive paths list", want)
	}
}

func TestBuildSensitivePathsForHome_includesSystemPaths(t *testing.T) {
	paths := buildSensitivePathsForHome("/any/home")
	got := make([]string, len(paths))
	for i, p := range paths {
		got[i] = p.path
	}

	system := []string{
		"/etc/passwd", "/etc/shadow", "/etc/group", "/etc/gshadow",
		"/etc/hostname", "/etc/hosts", "/etc/resolv.conf",
		"/etc/ssh/sshd_config", "/etc/sudoers",
		"/var/run/docker.sock",
		"/run/secrets",
		"/var/run/secrets/kubernetes.io/serviceaccount/token",
		"/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
	}
	for _, want := range system {
		assert.Contains(t, got, want, "system path %q should be in list", want)
	}
}

func TestBuildSensitivePathsForHome_gitconfigHasContentPredicate(t *testing.T) {
	home := "/tmp/fakehome"
	paths := buildSensitivePathsForHome(home)

	for _, p := range paths {
		if p.path == filepath.Join(home, ".gitconfig") {
			assert.Equal(t, "[credential]", p.contains,
				".gitconfig must carry a [credential] content predicate")
			return
		}
	}
	t.Fatal(".gitconfig not found in sensitive paths list")
}

func TestBuildSensitivePathsForHome_noDuplicates(t *testing.T) {
	paths := buildSensitivePathsForHome("/home/u")
	seen := make(map[string]bool)
	for _, p := range paths {
		assert.False(t, seen[p.path], "duplicate path: %q", p.path)
		seen[p.path] = true
	}
}

func TestBuildSensitivePathsForHome_noEmptyPaths(t *testing.T) {
	paths := buildSensitivePathsForHome("/home/u")
	for _, p := range paths {
		assert.NotEmpty(t, p.path)
	}
}

// ── scanTargetedPathsForHome — presence & readability ─────────────────────

func TestScanTargetedPathsForHome_neverReturnsNil(t *testing.T) {
	result := scanTargetedPathsForHome(t.TempDir())
	require.NotNil(t, result)
	assert.NotNil(t, result.ReadablePaths)
	assert.NotNil(t, result.WritablePaths)
}

func TestScanTargetedPathsForHome_missingFilesNotReported(t *testing.T) {
	home := t.TempDir() // empty — no credential files created
	result := scanTargetedPathsForHome(home)

	for _, p := range result.ReadablePaths {
		assert.NotContains(t, p, home,
			"no home-relative path should appear when home dir is empty")
	}
}

func TestScanTargetedPathsForHome_readableFileReported(t *testing.T) {
	home := t.TempDir()
	target := filepath.Join(home, ".aws", "credentials")
	writeTestFile(t, target, "[default]\nplaceholder = value\n")

	result := scanTargetedPathsForHome(home)

	assert.True(t, inReadable(result, target), ".aws/credentials should be reported")
}

func TestScanTargetedPathsForHome_unreadableFileNotReported(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 000 has no effect on Windows ACL-based permissions")
	}
	if os.Getuid() == 0 {
		t.Skip("chmod 000 has no effect as root")
	}
	home := t.TempDir()
	target := filepath.Join(home, ".aws", "credentials")
	writeTestFile(t, target, "[default]\nplaceholder = value\n")
	makeUnreadable(t, target)

	result := scanTargetedPathsForHome(home)
	assert.False(t, inReadable(result, target), "unreadable file must not be reported")
}

func TestScanTargetedPathsForHome_onlyReportsExistingPaths(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".npmrc"), "placeholder\n")

	result := scanTargetedPathsForHome(home)
	for _, p := range result.ReadablePaths {
		_, err := os.Stat(p)
		assert.NoError(t, err, "reported path %q must exist on disk", p)
	}
}

// ── Content predicate (.gitconfig) ────────────────────────────────────────

func TestScanTargetedPathsForHome_gitconfigWithCredentialSection(t *testing.T) {
	home := t.TempDir()
	gitconfig := filepath.Join(home, ".gitconfig")
	writeTestFile(t, gitconfig, "[user]\n\tname = Test\n\n[credential]\n\thelper = osxkeychain\n")

	result := scanTargetedPathsForHome(home)

	assert.True(t, inReadable(result, gitconfig),
		".gitconfig with [credential] should be reported")
}

func TestScanTargetedPathsForHome_gitconfigWithoutCredentialSection(t *testing.T) {
	home := t.TempDir()
	gitconfig := filepath.Join(home, ".gitconfig")
	writeTestFile(t, gitconfig, "[user]\n\tname = Test\n\temail = test@example.com\n")

	result := scanTargetedPathsForHome(home)
	assert.False(t, inReadable(result, gitconfig),
		".gitconfig without [credential] must not be reported")
}

func TestScanTargetedPathsForHome_gitconfigContentCheckCaseInsensitive(t *testing.T) {
	home := t.TempDir()
	gitconfig := filepath.Join(home, ".gitconfig")
	writeTestFile(t, gitconfig, "[CREDENTIAL]\n\thelper = store\n")

	result := scanTargetedPathsForHome(home)

	assert.True(t, inReadable(result, gitconfig),
		"content predicate must be case-insensitive")
}

func TestScanTargetedPathsForHome_gitconfigEmpty(t *testing.T) {
	home := t.TempDir()
	gitconfig := filepath.Join(home, ".gitconfig")
	writeTestFile(t, gitconfig, "")

	result := scanTargetedPathsForHome(home)
	assert.False(t, inReadable(result, gitconfig), "empty .gitconfig must not be reported")
}

// ── Per-category credential detection ─────────────────────────────────────

func TestScanTargetedPathsForHome_sshKeys(t *testing.T) {
	home := t.TempDir()
	files := []string{
		".ssh/id_rsa",
		".ssh/id_ed25519",
		".ssh/id_ecdsa",
		".ssh/config",
		".ssh/authorized_keys",
	}
	for _, rel := range files {
		writeTestFile(t, filepath.Join(home, rel), "placeholder\n")
	}

	result := scanTargetedPathsForHome(home)

	for _, rel := range files {
		assert.True(t, inReadable(result, filepath.Join(home, rel)), "%s should be reported", rel)
	}
}

func TestScanTargetedPathsForHome_cloudCredentials(t *testing.T) {
	home := t.TempDir()
	files := []string{
		".aws/credentials",
		".aws/config",
		".azure/credentials",
		".azure/msal_token_cache.json",
		".gcloud/credentials.db",
		".gcloud/access_tokens.db",
	}
	for _, rel := range files {
		writeTestFile(t, filepath.Join(home, rel), "placeholder\n")
	}

	result := scanTargetedPathsForHome(home)

	for _, rel := range files {
		assert.True(t, inReadable(result, filepath.Join(home, rel)), "%s should be reported", rel)
	}
}

func TestScanTargetedPathsForHome_containerCredentials(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".kube", "config"), "apiVersion: v1\n")
	writeTestFile(t, filepath.Join(home, ".docker", "config.json"), "{}\n")

	result := scanTargetedPathsForHome(home)

	assert.True(t, inReadable(result, filepath.Join(home, ".kube", "config")))
	assert.True(t, inReadable(result, filepath.Join(home, ".docker", "config.json")))
}

func TestScanTargetedPathsForHome_infraSecrets(t *testing.T) {
	home := t.TempDir()
	files := []string{
		".vault-token",
		".terraform.d/credentials.tfrc.json",
		".config/gh/hosts.yml",
		".config/doctl/config.yaml",
		".fly/config.yml",
	}
	for _, rel := range files {
		writeTestFile(t, filepath.Join(home, rel), "placeholder\n")
	}

	result := scanTargetedPathsForHome(home)

	for _, rel := range files {
		assert.True(t, inReadable(result, filepath.Join(home, rel)), "%s should be reported", rel)
	}
}

func TestScanTargetedPathsForHome_packageManagerTokens(t *testing.T) {
	home := t.TempDir()
	files := []string{
		".npmrc",
		".pypirc",
		".gem/credentials",
		".cargo/credentials.toml",
		".m2/settings.xml",
		".gradle/gradle.properties",
	}
	for _, rel := range files {
		writeTestFile(t, filepath.Join(home, rel), "placeholder\n")
	}

	result := scanTargetedPathsForHome(home)

	for _, rel := range files {
		assert.True(t, inReadable(result, filepath.Join(home, rel)), "%s should be reported", rel)
	}
}

func TestScanTargetedPathsForHome_vcsCredentials(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".git-credentials"), "placeholder\n")
	writeTestFile(t, filepath.Join(home, ".netrc"), "placeholder\n")

	result := scanTargetedPathsForHome(home)

	assert.True(t, inReadable(result, filepath.Join(home, ".git-credentials")))
	assert.True(t, inReadable(result, filepath.Join(home, ".netrc")))
}

func TestScanTargetedPathsForHome_directoryPaths(t *testing.T) {
	home := t.TempDir()
	dirs := []string{".gnupg", ".config/op", ".cloudflared"}
	for _, d := range dirs {
		require.NoError(t, os.MkdirAll(filepath.Join(home, d), 0o700))
	}

	result := scanTargetedPathsForHome(home)

	for _, d := range dirs {
		assert.True(t, inReadable(result, filepath.Join(home, d)),
			"directory %s should be reported", d)
	}
}
