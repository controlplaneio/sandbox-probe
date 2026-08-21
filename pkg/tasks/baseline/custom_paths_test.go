package tasks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/controlplaneio/sandbox-probe/v6/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckCustomPathsHonorsCheckOpsForMustRead(t *testing.T) {
	file := filepath.Join(t.TempDir(), "readable-file")
	require.NoError(t, os.WriteFile(file, []byte("contents"), 0600))

	results := CheckCustomPaths(&config.Config{CustomPaths: config.CustomPaths{
		MustRead: []config.PathEntry{{
			Path:     file,
			Label:    "readable-file",
			Severity: config.SeverityError,
			CheckOps: []config.CheckOp{config.OpOpen},
		}},
	}})

	require.Len(t, results, 1)
	assert.True(t, results[0].OpenOK)
	assert.Empty(t, results[0].Violations)
}

func TestCheckCustomPathsHonorsCheckOpsForMustReadWrite(t *testing.T) {
	file := filepath.Join(t.TempDir(), "readable-writable-file")
	require.NoError(t, os.WriteFile(file, []byte("contents"), 0600))

	results := CheckCustomPaths(&config.Config{CustomPaths: config.CustomPaths{
		MustReadWrite: []config.PathEntry{{
			Path:     file,
			Label:    "readable-writable-file",
			Severity: config.SeverityError,
			CheckOps: []config.CheckOp{config.OpOpen},
		}},
	}})

	require.Len(t, results, 1)
	assert.True(t, results[0].OpenOK)
	assert.Empty(t, results[0].Violations)
}

func TestCheckCustomPathsStatMayFailSkipsOnlyMissingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	results := CheckCustomPaths(&config.Config{CustomPaths: config.CustomPaths{
		MustRead: []config.PathEntry{{
			Path:        missing,
			Label:       "optional-read-path",
			Severity:    config.SeverityError,
			StatMayFail: true,
		}},
		MustReadWrite: []config.PathEntry{{
			Path:        missing,
			Label:       "optional-readwrite-path",
			Severity:    config.SeverityError,
			StatMayFail: true,
		}},
	}})

	require.Len(t, results, 2)
	assert.Empty(t, results[0].Violations)
	assert.Empty(t, results[1].Violations)
}

func TestCheckCustomPathsCheckFilesRespectsCheckOps(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "secret"), []byte("secret"), 0600))

	results := CheckCustomPaths(&config.Config{CustomPaths: config.CustomPaths{
		MustBlock: []config.PathEntry{{
			Path:       dir,
			Label:      "protected-dir",
			Severity:   config.SeverityCritical,
			CheckFiles: []string{"secret"},
			CheckOps:   []config.CheckOp{config.OpReaddir},
		}},
	}})

	require.Len(t, results, 2)
	assert.Len(t, results[0].Violations, 1, "the configured readdir check should run")
	assert.Empty(t, results[1].Violations, "check_files must not probe open when open is excluded")
}

func TestCheckCustomPathsAuditRespectsCheckOps(t *testing.T) {
	file := filepath.Join(t.TempDir(), "audit-file")
	require.NoError(t, os.WriteFile(file, []byte("contents"), 0600))

	results := CheckCustomPaths(&config.Config{CustomPaths: config.CustomPaths{
		Audit: []config.PathEntry{{
			Path:     file,
			Label:    "audit-file",
			CheckOps: []config.CheckOp{config.OpStat},
		}},
	}})

	require.Len(t, results, 1)
	assert.True(t, results[0].StatOK)
	assert.False(t, results[0].ReaddirOK)
	assert.False(t, results[0].OpenOK)
	assert.False(t, results[0].WriteOK)
}
