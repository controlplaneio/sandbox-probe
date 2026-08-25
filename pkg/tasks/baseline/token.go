//go:build !windows
// +build !windows

package tasks

// A restricted token and an AppContainer are Windows access-token concepts with no analogue
// elsewhere.
//
// The vars exist on every platform, not just Windows, so the seams are stubbable from tests
// that carry no build tag — which is what lets the false-positive table in
// environment_test.go run on Linux and macOS too. Same shape as proc.go and landlock.go.
var isRestrictedToken = isRestrictedTokenImpl

func isRestrictedTokenImpl() bool { return false }

var isAppContainer = isAppContainerImpl

func isAppContainerImpl() bool { return false }
