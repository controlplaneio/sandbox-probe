//go:build !darwin
// +build !darwin

package tasks

func isSeatbelt() bool {
	return false
}
