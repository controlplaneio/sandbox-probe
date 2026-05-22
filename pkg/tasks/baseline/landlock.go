//go:build !linux
// +build !linux

package tasks

func ProbeLandlockSelfDepth() (int, error) {
	return 0, nil
}

var probeForLandlock = probeForLandlockImpl

func ProbeForLandlock() (bool, error) {
	return probeForLandlock()
}

func probeForLandlockImpl() (bool, error) {
	return false, nil
}
