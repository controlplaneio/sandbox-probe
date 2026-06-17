package tasks

import (
	"context"
	"fmt"

	reportv1 "github.com/controlplaneio/sandbox-probe/api/gen/proto/report/v1"
	"github.com/controlplaneio/sandbox-probe/pkg/config"
	baselineTasks "github.com/controlplaneio/sandbox-probe/pkg/tasks/baseline"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	// FindingType for a path expectation violation (must_block / must_read / must_readwrite).
	CUSTOMPATHVIOLATION = "custom_path_violation"
	// FindingType for an audit-only path observation (no pass/fail).
	CUSTOMPATHAUDIT = "custom_path_audit"
)

// CustomPathsTask runs the custom_paths checks defined in a config file.
type CustomPathsTask struct {
	baseTask
	cfg *config.Config
}

// NewCustomPathsTask constructs a task from the loaded config.
func NewCustomPathsTask(cfg *config.Config) *CustomPathsTask {
	return &CustomPathsTask{
		baseTask: baseTask{
			name:        "custom_paths_checker",
			description: "Checks filesystem paths against must_block/must_read/must_readwrite/audit declarations",
		},
		cfg: cfg,
	}
}

func (t *CustomPathsTask) Run(_ context.Context, _ Inputs) ([]*reportv1.Finding, error) {
	log.Info().Str("task", t.GetName()).Msg("Starting custom path checks")

	results := baselineTasks.CheckCustomPaths(t.cfg)

	var findings []*reportv1.Finding
	for _, r := range results {
		if r.Category == "audit" {
			f, err := auditFinding(r)
			if err != nil {
				log.Warn().Err(err).Str("path", r.Entry.Path).Msg("Failed to build audit finding")
				continue
			}
			findings = append(findings, f)
			continue
		}

		if len(r.Violations) == 0 {
			// Pass — no finding emitted for clean paths
			continue
		}

		for _, v := range r.Violations {
			f, err := violationFinding(r, v)
			if err != nil {
				log.Warn().Err(err).Str("path", r.Entry.Path).Msg("Failed to build violation finding")
				continue
			}
			findings = append(findings, f)
		}
	}

	log.Info().
		Str("task", t.GetName()).
		Int("results", len(results)).
		Int("findings", len(findings)).
		Msg("Custom path checks complete")

	return findings, nil
}

func violationFinding(r baselineTasks.CheckResult, v baselineTasks.Violation) (*reportv1.Finding, error) {
	detail := map[string]interface{}{
		"path":     r.Entry.Path,
		"label":    r.Entry.Label,
		"category": r.Category,
		"op":       string(v.Op),
		"severity": string(v.Severity),
		"expected": fmt.Sprintf("%v", v.Expected),
		"got":      fmt.Sprintf("%v", v.Got),
		"message":  v.Message,
	}
	val, err := structpb.NewValue(detail)
	if err != nil {
		return nil, err
	}
	return &reportv1.Finding{
		FindingType: CUSTOMPATHVIOLATION,
		Task:        "custom_paths_checker",
		Description: fmt.Sprintf("[%s] %s", v.Severity, v.Message),
		Value:       val,
	}, nil
}

func auditFinding(r baselineTasks.CheckResult) (*reportv1.Finding, error) {
	detail := map[string]interface{}{
		"path":      r.Entry.Path,
		"label":     r.Entry.Label,
		"category":  "audit",
		"stat":      r.StatOK,
		"readdir":   r.ReaddirOK,
		"open":      r.OpenOK,
		"write":     r.WriteOK,
		"note":      r.Entry.Note,
	}
	val, err := structpb.NewValue(detail)
	if err != nil {
		return nil, err
	}
	return &reportv1.Finding{
		FindingType: CUSTOMPATHAUDIT,
		Task:        "custom_paths_checker",
		Description: fmt.Sprintf("[audit] %s (%s): stat=%v readdir=%v open=%v write=%v",
			r.Entry.Label, r.Entry.Path, r.StatOK, r.ReaddirOK, r.OpenOK, r.WriteOK),
		Value: val,
	}, nil
}
