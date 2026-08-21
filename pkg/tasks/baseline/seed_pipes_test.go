package tasks

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestMain doubles as the decoy pipe server. seedPipe spawns os.Executable()
// with `serve-pipe <name> <lifetime>` — the probe binary in production, this
// test binary here — so the round trip below exercises the real spawn, the real
// server and the real termination rather than a stand-in for them.
func TestMain(m *testing.M) {
	if len(os.Args) == 4 && os.Args[1] == "serve-pipe" {
		d, err := time.ParseDuration(os.Args[3])
		if err == nil {
			err = ServePipe(os.Args[2], d)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// windowsOnly skips everything below off Windows: no other OS has a pipe
// namespace to plant in.
func windowsOnly(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("named pipes are Windows-only")
	}
}

// testPipeName is a name this test invented — never one of the catalogue's, so
// a test can never serve, shadow or remove a pipe a real tool answers on.
func testPipeName(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf(`\\.\pipe\sandbox-probe-test-decoy-%d-%s`, os.Getpid(), strings.NewReplacer("/", "-", " ", "-").Replace(t.Name()))
}

func pipeTarget(name string) Target {
	return Target{Path: name, Kind: "pipe", Seedable: true, Category: "credential-agent", Evidence: "empirical-own-machine"}
}

// pipeScanned reports whether the probe's own pipe enumeration — the one behind
// named_pipe_detection — sees name.
func pipeScanned(t *testing.T, name string) bool {
	t.Helper()
	pipes, err := ListNamedPipes()
	if err != nil {
		t.Fatal(err)
	}
	return slices.ContainsFunc(pipes, func(p string) bool { return strings.EqualFold(p, name) })
}

// waitForPipe waits for name to appear or disappear, so an assertion never
// races a process that is starting or being terminated.
func waitForPipe(t *testing.T, name string, want bool) bool {
	t.Helper()
	for deadline := time.Now().Add(20 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if pipeScanned(t, name) == want {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
	}
}

// The round trip the Windows half of the capability rests on: the probe's own
// pipe scan does not report the decoy before seeding, does after it, and does
// not again once cleanup has taken the server down.
func TestSeedScanCleanupPipeRoundTrip(t *testing.T) {
	windowsOnly(t)
	record := filepath.Join(t.TempDir(), "record.json")
	name := testPipeName(t)
	shortLifetime(t, 60*time.Second)
	t.Cleanup(func() { _, _ = CleanupSeeded(record) })

	if pipeScanned(t, name) {
		t.Fatal("the decoy pipe is reported before seeding")
	}

	res, err := SeedTargets([]Target{pipeTarget(name)}, record)
	if err != nil {
		t.Fatal(err)
	}
	if res.Planted != 1 || res.Skipped != 0 {
		t.Fatalf("seed = planted %d skipped %d, want 1/0", res.Planted, res.Skipped)
	}
	if !pipeScanned(t, name) {
		t.Fatal("the decoy pipe is not reported after seeding")
	}

	removed, err := CleanupSeeded(record)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Errorf("cleanup removed %d, want 1", removed)
	}
	if pipeScanned(t, name) {
		t.Fatal("the decoy pipe is still reported after cleanup")
	}
	// Cleanup runs after every scan, including after a crashed one: a second pass
	// over a record whose artifacts are gone is a no-op, not an error.
	if removed, err := CleanupSeeded(record); err != nil || removed != 0 {
		t.Errorf("second cleanup = %d removed, %v; want 0, no error", removed, err)
	}
}

// The belt: a decoy pipe's server dies on its own, so a cleanup that never runs
// — a crashed scan, a machine losing power — is not a permanent leak. No
// cleanup step runs in this test at all.
func TestSeededPipeExpiresWithNoCleanup(t *testing.T) {
	if decoyProcessLifetime < 5*time.Minute {
		t.Errorf("the default decoy lifetime %s is not comfortably longer than a scan", decoyProcessLifetime)
	}
	windowsOnly(t)
	record := filepath.Join(t.TempDir(), "record.json")
	name := testPipeName(t)
	// Long enough to outlast the scan below, short enough that the test can wait
	// it out — the same trade the real fixed timeout makes at scan scale.
	shortLifetime(t, 5*time.Second)

	if res, err := SeedTargets([]Target{pipeTarget(name)}, record); err != nil || res.Planted != 1 {
		t.Fatalf("seed = %+v, %v; want 1 planted", res, err)
	}
	if !pipeScanned(t, name) {
		t.Fatal("the decoy pipe is not reported after seeding")
	}
	if !waitForPipe(t, name, false) {
		t.Fatal("the decoy pipe outlived its server's self-timeout")
	}
}

// Soft-plant rule for pipes: a name something already serves is left serving
// and counted as skipped, so a real docker_engine is never shadowed. The
// entries after it are still planted.
func TestSeedPipeSkipsAnOccupiedName(t *testing.T) {
	windowsOnly(t)
	record := filepath.Join(t.TempDir(), "record.json")
	shortLifetime(t, 60*time.Second)
	t.Cleanup(func() { _, _ = CleanupSeeded(record) })

	// Stands in for the real service already holding the name.
	occupied := testPipeName(t) + "-occupied"
	served := make(chan error, 1)
	go func() { served <- ServePipe(occupied, 30*time.Second) }()
	if !waitForPipe(t, occupied, true) {
		t.Fatal("the stand-in server never came up")
	}

	fresh := testPipeName(t) + "-fresh"
	res, err := SeedTargets([]Target{pipeTarget(occupied), pipeTarget(fresh)}, record)
	if err != nil {
		t.Fatalf("an occupied name aborted the run: %v", err)
	}
	if res.Planted != 1 || res.Skipped != 1 {
		t.Fatalf("seed = planted %d skipped %d, want 1/1", res.Planted, res.Skipped)
	}
	if !pipeScanned(t, occupied) {
		t.Error("the pipe that was already being served is gone")
	}
	if !pipeScanned(t, fresh) {
		t.Error("the entry after the occupied one was not planted")
	}

	rec, err := readRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range rec.Pipes {
		if strings.EqualFold(p.Name, occupied) {
			t.Fatal("a pipe the run did not plant was recorded, so cleanup would take down a real server")
		}
	}
	select {
	case err := <-served:
		t.Fatalf("the stand-in server exited during seeding: %v", err)
	default:
	}
}

// The suspenders must never fire at the wrong target: a recorded pid that some
// unrelated process now holds gets terminated by nothing. The pid here is this
// test binary's own — if the identity check regresses, the run dies rather than
// reporting a pass.
func TestCleanupTerminatesOnlyThePipeServerItSpawned(t *testing.T) {
	windowsOnly(t)
	record := filepath.Join(t.TempDir(), "record.json")
	created, err := processCreated(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	stale := seedRecord{Pipes: []seededPipe{
		{Name: `\\.\pipe\docker_engine`, PID: os.Getpid(), Created: created + 1}, // pid reused since
		{Name: `\\.\pipe\openssh-ssh-agent`, PID: 0x7FFFFFF0, Created: created},  // long gone
	}}
	if err := writeRecord(record, stale); err != nil {
		t.Fatal(err)
	}
	removed, err := CleanupSeeded(record)
	if err != nil {
		t.Fatalf("cleanup of a stale record errored: %v", err)
	}
	if removed != 0 {
		t.Errorf("cleanup terminated %d processes from a stale record, want 0", removed)
	}
}
