package tasks

import (
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
