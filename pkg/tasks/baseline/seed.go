package tasks

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"

	"github.com/rs/zerolog/log"
)

// SeedResult counts one seeding pass: what was planted, and what was left alone
// because something was already there or the path could not be written.
type SeedResult struct {
	Planted int
	Skipped int
}

// seedRecord is what a run planted. Cleanup acts only on this, so it can never
// remove an artifact the probe did not create.
type seedRecord struct {
	Sockets []string `json:"sockets"`
	Dirs    []string `json:"dirs"` // parent dirs seeding had to create, outermost first
}

// DefaultSeedRecordPath is where a seeding pass records what it created, for
// the cleanup pass that follows the scan.
func DefaultSeedRecordPath() string {
	return filepath.Join(os.TempDir(), "sandbox-probe-seed.json")
}

// errOccupied is the soft-plant rule firing: something already owns the target.
var errOccupied = errors.New("target already exists")

// SeedTargets soft-plants a decoy at every seedable socket target, recording
// what it created at recordPath so cleanup can remove exactly that. Soft in
// every case: a target something already owns is left untouched and counted as
// skipped, never shadowed. A per-entry failure (unwritable path, over-long
// path) is skipped too, not fatal — one bad entry must not cost the rest.
func SeedTargets(targets []Target, recordPath string) (SeedResult, error) {
	// A record from a crashed run is still ours to clean up: merge, don't drop.
	rec, _ := readRecord(recordPath)
	var res SeedResult
	for _, t := range targets {
		if !t.Seedable || t.Kind != "socket" {
			continue
		}
		dirs, err := seedSocket(t.Path)
		rec.Dirs = append(rec.Dirs, dirs...)
		if err != nil {
			log.Debug().Err(err).Str("path", t.Path).Msg("Socket decoy skipped")
			res.Skipped++
			continue
		}
		rec.Sockets = append(rec.Sockets, t.Path)
		res.Planted++
	}
	return res, writeRecord(recordPath, rec)
}

// seedSocket binds and immediately closes a Unix socket at path, leaving the
// socket-typed file behind — no listener has to stay alive, since the scan only
// stat()s for socket-typed entries and never connects. Returns the parent dirs
// it had to create, so cleanup can remove those too.
func seedSocket(path string) ([]string, error) {
	if len(path) > maxUnixSocketPathLen {
		return nil, fmt.Errorf("path is %d bytes, over the %d-byte AF_UNIX limit", len(path), maxUnixSocketPathLen)
	}
	if _, err := os.Lstat(path); err == nil {
		return nil, errOccupied // never shadow the socket a real tool already owns
	}
	dirs, err := makeDirs(filepath.Dir(path))
	if err != nil {
		return dirs, err
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		return dirs, err
	}
	// Go unlinks the socket file on Close; the decoy has to outlive the seeding
	// process because the scan runs after it.
	if ul, ok := l.(*net.UnixListener); ok {
		ul.SetUnlinkOnClose(false)
	}
	return dirs, l.Close()
}

// makeDirs creates dir and every missing parent, returning what it created
// (outermost first) so cleanup removes exactly what seeding added and nothing
// that was already there.
func makeDirs(dir string) ([]string, error) {
	var missing []string
	for d := dir; ; {
		if _, err := os.Stat(d); err == nil {
			break
		}
		missing = append(missing, d)
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	slices.Reverse(missing)
	for _, d := range missing {
		if err := os.Mkdir(d, 0o755); err != nil {
			return missing, err
		}
	}
	return missing, nil
}

// CleanupSeeded removes every artifact recorded at recordPath, then the record
// itself, and reports how many artifacts it removed. It is idempotent: a
// missing record, an artifact already gone, or a record left behind by a
// crashed run are all no-ops rather than errors. A recorded path that is no
// longer a socket is left alone — by then it belongs to something else.
func CleanupSeeded(recordPath string) (int, error) {
	rec, err := readRecord(recordPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	removed := 0
	for _, p := range rec.Sockets {
		fi, err := os.Lstat(p)
		if err != nil {
			continue // already gone
		}
		if fi.Mode()&os.ModeSocket == 0 {
			log.Warn().Str("path", p).Msg("Recorded decoy is no longer a socket; leaving it alone")
			continue
		}
		if err := os.Remove(p); err == nil {
			removed++
		}
	}
	// Innermost first, and os.Remove only takes an empty dir — so a dir that
	// gained real content since seeding survives.
	for i := len(rec.Dirs) - 1; i >= 0; i-- {
		_ = os.Remove(rec.Dirs[i])
	}
	if err := os.Remove(recordPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return removed, err
	}
	return removed, nil
}

func readRecord(path string) (seedRecord, error) {
	var rec seedRecord
	b, err := os.ReadFile(path)
	if err != nil {
		return rec, err
	}
	return rec, json.Unmarshal(b, &rec)
}

func writeRecord(path string, rec seedRecord) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
