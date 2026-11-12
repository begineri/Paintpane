//go:build windows

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestLowerProcessPriority(t *testing.T) {
	t.Cleanup(func() { _ = resetProcessPriority() })

	require.NoError(t, resetProcessPriority())
	class, err := ProcessPriorityClass()
	require.NoError(t, err)
	assert.Equal(t, uint32(windows.NORMAL_PRIORITY_CLASS), class)

	require.NoError(t, LowerProcessPriority())
	class, err = ProcessPriorityClass()
	require.NoError(t, err)
	assert.Equal(t, uint32(windows.BELOW_NORMAL_PRIORITY_CLASS), class, "the process runs below normal")
}
