---
task: Implement --config custom_paths in sandbox-probe scan
slug: 20260523-113700_custom-path-config
effort: advanced
phase: execute
progress: 0/28
mode: interactive
started: 2026-05-23T11:37:00+01:00
updated: 2026-05-23T11:37:00+01:00
---

## Context

The agent wrote a declarative YAML spec (`cpai-andy.yaml`) defining must_block /
must_read / must_readwrite / audit path categories. The `scan` command has no
`--config` flag and no concept of custom_paths. This task implements the full
pipeline: config schema → loader → custom-paths task → findings → scan wiring.

Branch: feat/custom-path-config (off feat/bwrap-detection)

## Criteria

- [ ] ISC-1: `--config <path>` flag added to root cmd and bound to cfgFile
- [ ] ISC-2: `pkg/config/config.go` defines Config, Identity, CustomPaths, PathEntry structs matching YAML schema
- [ ] ISC-3: Config.identity fields (sandbox_user, sandbox_uid, host_user, host_uid, shared_gid, nono_profile) parsed
- [ ] ISC-4: Config.custom_paths.must_block []PathEntry parsed (path, label, severity, reason, check_files, check_ops, stat_may_fail)
- [ ] ISC-5: Config.custom_paths.must_read []PathEntry parsed
- [ ] ISC-6: Config.custom_paths.must_readwrite []PathEntry parsed
- [ ] ISC-7: Config.custom_paths.audit []PathEntry parsed (path, label, note)
- [ ] ISC-8: LoadConfig(path) returns *Config, error; returns nil,nil when path is empty
- [ ] ISC-9: `pkg/tasks/baseline/custom_paths.go` implements CheckOps (stat, readdir, open, write) per path
- [ ] ISC-10: must_block: readdir=denied AND open=denied required; stat leak documented as info
- [ ] ISC-11: must_read: readdir=ok required
- [ ] ISC-12: must_readwrite: readdir=ok AND write=ok required
- [ ] ISC-13: audit: all four ops probed, no pass/fail verdict, result reported
- [ ] ISC-14: check_ops override applies when set (limits which ops are tested)
- [ ] ISC-15: check_files sub-entries on must_block paths tested for open=denied individually
- [ ] ISC-16: Severity maps to Finding severity field: critical→ERROR, error→WARNING, warn→INFO (or equivalent)
- [ ] ISC-17: `pkg/tasks/custom_paths_task.go` implements Task interface, wraps custom path checker
- [ ] ISC-18: Task name is `custom_paths_checker`
- [ ] ISC-19: Task registered as a named task (not in baseline taskset — opt-in via --tasks or --config)
- [ ] ISC-20: scan.go loads config after flag parse; if config present, appends CustomPathsTask to loaded tasks
- [ ] ISC-21: Findings use FindingType `custom_path_violation` for failures and `custom_path_audit` for audit entries
- [ ] ISC-22: Finding.Description includes label, path, op, and severity
- [ ] ISC-23: `--config` flag appears in `sandbox-probe scan --help`
- [ ] ISC-24: `go test ./pkg/config/...` passes (LoadConfig unit tests)
- [ ] ISC-25: `go test ./pkg/tasks/...` passes (custom path checker unit tests)
- [ ] ISC-26: `make build` produces working binary
- [ ] ISC-27: `sandbox-probe scan --config tests/cpai-andy/cpai-andy.yaml` runs without panic
- [ ] ISC-28: ShellCheck / existing tests unaffected (`make tests` passes)

## Decisions

- Config package lives at pkg/config/ — separate from tasks, imported by both cmd and task
- Custom paths task is NOT in baseline taskset — loaded only when --config is provided
- Finding severity: reuse existing reportv1.Finding fields (FindingType + Description encodes severity)
- isReadable/isWritable reused from filesystem.go (same package, no duplication)
