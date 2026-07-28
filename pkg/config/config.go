// Package config loads and validates the sandbox-probe custom-paths config file.
//
// Schema (YAML):
//
//	identity:
//	  sandbox_user: alice           # AI agent unix user
//	  sandbox_uid: 1001
//	  host_user: bob                # human operator unix user
//	  host_uid: 1000
//	  shared_gid: 1002              # optional: GID shared between sandbox and host
//	  nono_profile: my-profile      # optional: nono profile name (informational)
//	custom_paths:
//	  must_block:    []PathEntry
//	  must_read:     []PathEntry
//	  must_readwrite: []PathEntry
//	  audit:         []PathEntry
package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Severity levels for path expectations.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityError    Severity = "error"
	SeverityWarn     Severity = "warn"
)

// CheckOp names for per-path operation overrides.
type CheckOp string

const (
	OpStat    CheckOp = "stat"
	OpReaddir CheckOp = "readdir"
	OpOpen    CheckOp = "open"
	OpWrite   CheckOp = "write"
)

// PathEntry describes a single path expectation.
type PathEntry struct {
	// Path is the filesystem path to probe.
	Path string `yaml:"path"`
	// Label is a short identifier used in findings and output.
	Label string `yaml:"label"`
	// Severity applies to must_block and must_read/must_readwrite violations.
	Severity Severity `yaml:"severity"`
	// Reason is a human-readable explanation included in findings.
	Reason string `yaml:"reason"`
	// Note is used for audit entries (no pass/fail).
	Note string `yaml:"note"`
	// CheckFiles are individual file names to probe inside this directory
	// (must_block only: each file is checked for open=denied).
	CheckFiles []string `yaml:"check_files"`
	// CheckOps restricts which operations are tested. When empty all relevant
	// ops for the category are tested.
	CheckOps []CheckOp `yaml:"check_ops"`
	// StatMayFail marks paths that may not exist on all machines (no failure
	// if stat returns ENOENT).
	StatMayFail bool `yaml:"stat_may_fail"`
}

// CustomPaths holds the four path categories.
type CustomPaths struct {
	MustBlock     []PathEntry `yaml:"must_block"`
	MustRead      []PathEntry `yaml:"must_read"`
	MustReadWrite []PathEntry `yaml:"must_readwrite"`
	Audit         []PathEntry `yaml:"audit"`
}

// Identity describes the sandbox identity context (informational).
type Identity struct {
	SandboxUser string `yaml:"sandbox_user"`
	SandboxUID  int    `yaml:"sandbox_uid"`
	HostUser    string `yaml:"host_user"`
	HostUID     int    `yaml:"host_uid"`
	SharedGID   int    `yaml:"shared_gid"`
	NonoProfile string `yaml:"nono_profile"`
}

// Config is the top-level config file structure.
type Config struct {
	Identity    Identity    `yaml:"identity"`
	CustomPaths CustomPaths `yaml:"custom_paths"`
}

// LoadConfig reads and parses a YAML config file.
// Returns nil, nil when path is empty (no config provided).
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %q: %w", path, err)
	}

	var cfg Config
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config %q: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("parsing config %q: multiple YAML documents are not supported", path)
		}
		return nil, fmt.Errorf("parsing config %q: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config %q: %w", path, err)
	}

	return &cfg, nil
}

// validate performs basic sanity checks on the loaded config.
func (c *Config) validate() error {
	paths := c.CustomPaths
	if len(paths.MustBlock)+len(paths.MustRead)+len(paths.MustReadWrite)+len(paths.Audit) == 0 {
		return fmt.Errorf("custom_paths must contain at least one path entry")
	}
	if err := validateEntries("must_block", paths.MustBlock, true, OpReaddir, OpOpen, OpWrite); err != nil {
		return err
	}
	if err := validateEntries("must_read", paths.MustRead, true, OpStat, OpReaddir, OpOpen); err != nil {
		return err
	}
	if err := validateEntries("must_readwrite", paths.MustReadWrite, true, OpStat, OpReaddir, OpOpen, OpWrite); err != nil {
		return err
	}
	return validateEntries("audit", paths.Audit, false, OpStat, OpReaddir, OpOpen, OpWrite)
}

func validateEntries(category string, entries []PathEntry, severityRequired bool, allowedOps ...CheckOp) error {
	allowed := make(map[CheckOp]struct{}, len(allowedOps))
	for _, op := range allowedOps {
		allowed[op] = struct{}{}
	}

	for _, e := range entries {
		if e.Path == "" {
			return fmt.Errorf("%s path entry with label %q has empty path", category, e.Label)
		}
		if !filepath.IsAbs(e.Path) {
			return fmt.Errorf("%s path %q must be absolute", category, e.Path)
		}
		for _, op := range e.CheckOps {
			if !isKnownCheckOp(op) {
				return fmt.Errorf("path %q has unknown check_op %q", e.Path, op)
			}
			if _, ok := allowed[op]; !ok {
				return fmt.Errorf("%s path %q does not support check_op %q", category, e.Path, op)
			}
		}
		if category != "must_block" && len(e.CheckFiles) > 0 {
			return fmt.Errorf("%s path %q cannot use check_files", category, e.Path)
		}
		for _, file := range e.CheckFiles {
			if file == "" || file == "." || filepath.IsAbs(file) || filepath.Base(file) != file {
				return fmt.Errorf("must_block path %q has invalid check_file %q", e.Path, file)
			}
		}
		switch e.Severity {
		case SeverityCritical, SeverityError, SeverityWarn:
			// valid
		case "":
			if severityRequired {
				return fmt.Errorf("%s path %q is missing severity", category, e.Path)
			}
		default:
			return fmt.Errorf("path %q has unknown severity %q", e.Path, e.Severity)
		}
	}
	return nil
}

func isKnownCheckOp(op CheckOp) bool {
	switch op {
	case OpStat, OpReaddir, OpOpen, OpWrite:
		return true
	default:
		return false
	}
}

// HasOp returns true when the entry has no CheckOps override (all ops apply)
// or when the given op is explicitly listed.
func (e *PathEntry) HasOp(op CheckOp) bool {
	if len(e.CheckOps) == 0 {
		return true
	}
	for _, o := range e.CheckOps {
		if o == op {
			return true
		}
	}
	return false
}
