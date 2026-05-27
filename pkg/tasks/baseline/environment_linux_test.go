//go:build linux
// +build linux

package tasks

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getHostMounts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping mount tests in short mode")
	}

	mounts, err := GetHostMounts()
	t.Logf("Mounts found: %v", mounts)
	assert.NoError(t, err)
}
