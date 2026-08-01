package tasks

import (
	"context"
	"testing"

	"github.com/controlplaneio/sandbox-probe/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomPathsTaskSeverityControlsFailure(t *testing.T) {
	for _, tc := range []struct {
		name      string
		severity  config.Severity
		wantError bool
	}{
		{name: "critical aborts", severity: config.SeverityCritical, wantError: true},
		{name: "error fails scan", severity: config.SeverityError, wantError: true},
		{name: "warn reports only", severity: config.SeverityWarn, wantError: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{CustomPaths: config.CustomPaths{
				MustBlock: []config.PathEntry{{
					Path:     t.TempDir(),
					Label:    "readable-directory",
					Severity: tc.severity,
					Reason:   "test boundary",
				}},
			}}

			findings, err := NewCustomPathsTask(cfg).Run(context.Background(), Inputs{})
			require.NotEmpty(t, findings)
			if tc.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
