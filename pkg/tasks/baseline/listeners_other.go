//go:build !linux && !darwin
// +build !linux,!darwin

package tasks

// No socket-table reader on this platform yet. Windows has one — iphlpapi's
// GetExtendedTcpTable and GetExtendedUdpTable, which is what `netstat -ano`
// calls and which needs no elevation — but it is not written here yet, because
// none of it has been run on a real Windows host. The named-pipe work next door
// is the precedent: ADR 0002 asserted an enumeration mechanism from
// documentation, and testing on an actual Windows 11 machine disproved it.
//
// Reporting "unsupported" is deliberate and is not the same as reporting an
// empty table. An empty table is a measurement; a missing implementation is
// not, and a caller that cannot tell them apart publishes a silence that scores
// as a sandbox having blocked everything.
func listLocalListeners() ([]Listener, error) { return nil, errUnsupportedInventory }

// networkNamespace is Linux-only. Elsewhere there is nothing to name.
func networkNamespace() string { return "" }
