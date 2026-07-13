// Generates site/sample-data.json — a realistic multi-run history so the page's
// matrix / flip-log / charts have content to render before real CI data exists.
// Shape mirrors the aggregate job's output: [{os, harness, runTimestamp, report}].
// Not used in production; a dev fixture. Run: node scripts/gen-sample-data.mjs
import { writeFileSync } from "node:fs";

// finding_type builders keyed by capability category (see CONTEXT.md).
const VALUES = {
  fs_read: { findingType: "sensitive_readable_paths", task: "baseline_filesystem_enumerator",
    description: "Readable sensitive paths",
    value: ["~/.aws/credentials", "~/.ssh/id_rsa", "~/.kube/config"] },
  fs_write: { findingType: "writeable_paths", task: "baseline_path_task",
    description: "Writable system/home paths", value: ["~/.bashrc", "/etc/cron.d"] },
  net_dns: { findingType: "external_host_dns_resolution", task: "baseline_network_task",
    description: "Resolvable hosts", value: ["example.com", "169.254.169.254"] },
  net_conn: { findingType: "external_host_connectivity", task: "baseline_network_task",
    description: "Reachable hosts", value: ["1.1.1.1:443"] },
  ports_tcp: { findingType: "tcp_ports_open", task: "baseline_network_task",
    description: "Open TCP ports", value: [22, 631] },
  sockets: { findingType: "unix_socket_detection", task: "baseline_socket_task",
    description: "Unix sockets", value: ["/var/run/docker.sock"] },
  procs: { findingType: "process_detection", task: "baseline_process_task",
    description: "Visible process", value: { pid: 4211, name: "sshd" } },
  mounts: { findingType: "mounted_volumes_detections", task: "baseline_mount_task",
    description: "Mounted volumes", value: ["/", "/home"] },
};
// which VALUES entries belong to each leak category
const CAT = {
  fs_read: ["fs_read"], fs_write: ["fs_write"],
  net_egress: ["net_dns", "net_conn"], local_services: ["ports_tcp"],
  ipc_sockets: ["sockets"], process_visibility: ["procs"], host_mounts: ["mounts"],
};

function report({ os, ts, harness, probeCommit, kernel, sandbox, leaks, root }) {
  const findings = [];
  for (const cat of Object.keys(CAT)) {
    if (leaks.has(cat)) for (const k of CAT[cat]) findings.push({ ...VALUES[k] });
  }
  // privileged execution — absolute rule, euid 0 = exposed
  findings.push({ findingType: "user_context_detection", task: "baseline_user_context_task",
    description: "User identity", value: { uid: root ? 0 : 1001, gid: root ? 0 : 1001, euid: root ? 0 : 1001, egid: root ? 0 : 1001 } });
  findings.push({ findingType: "sandbox_detection", task: "baseline_sandbox_detector",
    description: "Container/wrapper runtime", value: sandbox });
  findings.push({ findingType: "environment_detection", task: "baseline_environment_task",
    description: "Host environment", value: { kernelRelease: kernel, osRelease: os } });
  const tags = [`os=${os}`, `harness=${harness}`];
  if (harness.includes("claude")) tags.push(`claude=${probeCommit.claudeVer}`);
  return {
    os, harness, runTimestamp: ts,
    report: {
      version: "1.0.0", timestamp: ts,
      probeBinary: { goVersion: "go1.26.1", os, arch: os === "macos" ? "arm64" : "amd64",
        binaryVersion: "dev", commit: probeCommit.probe, buildDate: ts },
      metadata: { tags }, findings,
    },
  };
}

const ALL = new Set(Object.keys(CAT));
const rows = [];
// five weekly runs; kernel bumps at run 3; claude version moves to create flips.
const runs = [
  { ts: "2026-05-04T08:00:00Z", probe: "aaaa111", kLinux: "6.8.0-1010", claudeVer: "2.1.202" },
  { ts: "2026-05-11T08:00:00Z", probe: "aaaa111", kLinux: "6.8.0-1010", claudeVer: "2.2.0" },
  { ts: "2026-05-18T08:00:00Z", probe: "bbbb222", kLinux: "6.11.0-1002", claudeVer: "2.2.0" },
  { ts: "2026-05-25T08:00:00Z", probe: "bbbb222", kLinux: "6.11.0-1002", claudeVer: "2.3.0" },
  { ts: "2026-06-01T08:00:00Z", probe: "bbbb222", kLinux: "6.11.0-1002", claudeVer: "2.3.0" },
];

for (const [i, r] of runs.entries()) {
  const pc = { probe: r.probe, claudeVer: r.claudeVer };
  const kL = r.kLinux, kM = "24.5.0";
  // ── linux baseline + harnesses ──
  rows.push(report({ os: "linux", ts: r.ts, harness: "direct", probeCommit: pc, kernel: kL, sandbox: "none", leaks: new Set(ALL), root: false }));
  // claude-sandbox: regresses net at run 2 (version), fs_read at run 3 (kernel), recovers net at run 4 (version)
  const claudeLeaks = new Set(["process_visibility"]);
  if (i >= 1) claudeLeaks.add("net_egress");
  if (i >= 2) claudeLeaks.add("fs_read");
  if (i >= 3) claudeLeaks.delete("net_egress");
  rows.push(report({ os: "linux", ts: r.ts, harness: "claude-sandbox", probeCommit: pc, kernel: kL, sandbox: "bubblewrap", leaks: claudeLeaks, root: false }));
  // codex-sandbox: consistently tight
  rows.push(report({ os: "linux", ts: r.ts, harness: "codex-sandbox", probeCommit: pc, kernel: kL, sandbox: "landlock", leaks: new Set(["process_visibility"]), root: false }));
  // gemini (unconfined agent): leaks almost everything, flat
  rows.push(report({ os: "linux", ts: r.ts, harness: "gemini", probeCommit: pc, kernel: kL, sandbox: "none",
    leaks: new Set(["fs_read", "fs_write", "net_egress", "local_services", "ipc_sockets", "process_visibility"]), root: false }));
  // docker runtime: tight but root inside
  rows.push(report({ os: "linux", ts: r.ts, harness: "docker", probeCommit: pc, kernel: kL, sandbox: "docker", leaks: new Set(["process_visibility"]), root: true }));
  // gvisor: tightest
  rows.push(report({ os: "linux", ts: r.ts, harness: "gvisor", probeCommit: pc, kernel: kL, sandbox: "gvisor", leaks: new Set(), root: false }));

  // ── macos baseline + harnesses ──
  rows.push(report({ os: "macos", ts: r.ts, harness: "direct", probeCommit: pc, kernel: kM, sandbox: "none", leaks: new Set(ALL), root: false }));
  const claudeMac = new Set(["process_visibility"]);
  if (i >= 3) claudeMac.add("fs_write"); // seatbelt profile loosened late
  rows.push(report({ os: "macos", ts: r.ts, harness: "claude-sandbox", probeCommit: pc, kernel: kM, sandbox: "seatbelt", leaks: claudeMac, root: false }));
  rows.push(report({ os: "macos", ts: r.ts, harness: "gemini-sandbox-exec", probeCommit: pc, kernel: kM, sandbox: "seatbelt", leaks: new Set(["process_visibility", "net_egress"]), root: false }));

  // ── windows: baseline + one unconfined agent (sockets/mounts naturally n/a) ──
  const winAll = new Set(["fs_read", "fs_write", "net_egress", "local_services", "process_visibility"]);
  rows.push(report({ os: "windows", ts: r.ts, harness: "direct", probeCommit: pc, kernel: "10.0.20348", sandbox: "none", leaks: winAll, root: false }));
  rows.push(report({ os: "windows", ts: r.ts, harness: "opencode", probeCommit: pc, kernel: "10.0.20348", sandbox: "none",
    leaks: new Set(["fs_read", "net_egress", "process_visibility"]), root: false }));
}

writeFileSync(new URL("../site/sample-data.json", import.meta.url), JSON.stringify(rows, null, 0));
console.log(`wrote ${rows.length} reports to site/sample-data.json`);
