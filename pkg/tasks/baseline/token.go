//go:build !windows
// +build !windows

package tasks

// A restricted token is a Windows access-token concept with no analogue elsewhere.
//
// The var exists on every platform, not just Windows, so the seam is stubbable from tests
// that carry no build tag — which is what lets the false-positive table in
// environment_test.go run on Linux and macOS too. Same shape as proc.go and landlock.go.
var isRestrictedToken = isRestrictedTokenImpl

func isRestrictedTokenImpl() bool { return false }
