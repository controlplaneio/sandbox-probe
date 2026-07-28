package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

func TestLoadConfig_empty_path(t *testing.T) {
	cfg, err := LoadConfig("")
	require.NoError(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfig_missing_file(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	assert.Error(t, err)
}

func TestLoadConfig_invalid_yaml(t *testing.T) {
	path := writeConfig(t, ":\tinvalid: yaml: {{{")
	_, err := LoadConfig(path)
	assert.Error(t, err)
}

func TestLoadConfig_rejectsUnknownFields(t *testing.T) {
	path := writeConfig(t, `
custom_paths:
  must_blocks:
    - path: /home/bob/.ssh
      label: ssh_keys
      severity: critical
`)

	_, err := LoadConfig(path)
	assert.Error(t, err)
}

func TestLoadConfig_requiresAtLeastOneCustomPath(t *testing.T) {
	path := writeConfig(t, `
identity:
  sandbox_user: alice
custom_paths: {}
`)

	_, err := LoadConfig(path)
	assert.Error(t, err)
}

func TestLoadConfig_rejectsMultipleDocuments(t *testing.T) {
	path := writeConfig(t, `
custom_paths:
  audit:
    - path: /tmp
      label: temporary
---
custom_paths:
  audit:
    - path: /var/tmp
      label: other
`)

	_, err := LoadConfig(path)
	assert.Error(t, err)
}

func TestLoadConfig_rejectsCheckOpsUnsupportedByCategory(t *testing.T) {
	path := writeConfig(t, `
custom_paths:
  must_read:
    - path: /tmp
      label: temporary
      severity: error
      check_ops: [write]
`)

	_, err := LoadConfig(path)
	assert.Error(t, err)
}

func TestLoadConfig_rejectsEscapingCheckFiles(t *testing.T) {
	path := writeConfig(t, `
custom_paths:
  must_block:
    - path: /tmp
      label: temporary
      severity: critical
      check_files: [../outside]
`)

	_, err := LoadConfig(path)
	assert.Error(t, err)
}

func TestLoadConfig_rejectsRelativePaths(t *testing.T) {
	path := writeConfig(t, `
custom_paths:
  must_block:
    - path: ../host/.ssh
      label: ssh
      severity: critical
`)

	_, err := LoadConfig(path)
	assert.Error(t, err)
}

func TestIsAbsolutePath(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{path: "/home/bob/.ssh", want: true},
		{path: `C:\Users\bob\.ssh`, want: true},
		{path: `D:/Users/bob/.ssh`, want: true},
		{path: `\\server\share\bob\.ssh`, want: true},
		{path: "relative/path", want: false},
		{path: `C:relative\path`, want: false},
	} {
		assert.Equal(t, tc.want, isAbsolutePath(tc.path), tc.path)
	}
}

func TestLoadConfig_requiresSeverityForExpectations(t *testing.T) {
	path := writeConfig(t, `
custom_paths:
  must_block:
    - path: /tmp
      label: temporary
`)

	_, err := LoadConfig(path)
	assert.Error(t, err)
}

func TestLoadConfig_minimal(t *testing.T) {
	path := writeConfig(t, `
identity:
  sandbox_user: alice
  sandbox_uid: 1001
  host_user: bob
  host_uid: 1000
custom_paths:
  must_block:
    - path: /home/bob/.ssh
      label: ssh_keys
      severity: critical
      reason: "SSH private keys"
`)
	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "alice", cfg.Identity.SandboxUser)
	assert.Equal(t, 1001, cfg.Identity.SandboxUID)
	assert.Equal(t, "bob", cfg.Identity.HostUser)

	require.Len(t, cfg.CustomPaths.MustBlock, 1)
	e := cfg.CustomPaths.MustBlock[0]
	assert.Equal(t, "/home/bob/.ssh", e.Path)
	assert.Equal(t, "ssh_keys", e.Label)
	assert.Equal(t, SeverityCritical, e.Severity)
}

func TestLoadConfig_all_categories(t *testing.T) {
	path := writeConfig(t, `
custom_paths:
  must_block:
    - path: /home/bob/.ssh
      label: ssh
      severity: critical
      reason: keys
  must_read:
    - path: /usr/bin
      label: usr_bin
      severity: error
      reason: binaries
  must_readwrite:
    - path: /tmp/workspace
      label: workspace
      severity: error
      reason: cwd
  audit:
    - path: /home/bob
      label: host_home
      note: "stat leaks existence"
`)
	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	assert.Len(t, cfg.CustomPaths.MustBlock, 1)
	assert.Len(t, cfg.CustomPaths.MustRead, 1)
	assert.Len(t, cfg.CustomPaths.MustReadWrite, 1)
	assert.Len(t, cfg.CustomPaths.Audit, 1)
	assert.Equal(t, "stat leaks existence", cfg.CustomPaths.Audit[0].Note)
}

func TestLoadConfig_check_ops_override(t *testing.T) {
	path := writeConfig(t, `
custom_paths:
  must_block:
    - path: /usr/local/src
      label: usr_local_src
      severity: warn
      reason: read ok, write must be denied
      check_ops: [readdir, open, write]
`)
	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	e := cfg.CustomPaths.MustBlock[0]
	assert.True(t, e.HasOp(OpReaddir))
	assert.True(t, e.HasOp(OpOpen))
	assert.True(t, e.HasOp(OpWrite))
	assert.False(t, e.HasOp(OpStat))
}

func TestLoadConfig_check_files(t *testing.T) {
	path := writeConfig(t, `
custom_paths:
  must_block:
    - path: /home/bob/.ssh
      label: ssh
      severity: critical
      reason: keys
      check_files:
        - id_rsa
        - id_ed25519
`)
	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"id_rsa", "id_ed25519"}, cfg.CustomPaths.MustBlock[0].CheckFiles)
}

func TestLoadConfig_unknown_severity(t *testing.T) {
	path := writeConfig(t, `
custom_paths:
  must_block:
    - path: /foo
      label: foo
      severity: catastrophic
      reason: bad
`)
	_, err := LoadConfig(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown severity")
}

func TestLoadConfig_unknown_check_op(t *testing.T) {
	path := writeConfig(t, `
custom_paths:
  must_block:
    - path: /foo
      label: foo
      severity: error
      reason: bad
      check_ops: [readdir, explode]
`)
	_, err := LoadConfig(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown check_op")
}

func TestLoadConfig_empty_path_in_entry(t *testing.T) {
	path := writeConfig(t, `
custom_paths:
  must_block:
    - label: noop
      severity: error
      reason: missing path
`)
	_, err := LoadConfig(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty path")
}

func TestLoadConfig_realfile(t *testing.T) {
	// Test against the public example config bundled with the repo.
	realFile := filepath.Join("..", "..", "tests", "example", "alice-sandbox.yaml")
	if _, err := os.Stat(realFile); os.IsNotExist(err) {
		t.Skip("alice-sandbox.yaml not present — run from repo root")
	}
	cfg, err := LoadConfig(realFile)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "alice", cfg.Identity.SandboxUser)
	assert.NotEmpty(t, cfg.CustomPaths.MustBlock)
	assert.NotEmpty(t, cfg.CustomPaths.MustRead)
	assert.NotEmpty(t, cfg.CustomPaths.MustReadWrite)
	assert.NotEmpty(t, cfg.CustomPaths.Audit)
}
