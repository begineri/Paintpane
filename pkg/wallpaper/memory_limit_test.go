package wallpaper

import (
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyMemoryLimit(t *testing.T) {
	prev := debug.SetMemoryLimit(-1)
	defer debug.SetMemoryLimit(prev)

	t.Setenv("GOMEMLIMIT", "")
	assert.Equal(t, int64(goMemoryLimit), ApplyMemoryLimit())
	assert.Equal(t, int64(goMemoryLimit), debug.SetMemoryLimit(-1))

	// A limit chosen in the environment is kept.
	debug.SetMemoryLimit(1 << 30)
	t.Setenv("GOMEMLIMIT", "1GiB")
	assert.Equal(t, int64(1<<30), ApplyMemoryLimit())
	assert.Equal(t, int64(1<<30), debug.SetMemoryLimit(-1))
}
