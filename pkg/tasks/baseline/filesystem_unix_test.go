//go:build !windows

package tasks

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsWritableDirectoryRequiresSearchPermission(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0200))
	t.Cleanup(func() { _ = os.Chmod(dir, 0700) })

	assert.False(t, isWritable(dir))
}
