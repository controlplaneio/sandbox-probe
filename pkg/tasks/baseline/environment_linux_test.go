//go:build linux
// +build linux

package tasks

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getHostMounts(t *testing.T) {
	mounts, err := GetHostMounts()
	t.Logf("Mounts found: %v", mounts)
	assert.NoError(t, err)
}
