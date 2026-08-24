package tasks

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

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
	Sockets   []string        `json:"sockets"`
	Processes []seededProcess `json:"processes"`
	Pipes     []seededPipe    `json:"pipes"`
	Dirs      []string        `json:"dirs"` // parent dirs seeding had to create, outermost first
}

// seededPipe is a decoy Windows pipe's recorded identity: the name it serves,
// and the server process holding it open. Creation time is the part of a pid
// Windows never reuses, so cleanup checks it before terminating anything.
type seededPipe struct {
	Name    string `json:"name"`
	PID     int    `json:"pid"`
	Created int64  `json:"created"`
}

// seededProcess is a live decoy's recorded identity. The command name is what
// cleanup checks the pid still holds before signalling it, so a pid reused
// since the record was written can never cost an unrelated process.
type seededProcess struct {
	PID     int    `json:"pid"`
	Command string `json:"command"`
}

// decoyProcessLifetime is the belt of the live-artifact lifecycle: a seeded
// process exits on its own after this long, comfortably longer than a scan, so
// a cleanup that never runs — a crashed run, a machine losing power — is not a
// permanent leak. The suspenders are CleanupSeeded, which is the normal path. A
// var, not a const, only so tests need not wait it out.
var decoyProcessLifetime = 10 * time.Minute

// DefaultSeedRecordPath is where a seeding pass records what it created, for
// the cleanup pass that follows the scan.
func DefaultSeedRecordPath() string {
	return filepath.Join(os.TempDir(), "sandbox-probe-seed.json")
}

// errOccupied is the soft-plant rule firing: something already owns the target.
var errOccupied = errors.New("target already exists")

// SeedTargets soft-plants a decoy at every seedable socket, pipe and process target,
// recording what it created at recordPath so cleanup can remove exactly that.
// Soft in every case: a target something already owns is left untouched and
// counted as skipped, never shadowed, and no process the seeder did not start
// is ever adopted. A per-entry failure (unwritable path, over-long path, a
// decoy that would not start) is skipped too, not fatal — one bad entry must
// not cost the rest.
func SeedTargets(targets []Target, recordPath string) (SeedResult, error) {
	// A record from a crashed run is still ours to clean up: merge, don't drop.
	rec, _ := readRecord(recordPath)
	var res SeedResult
	for _, t := range targets {
		if !t.Seedable {
			continue
		}
		switch t.Kind {
		case "socket":
			dirs, err := seedSocket(t.Path)
			rec.Dirs = append(rec.Dirs, dirs...)
			if err != nil {
				log.Debug().Err(err).Str("path", t.Path).Msg("Socket decoy skipped")
				res.Skipped++
				continue
			}
			rec.Sockets = append(rec.Sockets, t.Path)
		case "process":
			pid, err := seedProcess(t.Path)
			if err != nil {
				log.Debug().Err(err).Str("command", t.Path).Msg("Process decoy skipped")
				res.Skipped++
				continue
			}
			rec.Processes = append(rec.Processes, seededProcess{PID: pid, Command: t.Path})
		case "pipe":
			p, err := seedPipe(t.Path)
			if err != nil {
				log.Debug().Err(err).Str("pipe", t.Path).Msg("Named pipe decoy skipped")
				res.Skipped++
				continue
			}
			rec.Pipes = append(rec.Pipes, p)
		default:
			continue
		}
		res.Planted++
	}

	// The reachability decoy, which is NOT a catalogue entry and must not become one. It
	// stands in for no real tool, and that is the point: reachability is measured only
	// against a name no real service can hold, because opening a real service's pipe is not
	// passive. Same precedent as calibrate()'s control port — an instrument the probe plants
	// for itself. See ReachPipeName.
	//
	// Off Windows seedPipe reports errNoPipeNamespace, which is not a skip: there is nothing
	// there to skip.
	if p, err := seedPipe(ReachPipeName); err == nil {
		rec.Pipes = append(rec.Pipes, p)
		res.Planted++
	} else if !errors.Is(err, errNoPipeNamespace) {
		log.Debug().Err(err).Str("pipe", ReachPipeName).Msg("Reachability decoy skipped")
		res.Skipped++
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

// seedProcess starts a decoy under the target's distinctive command name and
// returns its pid. Nothing already running is adopted: unlike a path, a process
// is not somewhere a decoy can be planted on top of something else, so the soft
// rule here is that the seeder only ever counts — and only ever signals — a
// process it started itself.
//
// The decoy is a plain sleep under a replaced argv0, which is what the process
// scan reports, and which gives the self-timeout for free.
func seedProcess(name string) (int, error) {
	cmd := exec.Command("sleep", strconv.FormatFloat(decoyProcessLifetime.Seconds(), 'f', -1, 64))
	cmd.Args[0] = name
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	return cmd.Process.Pid, nil
}

// processCommand returns the command line pid is running now, or "" if it is
// gone or cannot be read. ps is the one lookup that works the same on every
// POSIX host without a build tag — the procfs reader beside it is Linux-only —
// and a failure here is read as "not ours", so it can only ever cost a signal
// that was not sent.
func processCommand(pid int) string {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "args=").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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
// longer a socket, and a recorded pid that no longer holds the command name it
// was seeded under, are both left alone — by then they belong to something
// else, so no signal is sent.
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
	for _, p := range rec.Processes {
		if !strings.Contains(processCommand(p.PID), p.Command) {
			log.Debug().Int("pid", p.PID).Str("command", p.Command).
				Msg("Recorded process decoy is gone or is no longer ours; sending no signal")
			continue
		}
		proc, err := os.FindProcess(p.PID)
		if err != nil {
			continue
		}
		if err := proc.Kill(); err != nil {
			continue
		}
		// Reaps the decoy when this process is the one that spawned it; harmless
		// when the seeding run has since exited and init owns it.
		_, _ = proc.Wait()
		removed++
	}
	for _, p := range rec.Pipes {
		// The pipe goes when its server does, so there is nothing else to remove.
		if removePipeDecoy(p) {
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
