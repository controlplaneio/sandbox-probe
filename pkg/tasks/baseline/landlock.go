//go:build !linux
// +build !linux

package tasks

func ProbeLandlockSelfDepth() (int, error) {
	return 0, nil
}
