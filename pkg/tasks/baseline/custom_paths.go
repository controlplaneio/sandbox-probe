package tasks

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/controlplaneio/sandbox-probe/pkg/config"
)

// CheckResult holds the outcome of probing a single path entry.
type CheckResult struct {
	Entry    config.PathEntry
	Category string // "must_block" | "must_read" | "must_readwrite" | "audit"

	StatOK    bool
	ReaddirOK bool
	OpenOK    bool
	WriteOK   bool

	// Violations is non-empty for must_* categories when an op fails its
	// expectation. Empty for audit entries.
	Violations []Violation
}

// Violation records a single failing operation check.
type Violation struct {
	Op       config.CheckOp
	Expected bool // true = expected accessible, false = expected denied
	Got      bool // actual result
	Severity config.Severity
	Message  string
}

// CheckCustomPaths runs all four categories from the config and returns
// one CheckResult per path entry (plus one per check_files sub-entry).
func CheckCustomPaths(cfg *config.Config) []CheckResult {
	var results []CheckResult

	for _, e := range cfg.CustomPaths.MustBlock {
		results = append(results, checkMustBlock(e))
		// Per-file checks inside the directory
		for _, fname := range e.CheckFiles {
			filePath := filepath.Join(e.Path, fname)
			fileEntry := config.PathEntry{
				Path:     filePath,
				Label:    e.Label + "/" + fname,
				Severity: e.Severity,
				Reason:   e.Reason,
			}
			results = append(results, checkMustBlockFile(fileEntry))
		}
	}

	for _, e := range cfg.CustomPaths.MustRead {
		results = append(results, checkMustRead(e))
	}

	for _, e := range cfg.CustomPaths.MustReadWrite {
		results = append(results, checkMustReadWrite(e))
	}

	for _, e := range cfg.CustomPaths.Audit {
		results = append(results, auditPath(e))
	}

	return results
}

// checkMustBlock: readdir=denied AND open=denied (write=denied too).
// stat visibility is acceptable (VFS path existence leak) — documented as info.
func checkMustBlock(e config.PathEntry) CheckResult {
	r := CheckResult{Entry: e, Category: "must_block"}
	r.StatOK = canStat(e.Path)
	r.ReaddirOK = e.HasOp(config.OpReaddir) && canReaddir(e.Path)
	r.OpenOK = e.HasOp(config.OpOpen) && canOpen(e.Path)
	r.WriteOK = e.HasOp(config.OpWrite) && canWrite(e.Path)

	if e.HasOp(config.OpReaddir) && r.ReaddirOK {
		r.Violations = append(r.Violations, Violation{
			Op:       config.OpReaddir,
			Expected: false,
			Got:      true,
			Severity: e.Severity,
			Message:  fmt.Sprintf("readdir() ALLOWED on %s — %s", e.Label, e.Reason),
		})
	}
	if e.HasOp(config.OpOpen) && r.OpenOK {
		r.Violations = append(r.Violations, Violation{
			Op:       config.OpOpen,
			Expected: false,
			Got:      true,
			Severity: e.Severity,
			Message:  fmt.Sprintf("open() ALLOWED on %s — %s", e.Label, e.Reason),
		})
	}
	if e.HasOp(config.OpWrite) && r.WriteOK {
		r.Violations = append(r.Violations, Violation{
			Op:       config.OpWrite,
			Expected: false,
			Got:      true,
			Severity: e.Severity,
			Message:  fmt.Sprintf("write() ALLOWED on %s — %s", e.Label, e.Reason),
		})
	}
	return r
}

// checkMustBlockFile: individual file inside a must_block dir.
// Only checks open() (readdir is already covered by the parent dir check).
func checkMustBlockFile(e config.PathEntry) CheckResult {
	r := CheckResult{Entry: e, Category: "must_block_file"}
	r.StatOK = canStat(e.Path)
	r.OpenOK = canOpen(e.Path)

	if r.OpenOK {
		r.Violations = append(r.Violations, Violation{
			Op:       config.OpOpen,
			Expected: false,
			Got:      true,
			Severity: e.Severity,
			Message:  fmt.Sprintf("file open() ALLOWED: %s — %s", e.Label, e.Reason),
		})
	}
	return r
}

// checkMustRead: readdir=ok required.
func checkMustRead(e config.PathEntry) CheckResult {
	r := CheckResult{Entry: e, Category: "must_read"}
	r.StatOK = canStat(e.Path)
	r.ReaddirOK = canReaddir(e.Path)

	if !r.ReaddirOK {
		r.Violations = append(r.Violations, Violation{
			Op:       config.OpReaddir,
			Expected: true,
			Got:      false,
			Severity: e.Severity,
			Message:  fmt.Sprintf("readdir() DENIED on %s — expected readable: %s", e.Label, e.Reason),
		})
	}
	return r
}

// checkMustReadWrite: readdir=ok AND write=ok required.
func checkMustReadWrite(e config.PathEntry) CheckResult {
	r := CheckResult{Entry: e, Category: "must_readwrite"}
	r.StatOK = canStat(e.Path)
	r.ReaddirOK = canReaddir(e.Path)
	r.WriteOK = canWrite(e.Path)

	if !r.ReaddirOK {
		r.Violations = append(r.Violations, Violation{
			Op:       config.OpReaddir,
			Expected: true,
			Got:      false,
			Severity: e.Severity,
			Message:  fmt.Sprintf("readdir() DENIED on %s — expected readable: %s", e.Label, e.Reason),
		})
	}
	if !r.WriteOK {
		r.Violations = append(r.Violations, Violation{
			Op:       config.OpWrite,
			Expected: true,
			Got:      false,
			Severity: e.Severity,
			Message:  fmt.Sprintf("write() DENIED on %s — expected writable: %s", e.Label, e.Reason),
		})
	}
	return r
}

// auditPath: probe all ops, no pass/fail verdict.
func auditPath(e config.PathEntry) CheckResult {
	return CheckResult{
		Entry:     e,
		Category:  "audit",
		StatOK:    canStat(e.Path),
		ReaddirOK: canReaddir(e.Path),
		OpenOK:    canOpen(e.Path),
		WriteOK:   canWrite(e.Path),
		// Violations intentionally empty — audit has no pass/fail
	}
}

// ── Low-level probes ──────────────────────────────────────────────────────
// canStat/canReaddir wrap stdlib directly; canOpen/canWrite delegate to the
// existing isReadable/isWritable helpers in filesystem.go (same package).

func canStat(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func canReaddir(path string) bool {
	entries, err := os.ReadDir(path)
	// A non-nil error means denied (or not a dir). An empty dir is fine.
	_ = entries
	return err == nil
}

func canOpen(path string) bool {
	return isReadable(path)
}

func canWrite(path string) bool {
	return isWritable(path)
}
