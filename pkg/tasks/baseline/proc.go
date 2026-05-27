//go:build !linux
// +build !linux

package tasks

func isProcSelfSetNoNewPrivs() bool {
	return false
}

func isUserNamespaceWithUIDMap() bool {
	return false
}
