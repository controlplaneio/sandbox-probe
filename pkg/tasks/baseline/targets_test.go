package tasks

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The registry is the single source of truth: every check has a Target, and
// nothing gets invented or dropped in translation.
func TestListTargetsMirrorsRegistry(t *testing.T) {
	home := "/home/tester"
	files := 0
	for _, tg := range listTargetsForHome(home, "") {
		if tg.Kind == "file" || tg.Kind == "dir" {
			files++
		}
	}
	if want := len(buildSensitivePathsForHome(home)); files != want {
		t.Fatalf("target count %d != registry count %d", files, want)
	}
}

// Safety invariant the seeder relies on: a seedable *file* target is always a
// home-scoped regular file, never a directory and never outside $HOME. If this
// breaks, the seeder could try to clobber a system path or a directory. Socket
// targets are a catalogue of conventional IPC paths, most of them system-scoped
// by nature, and carry their own invariants below.
func TestSeedableTargetsAreHomeFilesOnly(t *testing.T) {
	home := "/home/tester"
	seedable := 0
	for _, tg := range listTargetsForHome(home, "") {
		if !tg.Seedable || tg.Kind != "file" {
			continue
		}
		seedable++
		if tg.Scope != "home" || !strings.HasPrefix(tg.Path, home+"/") {
			t.Errorf("seedable target %q escapes home %q", tg.Path, home)
		}
		if strings.Contains(tg.Path, "..") {
			t.Errorf("seedable target %q contains a parent-dir escape", tg.Path)
		}
	}
	if seedable == 0 {
		t.Fatal("expected at least one seedable target")
	}
}

// Every entry's kind comes from the fixed set, and every IPC entry (socket /
// pipe / process) says what tool class it stands in for and how strong the
// evidence for it is — so a catalogue addition cannot land with an invented
// vocabulary, and the seeder can dispatch on kind.
func TestTargetKindAndProvenanceVocabularies(t *testing.T) {
	for _, tg := range listTargetsForHome("/home/tester", "/private/tmp/cc-daemon-1/decoy/control.sock") {
		if !targetKinds[tg.Kind] {
			t.Errorf("target %q has unrecognised kind %q", tg.Path, tg.Kind)
		}
		if tg.Category != "" && !targetCategories[tg.Category] {
			t.Errorf("target %q has unrecognised category %q", tg.Path, tg.Category)
		}
		if tg.Evidence != "" && !evidenceTiers[tg.Evidence] {
			t.Errorf("target %q has unrecognised evidence tier %q", tg.Path, tg.Evidence)
		}
		if tg.Kind == "file" || tg.Kind == "dir" {
			continue // the probe's own check list, not a tool catalogue
		}
		if tg.Category == "" || tg.Evidence == "" {
			t.Errorf("%s target %q must carry a category and an evidence tier (got %q / %q)",
				tg.Kind, tg.Path, tg.Category, tg.Evidence)
		}
	}
}

// The listing is OS-scoped, so the seeder is never handed a Windows pipe on
// Linux.
func TestTargetsForOSOmitsOtherOperatingSystems(t *testing.T) {
	all := []Target{
		{Path: "/everywhere"},
		{Path: `\\.\pipe\thing`, OS: []string{"windows"}},
		{Path: "/run/thing.sock", OS: []string{"linux", "darwin"}},
	}
	want := map[string][]string{
		"windows": {"/everywhere", `\\.\pipe\thing`},
		"linux":   {"/everywhere", "/run/thing.sock"},
		"darwin":  {"/everywhere", "/run/thing.sock"},
	}
	for goos, paths := range want {
		got := []string{}
		for _, tg := range targetsForOS(all, goos) {
			got = append(got, tg.Path)
		}
		if !slices.Equal(got, paths) {
			t.Errorf("targetsForOS(%s) = %v, want %v", goos, got, paths)
		}
	}
}

// The OS filter must not disturb what was already emitted: the sensitive-file
// registry and its seedable flags stay identical on every operating system.
func TestFileRegistryUnchangedOnEveryOS(t *testing.T) {
	home := "/home/tester"
	all := []Target{}
	for _, tg := range listTargetsForHome(home, "") {
		if tg.Kind == "file" || tg.Kind == "dir" {
			all = append(all, tg) // the IPC catalogue is OS-scoped on purpose
		}
	}
	same := func(a, b Target) bool {
		return a.Path == b.Path && a.Kind == b.Kind && a.Scope == b.Scope && a.Seedable == b.Seedable
	}
	for _, goos := range []string{"linux", "darwin", "windows"} {
		if got := targetsForOS(all, goos); !slices.EqualFunc(got, all, same) {
			t.Errorf("registry on %s has %d targets, want the full %d unchanged", goos, len(got), len(all))
		}
	}
}

// A known credential file must be seedable, and a known directory / system path
// must not — guards the classification itself, not just its shape.
func TestSeedableClassificationSpotChecks(t *testing.T) {
	home := "/home/tester"
	want := map[string]bool{
		home + "/.aws/credentials": true,
		home + "/.ssh/id_rsa":      true,
		home + "/.npmrc":           true,
		home + "/.gnupg":           false, // directory
		home + "/.config/gcloud":   false, // directory
		"/etc/shadow":              false, // system
		home + "/.gitconfig":       false, // content-predicate: a decoy corrupts git and never matches the predicate
	}
	got := map[string]bool{}
	for _, tg := range listTargetsForHome(home, "") {
		got[tg.Path] = tg.Seedable
	}
	for path, exp := range want {
		if got[path] != exp {
			t.Errorf("seedable(%q) = %v, want %v", path, got[path], exp)
		}
	}
}

// A socket path over the AF_UNIX sun_path limit fails at bind() time with a
// cryptic error, so the registry rejects it here instead, naming the entry.
func TestSocketTargetsWithinUnixPathLimit(t *testing.T) {
	for _, tg := range listTargetsForHome("/home/tester", "/private/tmp/cc-daemon-1/decoy/control.sock") {
		if tg.Kind != "socket" {
			continue
		}
		if len(tg.Path) > maxUnixSocketPathLen {
			t.Errorf("socket target %q is %d bytes, over the %d-byte AF_UNIX limit",
				tg.Path, len(tg.Path), maxUnixSocketPathLen)
		}
	}
}

// Seeding and detection must not drift: a decoy planted somewhere the probe
// never looks is inert. Every seedable file target is one of the probe's own
// sensitive-path checks, and every seedable socket target sits inside a scanned
// socket root — which is what pulls Linux's editor-ipc socket, outside every
// FHS runtime dir, into a root of its own.
func TestEverySeedableTargetIsScanned(t *testing.T) {
	home := "/home/tester"
	t.Setenv("TMPDIR", "/tmp/probe-tmpdir")
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")

	checked := map[string]bool{}
	for _, sp := range buildSensitivePathsForHome(home) {
		checked[sp.path] = true
	}
	roots := socketRootsForHome(home)

	sockets, processes := 0, 0
	for _, tg := range listTargetsForHome(home, "/private/tmp/cc-daemon-1/decoy/control.sock") {
		if !tg.Seedable {
			continue
		}
		switch tg.Kind {
		case "file":
			if !checked[tg.Path] {
				t.Errorf("seedable file target %q is not one of the paths the probe checks", tg.Path)
			}
		case "socket":
			sockets++
			if !underAnyRoot(tg.Path, roots) {
				t.Errorf("seedable socket target %q is under none of the scanned socket roots %v", tg.Path, roots)
			}
		case "process":
			processes++
			// The process scan enumerates everything running, so the coupling here is
			// the other way round: the decoy is only emitted where that scan exists,
			// and carries the marker that tells it from the daemon it stands in for —
			// which is what cleanup verifies a recorded pid against.
			if !strings.Contains(tg.Path, decoyTag) {
				t.Errorf("seedable process target %q carries no decoy marker, so cleanup cannot tell it from a real process", tg.Path)
			}
			if !slices.Equal(tg.OS, []string{"linux"}) {
				t.Errorf("process target %q is emitted on %v, where the probe has no process scan", tg.Path, tg.OS)
			}
		default:
			t.Errorf("target %q has seedable kind %q with no scan to back it", tg.Path, tg.Kind)
		}
	}
	if sockets == 0 {
		t.Fatal("expected at least one seedable socket target")
	}
	if processes == 0 {
		t.Fatal("expected at least one seedable process target")
	}
	if !underAnyRoot(home+"/.local/share/code-server/code-server-ipc.sock", roots) {
		t.Error("the Linux editor-ipc socket location is outside every scanned socket root")
	}
}

func underAnyRoot(path string, roots []string) bool {
	for _, r := range roots {
		if strings.HasPrefix(path, r+"/") {
			return true
		}
	}
	return false
}

// The catalogue carries a sibling session's daemon socket, so cross-instance
// reach is measured rather than assumed — and carries it only when a sibling
// identifier could be generated at all.
func TestSiblingAgentSocketIsCatalogued(t *testing.T) {
	const sibling = "/private/tmp/cc-daemon-1/decoy/control.sock"
	var got *Target
	for _, tg := range socketTargets("/home/tester", sibling) {
		if tg.Path == sibling {
			got = &tg
		}
	}
	if got == nil {
		t.Fatal("no catalogue entry for the sibling agent daemon socket")
	}
	if got.Kind != "socket" || got.Category != "agent-ipc" || got.Evidence == "" {
		t.Errorf("sibling entry = kind %q category %q evidence %q, want a socket/agent-ipc entry with an evidence tier",
			got.Kind, got.Category, got.Evidence)
	}
	for _, tg := range socketTargets("/home/tester", "") {
		if tg.Category == "agent-ipc" {
			t.Errorf("sibling entry %q seeded although the running session could not be identified", tg.Path)
		}
	}
}

// The whole point of the sibling socket is that it is *not* this session's, so
// the generated identifier can never be one that already exists — otherwise the
// probe measures itself, or shadows a real session's socket.
func TestSiblingSessionIDNeverCollides(t *testing.T) {
	first := distinctID("seed", nil)
	if got := distinctID("seed", []string{first}); got == first {
		t.Errorf("distinctID returned the taken identifier %q", got)
	}
	// Every identifier a real daemon dir could hold is taken: the derivation must
	// still land outside that set.
	taken := []string{first, distinctID("seed", []string{first})}
	if got := distinctID("seed", taken); slices.Contains(taken, got) {
		t.Errorf("distinctID(%v) = %q, which is one of the running sessions", taken, got)
	}

	dir := t.TempDir()
	if got := siblingSessionSocket(dir); got != "" {
		t.Errorf("siblingSessionSocket with no session present = %q, want it skipped", got)
	}
	if got := siblingSessionSocket(filepath.Join(dir, "absent")); got != "" {
		t.Errorf("siblingSessionSocket with no daemon dir = %q, want it skipped", got)
	}
	running := []string{"830dcffe", "0badf00d"}
	for _, s := range running {
		if err := os.Mkdir(filepath.Join(dir, s), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got := siblingSessionSocket(dir)
	if got == "" {
		t.Fatal("siblingSessionSocket returned nothing with sessions present")
	}
	id := filepath.Base(filepath.Dir(got))
	if slices.Contains(running, id) {
		t.Errorf("sibling identifier %q is a running session's", id)
	}
}
