#!/usr/bin/env bash
# test-sandbox-boundaries.sh
# Self-contained CPAI sandbox boundary test harness.
# Verifies filesystem access matches the declarations in cpai-andy.yaml.
# Runs as cpai-andy inside nono sandbox — no sandbox-probe binary required.
#
# Usage:  bash test-sandbox-boundaries.sh
#         cpai run "bash /home/cpai-andy/src/cpai-workspace/sandbox-probe/test-sandbox-boundaries.sh"
#
# Exit code: 0 = all critical/error checks passed
#            1 = one or more critical failures
#            2 = one or more error-level failures (no critical failures)

set -uo pipefail

# ── Colours ─────────────────────────────────────────────────────────────────
RED='\033[0;31m'; YELLOW='\033[0;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; DIM='\033[2m'; RESET='\033[0m'

# ── Counters ─────────────────────────────────────────────────────────────────
PASS=0; FAIL_CRITICAL=0; FAIL_ERROR=0; FAIL_WARN=0; INFO=0

# ── Helpers ──────────────────────────────────────────────────────────────────
pass()     { echo -e "  ${GREEN}✅ PASS${RESET}   $*"; PASS=$((PASS+1)); }
fail_c()   { echo -e "  ${RED}${BOLD}🔴 CRITICAL${RESET} $*"; FAIL_CRITICAL=$((FAIL_CRITICAL+1)); }
fail_e()   { echo -e "  ${RED}❌ ERROR${RESET}  $*"; FAIL_ERROR=$((FAIL_ERROR+1)); }
fail_w()   { echo -e "  ${YELLOW}⚠️  WARN${RESET}   $*"; FAIL_WARN=$((FAIL_WARN+1)); }
info()     { echo -e "  ${DIM}ℹ️  INFO${RESET}    $*"; INFO=$((INFO+1)); }
section()  { echo -e "\n${CYAN}${BOLD}── $* ──${RESET}"; }
subsect()  { echo -e "\n${DIM}   $*${RESET}"; }

# Test whether a path can be stat'd (existence visible in VFS)
can_stat() { stat "$1" >/dev/null 2>&1; }

# Test whether a directory can be listed (readdir)
can_readdir() { ls "$1" >/dev/null 2>&1; }

# Test whether a file/dir can be opened for reading
can_open() {
  if [ -d "$1" ]; then
    # For dirs: try to open via python (ls might use getdents, open is different)
    python3 -c "import os; os.open('$1', os.O_RDONLY)" >/dev/null 2>&1
  else
    cat "$1" >/dev/null 2>&1
  fi
}

# Test whether we can create a file inside a directory
can_write() {
  local probe="${1}/.cpai_boundary_probe_$$"
  touch "$probe" >/dev/null 2>&1 && rm -f "$probe" >/dev/null 2>&1
}

# Check a path that MUST be blocked (readdir=DENIED, open=DENIED)
# stat() returning true is documented/acceptable (VFS path existence leak)
check_blocked() {
  local path="$1" label="$2" severity="$3" reason="$4"
  local stat_ok=0 readdir_ok=0 open_ok=0 write_ok=0

  can_stat    "$path" && stat_ok=1    || true
  can_readdir "$path" && readdir_ok=1 || true
  can_open    "$path" && open_ok=1    || true
  can_write   "$path" && write_ok=1   || true

  echo -e "\n   ${BOLD}${label}${RESET} ${DIM}(${path})${RESET}"
  echo -e "   ${DIM}stat=$stat_ok  readdir=$readdir_ok  open=$open_ok  write=$write_ok${RESET}"

  # stat visibility is expected/acceptable — log as info
  if [ "$stat_ok" -eq 1 ]; then
    info "Path existence visible via stat() — VFS leak, classified acceptable"
  else
    info "Path not stat-able — either doesn't exist or Landlock blocks stat"
  fi

  # readdir must be denied
  if [ "$readdir_ok" -eq 1 ]; then
    case "$severity" in
      critical) fail_c "readdir() ALLOWED on ${label} — ${reason}" ;;
      error)    fail_e "readdir() ALLOWED on ${label} — ${reason}" ;;
      *)        fail_w "readdir() ALLOWED on ${label} — ${reason}" ;;
    esac
  else
    pass "readdir() denied on ${label}"
  fi

  # open must be denied
  if [ "$open_ok" -eq 1 ]; then
    case "$severity" in
      critical) fail_c "open() ALLOWED on ${label} — ${reason}" ;;
      error)    fail_e "open() ALLOWED on ${label} — ${reason}" ;;
      *)        fail_w "open() ALLOWED on ${label} — ${reason}" ;;
    esac
  else
    pass "open() denied on ${label}"
  fi

  # write must be denied
  if [ "$write_ok" -eq 1 ]; then
    case "$severity" in
      critical) fail_c "write() ALLOWED on ${label} — ${reason}" ;;
      error)    fail_e "write() ALLOWED on ${label} — ${reason}" ;;
      *)        fail_w "write() ALLOWED on ${label} — ${reason}" ;;
    esac
  else
    pass "write() denied on ${label}"
  fi
}

# Check a specific file that must not be readable
check_file_blocked() {
  local path="$1" label="$2" severity="$3"
  echo -e "\n   ${BOLD}${label}${RESET} ${DIM}(${path})${RESET}"

  local stat_ok=0 open_ok=0
  can_stat "$path" && stat_ok=1 || true
  can_open "$path" && open_ok=1 || true
  echo -e "   ${DIM}stat=$stat_ok  open=$open_ok${RESET}"

  if [ "$stat_ok" -eq 1 ]; then
    info "File existence visible via stat() — path leaked but content protected"
  fi

  if [ "$open_ok" -eq 1 ]; then
    case "$severity" in
      critical) fail_c "File READABLE: ${label} at ${path}" ;;
      error)    fail_e "File READABLE: ${label} at ${path}" ;;
      *)        fail_w "File READABLE: ${label} at ${path}" ;;
    esac
  else
    pass "File not readable: ${label}"
  fi
}

# Check a path that MUST be readable (readdir=OK)
check_readable() {
  local path="$1" label="$2" reason="$3"
  echo -e "\n   ${BOLD}${label}${RESET} ${DIM}(${path})${RESET}"

  local readdir_ok=0
  can_readdir "$path" && readdir_ok=1 || true
  echo -e "   ${DIM}readdir=$readdir_ok${RESET}"

  if [ "$readdir_ok" -eq 1 ]; then
    pass "readdir() OK on ${label} — ${reason}"
  else
    fail_e "readdir() DENIED on ${label} — expected readable. ${reason}"
  fi
}

# Check a path that MUST be read+writable
check_readwrite() {
  local path="$1" label="$2" reason="$3"
  echo -e "\n   ${BOLD}${label}${RESET} ${DIM}(${path})${RESET}"

  local readdir_ok=0 write_ok=0
  can_readdir "$path" && readdir_ok=1 || true
  can_write   "$path" && write_ok=1   || true
  echo -e "   ${DIM}readdir=$readdir_ok  write=$write_ok${RESET}"

  if [ "$readdir_ok" -eq 1 ]; then
    pass "readdir() OK on ${label}"
  else
    fail_e "readdir() DENIED on ${label} — expected readable. ${reason}"
  fi

  if [ "$write_ok" -eq 1 ]; then
    pass "write() OK on ${label}"
  else
    fail_e "write() DENIED on ${label} — expected writable. ${reason}"
  fi
}

# Audit a path — report state, no pass/fail
audit_path() {
  local path="$1" label="$2" note="$3"
  echo -e "\n   ${BOLD}${label}${RESET} ${DIM}(${path})${RESET}"

  local stat_ok=0 readdir_ok=0 open_ok=0 write_ok=0
  can_stat    "$path" && stat_ok=1    || true
  can_readdir "$path" && readdir_ok=1 || true
  can_open    "$path" && open_ok=1    || true
  can_write   "$path" && write_ok=1   || true

  echo -e "   ${DIM}stat=$stat_ok  readdir=$readdir_ok  open=$open_ok  write=$write_ok${RESET}"
  info "${note}"
}

# ── Main ─────────────────────────────────────────────────────────────────────
echo ""
echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}${CYAN}║     CPAI Sandbox Boundary Test — cpai-andy @ fortuna     ║${RESET}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════════════════════╝${RESET}"
echo ""
echo -e "  Sandbox user : ${BOLD}$(id -u)${RESET} ($(whoami 2>/dev/null || echo 'uid=30033'))"
echo -e "  Host         : ${BOLD}$(hostname)${RESET}"
echo -e "  CWD          : ${BOLD}$(pwd)${RESET}"
echo -e "  Date         : ${BOLD}$(date -Iseconds)${RESET}"
echo -e "  nono version : ${BOLD}$(nono --version 2>&1 | head -1)${RESET}"

# ─────────────────────────────────────────────────────────────────────────────
section "MUST BLOCK — Cryptographic secrets & credentials"
# These are CRITICAL: any readability here = real security failure

check_blocked /home/andy/.ssh       "ssh_keys"       "critical" "SSH private keys"
check_blocked /home/andy/.aws       "aws_creds"      "critical" "AWS credentials"
check_blocked /home/andy/.gnupg     "gpg_keys"       "critical" "GPG private keys"
check_blocked /home/andy/.kube      "kube_config"    "critical" "Kubernetes credentials"
check_blocked /home/andy/.config    "host_config"    "error"    "Host config (gcloud, 1password, etc.)"
check_blocked /home/andy/Dropbox    "dropbox"        "error"    "Personal cloud sync"

subsect "Known-path file checks (stat leaks existence, open must be denied)"
check_file_blocked /home/andy/.ssh/id_rsa            "ssh_id_rsa"        "critical"
check_file_blocked /home/andy/.ssh/id_ed25519        "ssh_id_ed25519"    "critical"
check_file_blocked /home/andy/.aws/credentials       "aws_credentials"   "critical"
check_file_blocked /home/andy/.kube/config           "kube_config_file"  "critical"

# ─────────────────────────────────────────────────────────────────────────────
section "MUST BLOCK — Host source code (enumeration risk)"

check_blocked /home/andy/src          "host_src_root"  "error" "Host project tree"
check_blocked /home/andy/src/cp       "host_src_cp"    "error" "ControlPlane projects (may contain .env secrets)"
check_blocked /home/andy/src/2026     "host_src_2026"  "error" "Current year's projects"

# ─────────────────────────────────────────────────────────────────────────────
section "MUST BLOCK — System write paths (must not be writable)"

check_blocked /usr/local/src   "usr_local_src"   "warn"  "/usr/local/src must not be writable"

# ─────────────────────────────────────────────────────────────────────────────
section "MUST READ — System tooling (expected readable)"

check_readable /usr/local/bin  "usr_local_bin"  "User-installed tools must be accessible"
check_readable /usr/bin        "usr_bin"         "System binaries must be accessible"

# ─────────────────────────────────────────────────────────────────────────────
section "MUST READWRITE — Sandbox working area"

check_readwrite /home/cpai-andy/src/cpai-workspace  "cpai_workspace"  "nono --allow-cwd"

# /tmp: write is always allowed (system_write_linux group).
# readdir of /tmp is intentionally blocked — we don't need to enumerate it.
# read-by-known-path requires $TMPDIR in filesystem.allow (see profile patch).
# We test what the current profile SHOULD provide: write=OK.
# The profile patch test (read-by-known-path) is run separately below.
echo ""
echo -e "   ${BOLD}tmp${RESET} ${DIM}(/tmp)${RESET}"
tmp_write=0
TF=/tmp/.cpai_boundary_probe_$$
echo "probe" > "$TF" 2>/dev/null && tmp_write=1 && rm -f "$TF" 2>/dev/null || true
echo -e "   ${DIM}write=${tmp_write}${RESET}"
if [ "$tmp_write" -eq 1 ]; then
  pass "write() OK on tmp (system_write_linux group)"
else
  fail_e "write() DENIED on tmp — unexpected, system_write_linux should grant this"
fi

# readdir of /tmp: intentionally blocked — document as INFO not failure
tmp_readdir=0
ls /tmp >/dev/null 2>&1 && tmp_readdir=1 || true
if [ "$tmp_readdir" -eq 0 ]; then
  info "/tmp readdir blocked — correct. Enumeration of /tmp not needed or desired."
else
  fail_w "/tmp readdir is open — not a security failure, but worth noting"
fi

# Read-by-known-path (requires profile patch):
TF2=/tmp/.cpai_read_probe_$$
echo "probe" > "$TF2" 2>/dev/null || true
tmp_read=0
cat "$TF2" >/dev/null 2>&1 && tmp_read=1 || true
rm -f "$TF2" 2>/dev/null || true
echo -e "   ${DIM}read_by_known_path=${tmp_read}${RESET}"
if [ "$tmp_read" -eq 1 ]; then
  pass "read-by-known-path OK on tmp (profile patch applied ✓)"
else
  info "/tmp read-by-known-path BLOCKED — apply cpai-opencode-local-patch.json to enable"
  info "  See: sandbox-probe/cpai-opencode-local-patch.json"
  info "  Run: nono profile promote cpai-opencode-local  (after andy applies the patch)"
fi

# ─────────────────────────────────────────────────────────────────────────────
section "AUDIT — Informational (no pass/fail)"

audit_path /home/andy              "host_home_root"  "mode=0710 drwx--x--- gid=30037. Path visible, content blocked."
audit_path /home/andy/Documents    "host_documents"  "mode=0755 world-readable by DAC, but Landlock blocks content."
audit_path /home/cpai-andy         "cpai_home"       "Own home — mostly Landlock-blocked except explicit allows."
audit_path /home/cpai-andy/.opencode "opencode_data" "Allowed r+w by opencode profile group."
audit_path /proc/self              "proc_self"       "Own process — readable, expected."

# ─────────────────────────────────────────────────────────────────────────────
section "VERDICT"

TOTAL_CHECKS=$((PASS + FAIL_CRITICAL + FAIL_ERROR + FAIL_WARN))
echo ""
echo -e "  Total checks : ${BOLD}${TOTAL_CHECKS}${RESET}"
echo -e "  ${GREEN}Passed${RESET}       : ${BOLD}${PASS}${RESET}"
echo -e "  ${RED}Critical${RESET}     : ${BOLD}${FAIL_CRITICAL}${RESET}"
echo -e "  ${RED}Errors${RESET}       : ${BOLD}${FAIL_ERROR}${RESET}"
echo -e "  ${YELLOW}Warnings${RESET}     : ${BOLD}${FAIL_WARN}${RESET}"
echo -e "  ${DIM}Info${RESET}         : ${BOLD}${INFO}${RESET}"
echo ""

if [ "$FAIL_CRITICAL" -gt 0 ]; then
  echo -e "  ${RED}${BOLD}🔴 VERDICT: CRITICAL FAILURE — sandbox has unacceptable access leaks${RESET}"
  echo ""
  exit 1
elif [ "$FAIL_ERROR" -gt 0 ]; then
  echo -e "  ${RED}${BOLD}❌ VERDICT: FAIL — sandbox has error-level access violations${RESET}"
  echo ""
  exit 2
elif [ "$FAIL_WARN" -gt 0 ]; then
  echo -e "  ${YELLOW}${BOLD}⚠️  VERDICT: WARN — sandbox passes with warnings${RESET}"
  echo ""
  exit 0
else
  echo -e "  ${GREEN}${BOLD}✅ VERDICT: PASS — sandbox boundaries correctly configured${RESET}"
  echo ""
  exit 0
fi
