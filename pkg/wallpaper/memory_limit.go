package wallpaper

import (
	"os"
	"runtime/debug"
)

// goMemoryLimit is the soft limit on the memory the Go runtime manages (A2 phase 5 #1). Near it
// the garbage collector runs sooner and hands freed memory back to the system, instead of letting
// the heap grow to twice what is live. The app's own heap is about 100 MB; the largest original
// it decodes (a 63 MP progressive panorama) keeps about 550 MB live, so 700 MiB is reached only
// while such an image is processed. In the A2 measurement that image peaked at 647 MB of heap
// without the limit and 549 MB with it, in the same time. It is a soft limit: nothing fails above
// it, the collector only works harder (at most half the CPU).
const goMemoryLimit = 700 << 20

// ApplyMemoryLimit sets the soft memory limit, unless GOMEMLIMIT is set in the environment.
// It returns the limit in effect.
func ApplyMemoryLimit() int64 {
	if os.Getenv("GOMEMLIMIT") != "" {
		return debug.SetMemoryLimit(-1)
	}
	debug.SetMemoryLimit(goMemoryLimit)
	return goMemoryLimit
}
