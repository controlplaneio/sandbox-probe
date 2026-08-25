package tasks

import (
	"errors"
	"os"
	"strconv"
)

// errNoPipeNamespace is what the seeding half of the pipe seam reports off Windows. Declared
// here rather than in the !windows file because portable code needs to recognise it: seeding
// the reachability decoy must tell "there is no pipe namespace on this OS" apart from "the
// plant failed", and only the second is a skip worth counting.
var errNoPipeNamespace = errors.New("named pipes are Windows-only")

// ReachPipeName is the ONLY pipe this probe ever opens as a client, and it is deliberately
// NOT a catalogue entry.
//
// Opening a real service's pipe is not passive: it consumes a server instance, delivers a
// connection event, and can hang a badly written server. So reachability is measured only
// against a name no real service can hold.
//
// Why not the catalogue names, which are what an attacker would actually target. seedPipe
// plants a decoy at one of those only when it is free, so at SCAN time `\\.\pipe\docker_engine`
// is either our decoy or a live Docker Desktop — and the scan is a different process from the
// seeder, with no way to tell which. Making it tell would mean reading the seed record, which
// puts a SAFETY property behind a filesystem read the sandbox under test may block and whose
// TMPDIR may differ. That is exactly backwards. Whether the catalogue names are visible is
// already answered by named_pipe_detection; what reachability adds is whether an open succeeds,
// and that question can be asked of any pipe — so ask it of the one where the answer costs
// nobody anything.
//
// A name of this shape can only ever be ours: FILE_FLAG_FIRST_PIPE_INSTANCE means the seeder
// holds it or nobody does. It is fixed rather than per-run for the same reason the record is
// not consulted — seeder and scan share no state, and the only channel between them is that
// record. Two concurrent probe runs collide benignly: the second seeder's plant is skipped and
// the scan measures the first run's decoy, which is still the probe's own.
const ReachPipeName = pipePrefix + "sandbox-probe-reachability-decoy"

// reachControlPipeName is a name nothing ever serves: the calibration control, the same
// instrument as calibrate()'s known-free loopback port. It says what "absent" looks like on
// THIS host, so a not-found against the decoy is interpretable rather than merely
// disappointing. Per-pid, so two concurrent runs cannot make each other's control real.
func reachControlPipeName() string {
	return pipePrefix + "sandbox-probe-control-" + strconv.Itoa(os.Getpid())
}

// PipeReach is the named-pipe reachability measurement. Same split as LocalServices: what was
// REACHED is scored, how the measurement went is not.
//
// Portable on purpose. The accessor rules below are the `absent != empty` contract, and they
// are the part most worth testing on every OS — a Linux-only contributor must be able to break
// them and see it.
type PipeReach struct {
	Reached []string
	Created string // the name creation actually made, "" if it did not
	Status  map[string]any
}

// Names returns the pipes this process reached, and whether the measurement concluded at all.
//
// A false second return means the caller must emit no finding. The site reads an absent finding
// as unmeasured and an empty one as a measured negative, so publishing [] here would claim a
// sandbox blocked something that was never planted — which is the false-blocked class of bug
// this whole measurement layer was rebuilt to remove.
//
// The gate is enumeration. A decoy the scan can SEE was planted, so failing to open it is a fact
// about policy. A decoy it cannot see is either a host nobody seeded or a namespace that hides
// everything, and those two are not separable from in here — named_pipe_detection's own count
// already carries that story, so nothing is claimed twice.
func (p PipeReach) Names() ([]string, bool) {
	if p.Status["decoy_enumerated"] != true {
		return nil, false
	}
	return p.Reached, true
}

// CreatedPipe is the creation half, shaped as a list to match the enumeration finding.
//
// Absence of the FINDING means the capability was denied, so this returns a present-but-empty
// list for a denial and reports unmeasured for anything else. The distinction matters: a
// creation that failed because the probe built a bad name (OutcomeProbeError) says nothing
// whatever about the sandbox, and emitting no finding for it would publish a denial the
// sandbox never made — the false-blocked class this layer exists to remove. reach.go says the
// same thing of that outcome: "never a statement about the sandbox".
//
// The reason a denial happened lives in Status, not here.
func (p PipeReach) CreatedPipe() ([]string, bool) {
	if p.Created != "" {
		return []string{p.Created}, true
	}
	// Unmeasured: off Windows (no Status at all), or the probe itself failed.
	if s, ok := p.Status["creation"].(string); !ok || s == string(OutcomeProbeError) {
		return nil, false
	}
	return []string{}, true
}
