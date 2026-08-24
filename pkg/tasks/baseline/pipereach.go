package tasks

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
// Absence is the finding contract's "blocked" — a present finding saying false would invert it —
// so the reason a creation failed lives in Status, not here.
func (p PipeReach) CreatedPipe() ([]string, bool) {
	if p.Created == "" {
		return nil, false
	}
	return []string{p.Created}, true
}
