package tasks

import (
	"slices"
	"strings"
	"testing"
)

// The registry is the single source of truth: every check has a Target, and
// nothing gets invented or dropped in translation.
func TestListTargetsMirrorsRegistry(t *testing.T) {
	home := "/home/tester"
	if got, want := len(listTargetsForHome(home)), len(buildSensitivePathsForHome(home)); got != want {
		t.Fatalf("target count %d != registry count %d", got, want)
	}
}

// Safety invariant the seeder relies on: a seedable target is always a
// home-scoped regular file, never a directory and never outside $HOME. If this
// breaks, the seeder could try to clobber a system path or a directory.
func TestSeedableTargetsAreHomeFilesOnly(t *testing.T) {
	home := "/home/tester"
	seedable := 0
	for _, tg := range listTargetsForHome(home) {
		if !tg.Seedable {
			continue
		}
		seedable++
		if tg.Kind != "file" {
			t.Errorf("seedable target %q is not a file (kind=%s)", tg.Path, tg.Kind)
		}
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
	for _, tg := range listTargetsForHome("/home/tester") {
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
	all := listTargetsForHome(home)
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
	for _, tg := range listTargetsForHome(home) {
		got[tg.Path] = tg.Seedable
	}
	for path, exp := range want {
		if got[path] != exp {
			t.Errorf("seedable(%q) = %v, want %v", path, got[path], exp)
		}
	}
}
