//go:build !linux
// +build !linux

package tasks

var isChroot = isChrootImpl

func isChrootImpl() bool {
	return false
}

func isProcSelfSetNoNewPrivs() bool {
	return false
}

func isUserNamespaceWithUIDMap() bool {
	return false
}
