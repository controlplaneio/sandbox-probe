package tasks

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// scratchDir gives a short /tmp base: macOS caps Unix socket bind paths at ~104
// bytes, which t.TempDir()'s nesting blows. Tests own everything under it — no
// catalogue path is ever seeded for real.
func scratchDir(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Unix domain sockets are POSIX-only")
	}
	dir, err := os.MkdirTemp("/tmp", "spseed")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// scanned reports whether the probe's own socket scan of root finds path.
// Roots are symlink-resolved (/tmp -> /private/tmp on macOS), so the resolved
// form counts too.
func scanned(t *testing.T, root, path string) bool {
	t.Helper()
	got, err := ScanSocketRoots([]string{root}, false)
	if err != nil {
		t.Fatal(err)
	}
	real, _ := filepath.EvalSymlinks(path)
	return slices.Contains(got, path) || (real != "" && slices.Contains(got, real))
}

func socketTarget(path string) Target {
	return Target{Path: path, Kind: "socket", Seedable: true, Category: "container-runtime", Evidence: "empirical-own-machine"}
}

func processTarget(name string) Target {
	return Target{Path: name, Kind: "process", Seedable: true, Category: "container-runtime", Evidence: "empirical-own-machine"}
}

// testDecoyName is a command name this test invented: distinctive enough that
// nothing else on the machine can hold it, and never one of the catalogue's, so
// a test never signals or counts a real process.
func testDecoyName(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("sandbox-probe-test-decoy-%s-%d", t.Name(), os.Getpid())
}

// shortLifetime cuts the self-timeout down for the duration of one test, so no
// decoy a test spawns can outlive it by more than d even if the test fails.
func shortLifetime(t *testing.T, d time.Duration) {
	t.Helper()
	prev := decoyProcessLifetime
	decoyProcessLifetime = d
	t.Cleanup(func() { decoyProcessLifetime = prev })
}

// processScanned reports whether the probe's own process scan sees a process
// running under name.
func processScanned(t *testing.T, name string) bool {
	t.Helper()
	procs, err := GetRunningProcesses()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range procs {
		if strings.Contains(p.Command, name) {
			return true
		}
	}
	return false
}

// The process round trip, the same shape as the socket one: the probe's own
// process scan does not report the decoy before seeding, does after it, and
// does not again once cleanup has terminated it.
func TestSeedScanCleanupProcessRoundTrip(t *testing.T) {
	dir := scratchDir(t)
	if procs, err := GetRunningProcesses(); err != nil || len(procs) == 0 {
		t.Skipf("the probe has no process scan on %s", runtime.GOOS)
	}
	record := filepath.Join(dir, "record.json")
	name := testDecoyName(t)
	shortLifetime(t, 30*time.Second)
	t.Cleanup(func() { _, _ = CleanupSeeded(record) })

	if processScanned(t, name) {
		t.Fatal("the decoy is reported before seeding")
	}

	res, err := SeedTargets([]Target{processTarget(name)}, record)
	if err != nil {
		t.Fatal(err)
	}
	if res.Planted != 1 || res.Skipped != 0 {
		t.Fatalf("seed = planted %d skipped %d, want 1/0", res.Planted, res.Skipped)
	}
	// The decoy is a process of its own, so give the scan a moment to catch it.
	for deadline := time.Now().Add(5 * time.Second); !processScanned(t, name); {
		if time.Now().After(deadline) {
			t.Fatal("the decoy is not reported after seeding")
		}
		time.Sleep(50 * time.Millisecond)
	}

	removed, err := CleanupSeeded(record)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Errorf("cleanup removed %d, want 1", removed)
	}
	if processScanned(t, name) {
		t.Fatal("the decoy is still reported after cleanup")
	}
}

// The belt: a decoy dies on its own, so a cleanup that never runs — a crashed
// scan, a machine losing power — is not a permanent leak. No cleanup step runs
// in this test at all.
func TestSeededProcessExpiresWithNoCleanup(t *testing.T) {
	if decoyProcessLifetime < 5*time.Minute {
		t.Errorf("the default decoy lifetime %s is not comfortably longer than a scan", decoyProcessLifetime)
	}
	dir := scratchDir(t)
	record := filepath.Join(dir, "record.json")
	shortLifetime(t, time.Second)
	name := testDecoyName(t)

	if res, err := SeedTargets([]Target{processTarget(name)}, record); err != nil || res.Planted != 1 {
		t.Fatalf("seed = %+v, %v; want 1 planted", res, err)
	}
	rec, err := readRecord(record)
	if err != nil || len(rec.Processes) != 1 {
		t.Fatalf("record = %+v, %v; want one seeded process", rec, err)
	}
	pid := rec.Processes[0].PID
	if !strings.Contains(processCommand(pid), name) {
		t.Fatalf("the decoy is not running under %q as pid %d", name, pid)
	}

	deadline := time.Now().Add(30 * time.Second)
	for strings.Contains(processCommand(pid), name) {
		if time.Now().After(deadline) {
			t.Fatal("the decoy outlived its self-timeout")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// The suspenders must never fire at the wrong target: a recorded pid that some
// unrelated process now holds gets no signal at all, and a second cleanup pass
// is a no-op rather than an error.
func TestCleanupSignalsOnlyTheProcessItSpawned(t *testing.T) {
	dir := scratchDir(t)
	record := filepath.Join(dir, "record.json")

	// Stands in for whatever inherited a recorded pid after the seeded decoy
	// exited — this run did not spawn it, so it must not be touched.
	other := exec.Command("sleep", "30")
	if err := other.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = other.Process.Kill()
		_, _ = other.Process.Wait()
	})

	// A pid that is gone for good: this one has already exited and been reaped.
	dead := exec.Command("sleep", "30")
	if err := dead.Start(); err != nil {
		t.Fatal(err)
	}
	deadPID := dead.Process.Pid
	if err := dead.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if _, err := dead.Process.Wait(); err != nil {
		t.Fatal(err)
	}

	stale := seedRecord{Processes: []seededProcess{
		{PID: other.Process.Pid, Command: "dockerd-" + decoyTag},
		{PID: deadPID, Command: "ssh-agent-" + decoyTag},
	}}
	if err := writeRecord(record, stale); err != nil {
		t.Fatal(err)
	}

	removed, err := CleanupSeeded(record)
	if err != nil {
		t.Fatalf("cleanup of a stale record errored: %v", err)
	}
	if removed != 0 {
		t.Errorf("cleanup removed %d processes from a stale record, want 0", removed)
	}
	if !strings.Contains(processCommand(other.Process.Pid), "sleep") {
		t.Error("cleanup signalled a process it did not spawn")
	}
	if removed, err := CleanupSeeded(record); err != nil || removed != 0 {
		t.Errorf("second cleanup = %d removed, %v; want 0, no error", removed, err)
	}
}

// A decoy that cannot be started is one skipped entry, not a failed run: the
// entries after it — of any kind — are still planted.
func TestProcessSpawnFailureIsSkippedNotFatal(t *testing.T) {
	dir := scratchDir(t)
	record := filepath.Join(dir, "record.json")
	shortLifetime(t, 5*time.Second)
	t.Cleanup(func() { _, _ = CleanupSeeded(record) })
	// Nothing to exec: no decoy can start.
	t.Setenv("PATH", filepath.Join(dir, "no-such-bin"))

	sock := filepath.Join(dir, "after.sock")
	res, err := SeedTargets([]Target{processTarget(testDecoyName(t)), socketTarget(sock)}, record)
	if err != nil {
		t.Fatalf("a decoy that would not start aborted the run: %v", err)
	}
	if res.Planted != 1 || res.Skipped != 1 {
		t.Fatalf("seed = planted %d skipped %d, want 1/1", res.Planted, res.Skipped)
	}
	if !scanned(t, dir, sock) {
		t.Error("the entry after the failing one was not planted")
	}
}

// The round trip the whole capability rests on: a scan before seeding does not
// report the socket, a scan after seeding does, and a scan after cleanup does
// not again.
func TestSeedScanCleanupRoundTrip(t *testing.T) {
	dir := scratchDir(t)
	record := filepath.Join(dir, "record.json")
	// A missing parent dir is the normal case on a bare runner: seeding creates it.
	sock := filepath.Join(dir, "run", "docker.sock")

	if scanned(t, dir, sock) {
		t.Fatal("socket reported before seeding")
	}

	res, err := SeedTargets([]Target{socketTarget(sock)}, record)
	if err != nil {
		t.Fatal(err)
	}
	if res.Planted != 1 || res.Skipped != 0 {
		t.Fatalf("seed = planted %d skipped %d, want 1/0", res.Planted, res.Skipped)
	}
	if !scanned(t, dir, sock) {
		t.Fatal("socket not reported after seeding")
	}

	removed, err := CleanupSeeded(record)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Errorf("cleanup removed %d, want 1", removed)
	}
	if scanned(t, dir, sock) {
		t.Fatal("socket still reported after cleanup")
	}
	if _, err := os.Stat(filepath.Join(dir, "run")); !os.IsNotExist(err) {
		t.Errorf("cleanup left the directory seeding created: %v", err)
	}
}

// Soft-plant rule: what is already there is left exactly as it was, and counts
// as skipped. An entry that cannot be written is skipped too, and the entries
// after it are still planted.
func TestSeedIsSoftAndPerEntryFailuresAreNotFatal(t *testing.T) {
	dir := scratchDir(t)
	record := filepath.Join(dir, "record.json")

	// Something real already owns this path.
	occupied := filepath.Join(dir, "real.sock")
	l, err := net.Listen("unix", occupied)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	before, err := os.Lstat(occupied)
	if err != nil {
		t.Fatal(err)
	}

	// A regular file stands where this target's parent dir would have to go, so
	// the directory can never be created — unwritable even as root.
	blocker := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocker, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	unwritable := filepath.Join(blocker, "x.sock")

	overLong := filepath.Join(dir, strings.Repeat("l", maxUnixSocketPathLen)+".sock")
	fresh := filepath.Join(dir, "fresh.sock")

	res, err := SeedTargets([]Target{
		socketTarget(occupied),
		socketTarget(unwritable),
		socketTarget(overLong),
		socketTarget(fresh),
	}, record)
	if err != nil {
		t.Fatal(err)
	}
	if res.Planted != 1 || res.Skipped != 3 {
		t.Fatalf("seed = planted %d skipped %d, want 1/3", res.Planted, res.Skipped)
	}
	if !scanned(t, dir, fresh) {
		t.Error("the entry after the failing ones was not planted")
	}
	after, err := os.Lstat(occupied)
	if err != nil {
		t.Fatalf("the occupied socket was disturbed: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) || after.Mode() != before.Mode() {
		t.Error("the socket already at the target was not left untouched")
	}
	if _, err := os.Stat(overLong); err == nil {
		t.Error("an over-long path was bound rather than rejected")
	}

	// Cleanup must remove only what this run planted, never the real socket.
	if _, err := CleanupSeeded(record); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(occupied); err != nil {
		t.Errorf("cleanup removed a socket the run did not plant: %v", err)
	}
}

// Cleanup runs after every scan, including after a crashed one — so a second
// run, a record naming an artifact that has gone, and a record whose path is
// now something else must all be no-ops rather than errors or collateral.
func TestCleanupIsIdempotentAndToleratesStaleRecords(t *testing.T) {
	dir := scratchDir(t)
	record := filepath.Join(dir, "record.json")
	sock := filepath.Join(dir, "a.sock")

	if _, err := SeedTargets([]Target{socketTarget(sock)}, record); err != nil {
		t.Fatal(err)
	}
	if _, err := CleanupSeeded(record); err != nil {
		t.Fatal(err)
	}
	// Twice: the record is gone, and so is everything it named.
	if removed, err := CleanupSeeded(record); err != nil || removed != 0 {
		t.Errorf("second cleanup = %d removed, %v; want 0, no error", removed, err)
	}

	// A record left behind by a crashed run: one path since taken by something
	// that is not a socket, one path that no longer exists at all.
	notASocket := filepath.Join(dir, "b.sock")
	if err := os.WriteFile(notASocket, []byte("someone else's"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := seedRecord{
		Sockets: []string{notASocket, filepath.Join(dir, "gone.sock")},
		Dirs:    []string{filepath.Join(dir, "never-created")},
	}
	if err := writeRecord(record, stale); err != nil {
		t.Fatal(err)
	}
	removed, err := CleanupSeeded(record)
	if err != nil {
		t.Fatalf("cleanup of a stale record errored: %v", err)
	}
	if removed != 0 {
		t.Errorf("cleanup removed %d from a stale record, want 0", removed)
	}
	if _, err := os.Stat(notASocket); err != nil {
		t.Errorf("cleanup removed a path that is no longer the socket it planted: %v", err)
	}
}

// The sibling daemon socket is the cross-instance case: it must be found by a
// scan once seeded, and cleanup must take it back out without touching the
// sockets of the sessions that were already there.
func TestSiblingDaemonSocketRoundTripLeavesRealSessionsAlone(t *testing.T) {
	dir := scratchDir(t)
	record := filepath.Join(dir, "record.json")

	// Two sessions already on the box, one of them the one running the probe.
	realSocks := []string{}
	for _, id := range []string{"830dcffe", "0badf00d"} {
		if err := os.Mkdir(filepath.Join(dir, id), 0o755); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, id, "control.sock")
		l, err := net.Listen("unix", p)
		if err != nil {
			t.Fatal(err)
		}
		defer l.Close()
		realSocks = append(realSocks, p)
	}

	sibling := siblingSessionSocket(dir)
	if sibling == "" {
		t.Fatal("no sibling socket derived from a populated daemon dir")
	}
	if slices.Contains(realSocks, sibling) {
		t.Fatalf("sibling socket %q is a real session's", sibling)
	}

	if res, err := SeedTargets([]Target{socketTarget(sibling)}, record); err != nil || res.Planted != 1 {
		t.Fatalf("seed = %+v, %v; want 1 planted", res, err)
	}
	if !scanned(t, dir, sibling) {
		t.Error("the sibling socket is not reported by a scan")
	}
	if _, err := CleanupSeeded(record); err != nil {
		t.Fatal(err)
	}
	if scanned(t, dir, sibling) {
		t.Error("the sibling socket survived cleanup")
	}
	for _, p := range realSocks {
		if _, err := os.Lstat(p); err != nil {
			t.Errorf("a real session's socket %q was removed: %v", p, err)
		}
	}
}
