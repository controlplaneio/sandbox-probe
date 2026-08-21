package tasks

import (
	"errors"
	"fmt"
	"io/fs"
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
		// Per-file checks inside the directory.
		// CheckOps is propagated so that a user-specified check_ops override on
		// the parent directory entry is respected for the per-file open checks too.
		for _, fname := range e.CheckFiles {
			filePath := filepath.Join(e.Path, fname)
			fileEntry := config.PathEntry{
				Path:     filePath,
				Label:    e.Label + "/" + fname,
				Severity: e.Severity,
				Reason:   e.Reason,
				CheckOps: e.CheckOps,
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
	r.ReaddirOK = shouldCheck(e, config.OpReaddir, config.OpReaddir, config.OpOpen, config.OpWrite) && canReaddir(e.Path)
	r.OpenOK = shouldCheck(e, config.OpOpen, config.OpReaddir, config.OpOpen, config.OpWrite) && canOpen(e.Path)
	r.WriteOK = shouldCheck(e, config.OpWrite, config.OpReaddir, config.OpOpen, config.OpWrite) && canWrite(e.Path)

	if shouldCheck(e, config.OpReaddir, config.OpReaddir, config.OpOpen, config.OpWrite) && r.ReaddirOK {
		r.Violations = append(r.Violations, Violation{
			Op:       config.OpReaddir,
			Expected: false,
			Got:      true,
			Severity: e.Severity,
			Message:  fmt.Sprintf("readdir() ALLOWED on %s — %s", e.Label, e.Reason),
		})
	}
	if shouldCheck(e, config.OpOpen, config.OpReaddir, config.OpOpen, config.OpWrite) && r.OpenOK {
		r.Violations = append(r.Violations, Violation{
			Op:       config.OpOpen,
			Expected: false,
			Got:      true,
			Severity: e.Severity,
			Message:  fmt.Sprintf("open() ALLOWED on %s — %s", e.Label, e.Reason),
		})
	}
	if shouldCheck(e, config.OpWrite, config.OpReaddir, config.OpOpen, config.OpWrite) && r.WriteOK {
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
// Respects CheckOps: if the parent entry explicitly excluded OpOpen via
// check_ops, no open violation is raised for per-file entries either.
func checkMustBlockFile(e config.PathEntry) CheckResult {
	r := CheckResult{Entry: e, Category: "must_block_file"}
	r.StatOK = canStat(e.Path)
	r.OpenOK = shouldCheck(e, config.OpOpen, config.OpOpen) && canOpen(e.Path)

	if shouldCheck(e, config.OpOpen, config.OpOpen) && r.OpenOK {
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

// checkMustRead verifies the configured read operations. By default it checks
// readdir; check_ops can select stat, readdir, or open explicitly.
func checkMustRead(e config.PathEntry) CheckResult {
	r := CheckResult{Entry: e, Category: "must_read"}
	r.StatOK = canStat(e.Path)

	if statMayFail(e) {
		return r
	}

	r.ReaddirOK = shouldCheck(e, config.OpReaddir, config.OpReaddir) && canReaddir(e.Path)
	r.OpenOK = shouldCheck(e, config.OpOpen, config.OpReaddir) && canOpen(e.Path)
	appendAccessibilityViolations(&r, e, config.OpStat, r.StatOK, config.OpReaddir, r.ReaddirOK, config.OpOpen, r.OpenOK)
	return r
}

// checkMustReadWrite verifies the configured read-write operations. By default
// it checks readdir and write; check_ops can select individual operations.
func checkMustReadWrite(e config.PathEntry) CheckResult {
	r := CheckResult{Entry: e, Category: "must_readwrite"}
	r.StatOK = canStat(e.Path)

	if statMayFail(e) {
		return r
	}

	r.ReaddirOK = shouldCheck(e, config.OpReaddir, config.OpReaddir, config.OpWrite) && canReaddir(e.Path)
	r.OpenOK = shouldCheck(e, config.OpOpen, config.OpReaddir, config.OpWrite) && canOpen(e.Path)
	r.WriteOK = shouldCheck(e, config.OpWrite, config.OpReaddir, config.OpWrite) && canWrite(e.Path)
	appendAccessibilityViolations(&r, e, config.OpStat, r.StatOK, config.OpReaddir, r.ReaddirOK, config.OpOpen, r.OpenOK, config.OpWrite, r.WriteOK)
	return r
}

// auditPath: probe all ops, no pass/fail verdict.
func auditPath(e config.PathEntry) CheckResult {
	return CheckResult{
		Entry:     e,
		Category:  "audit",
		StatOK:    shouldCheck(e, config.OpStat, config.OpStat, config.OpReaddir, config.OpOpen, config.OpWrite) && canStat(e.Path),
		ReaddirOK: shouldCheck(e, config.OpReaddir, config.OpStat, config.OpReaddir, config.OpOpen, config.OpWrite) && canReaddir(e.Path),
		OpenOK:    shouldCheck(e, config.OpOpen, config.OpStat, config.OpReaddir, config.OpOpen, config.OpWrite) && canOpen(e.Path),
		WriteOK:   shouldCheck(e, config.OpWrite, config.OpStat, config.OpReaddir, config.OpOpen, config.OpWrite) && canWrite(e.Path),
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

func statMayFail(e config.PathEntry) bool {
	if !e.StatMayFail {
		return false
	}
	_, err := os.Stat(e.Path)
	return errors.Is(err, fs.ErrNotExist)
}

func shouldCheck(e config.PathEntry, op config.CheckOp, defaults ...config.CheckOp) bool {
	if len(e.CheckOps) > 0 {
		return e.HasOp(op)
	}
	for _, defaultOp := range defaults {
		if op == defaultOp {
			return true
		}
	}
	return false
}

func appendAccessibilityViolations(r *CheckResult, e config.PathEntry, operations ...interface{}) {
	for i := 0; i < len(operations); i += 2 {
		op := operations[i].(config.CheckOp)
		got := operations[i+1].(bool)
		if !shouldCheck(e, op, defaultOpsFor(r.Category)...) || got {
			continue
		}
		r.Violations = append(r.Violations, Violation{
			Op:       op,
			Expected: true,
			Got:      false,
			Severity: e.Severity,
			Message:  fmt.Sprintf("%s() DENIED on %s — expected accessible: %s", op, e.Label, e.Reason),
		})
	}
}

func defaultOpsFor(category string) []config.CheckOp {
	switch category {
	case "must_read":
		return []config.CheckOp{config.OpReaddir}
	case "must_readwrite":
		return []config.CheckOp{config.OpReaddir, config.OpWrite}
	default:
		return nil
	}
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
