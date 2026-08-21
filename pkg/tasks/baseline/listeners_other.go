//go:build !linux && !darwin && !windows
// +build !linux,!darwin,!windows

package tasks

// No socket-table reader on this platform. Linux, macOS and Windows each have
// one; anything else (freebsd, and the js/wasm and plan9 targets the toolchain
// will happily build for) does not.
//
// Reporting "unsupported" is deliberate and is not the same as reporting an
// empty table. An empty table is a measurement; a missing implementation is
// not, and a caller that cannot tell them apart publishes a silence that scores
// as a sandbox having blocked everything.
func listLocalListeners() ([]Listener, error) { return nil, errUnsupportedInventory }

// networkNamespace is Linux-only. Elsewhere there is nothing to name.
func networkNamespace() string { return "" }

// inventorySource names where the table came from, for the report.
func inventorySource() string { return "unsupported" }
