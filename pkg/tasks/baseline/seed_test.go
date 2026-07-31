package tasks

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
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
