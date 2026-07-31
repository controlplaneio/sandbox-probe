Type: task
Status: resolved

## Question

Not a decision — concrete, already-decided implementation work that
blocks [ticket 04](04-repo-split-scope.md)'s dependency design from
actually functioning (a `go.mod` pin to `sandbox-probe` releases needs
real, working releases to pin to).

Two parts, both confirmed necessary during grilling:

1. **Fix the darwin/amd64 build failure.** Both existing Release runs
   (`v1.0.0`, `v1.1.0`) failed identically: `pkg/tasks/baseline/environment.go:105:5:
   undefined: isSeatbelt`, target `darwin_amd64_v1`. Root cause:
   `seatbelt_darwin.go` uses cgo (calls Apple's private `sandbox_check` via
   a C wrapper — confirmed fine to keep, see map Notes); `seatbelt.go`'s
   fallback is gated `//go:build !darwin`. When cross-compiling
   `darwin/amd64` without a working C toolchain for that target, cgo files
   get silently dropped and neither file provides `isSeatbelt`. Fix
   options: get a working cross-C-toolchain path for `darwin/amd64` in the
   release runner, or drop `darwin/amd64` from `.goreleaser.yml`'s build
   matrix if arm64-only is acceptable for macOS releases now (worth
   confirming with Chris which, but leaning toward whichever is less
   fragile long-term).
2. **Switch the release trigger** from manually-pushed
   `v[0-9]+.[0-9]+.[0-9]+` tags to push-to-`main` + `lukaszraczylo/semver-generator`
   (same `semver.yaml` keyword config as `chrisns/MacWhisperAuto`), so
   releases stop depending on someone remembering to tag.

## Answer

Fixed on branch `fix/release-pipeline` (local only — not pushed, no PR
opened, waiting on Chris's review). Three files changed:
`.github/workflows/release.yml`, `.goreleaser.yml`, new `semver.yaml`
(verbatim from `chrisns/MacWhisperAuto`).

**Root cause was more precise than this ticket's original framing** —
it wasn't a missing cross-C-toolchain. `grep -rn 'import "C"'` confirms
only `seatbelt_darwin.go` uses cgo anywhere in the repo; direct
`GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build` on real Apple Silicon
hardware (Xcode clang) cross-compiles fine on its own. The actual bug:
**goreleaser itself forces `CGO_ENABLED=0` for cross-compiled targets by
default, regardless of the ambient shell's `CGO_ENABLED`** — confirmed by
reproducing the exact CI failure locally with `goreleaser build --snapshot`
even with `CGO_ENABLED=1` exported. Fixed with a templated per-build env
in `.goreleaser.yml`: `CGO_ENABLED={{ if eq .Os "darwin" }}1{{ else }}0{{ end }}`.
Setting it on unconditionally broke the linux/arm cross-builds instead
(`clang: error: unsupported option '-mno-thumb'` — no linux C
cross-toolchain on macOS), confirming the per-OS conditional is required,
not optional.

Release job also moved `ubuntu-latest` → `macos-latest` (Linux can't link
Apple's private frameworks for cgo at all, regardless of the
`CGO_ENABLED` fix — both changes were necessary together). Trigger
switched from manual `v[0-9]+.[0-9]+.[0-9]+` tag push to push-to-`main` +
`lukaszraczylo/semver-generator`, matching the agreed `MacWhisperAuto`
pattern — goreleaser needs a real tag on HEAD (unlike MacWhisperAuto's
non-goreleaser `gh release create` flow), so the release job tags and
pushes the computed semver before invoking goreleaser.

**Verified empirically, thoroughly**: all 6 build targets
(`darwin/{arm64,amd64}`, `linux/{amd64,arm64,arm-v6,arm-v7}`) build clean
via direct `go build`; then, going further, installed goreleaser locally
and reproduced the *exact* original CI failure with the unmodified config,
then reproduced full success (`goreleaser release --snapshot --clean` —
all 6 archives + checksums) after the fix. **Not verified**: the actual
GitHub Actions environment itself (no access to trigger real CI from
here) — local `goreleaser` on real macOS hardware is strong evidence, but
isn't identical to a real `macos-latest` runner. Chris should confirm with
one real push-to-main once the diff looks right.

**This map (`profile-attestation`) is now fully clear** — every ticket
(01–06) resolved. Ready to hand off to `/to-spec`.
