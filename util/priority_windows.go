//go:build windows

package util

import "golang.org/x/sys/windows"

// LowerProcessPriority runs the whole program at below-normal priority, so fetching, decoding and
// resizing pictures in the background never take the processor from what the person is doing
// (docs/lightweight-eval.md §4 B5). The tray menu and the settings window are still drawn at once:
// the lower class only matters while something else wants the processor.
func LowerProcessPriority() error {
	return windows.SetPriorityClass(windows.CurrentProcess(), windows.BELOW_NORMAL_PRIORITY_CLASS)
}

// ProcessPriorityClass returns the priority class of this process (for tests).
func ProcessPriorityClass() (uint32, error) {
	return windows.GetPriorityClass(windows.CurrentProcess())
}

// resetProcessPriority puts the priority class back to normal (for tests).
func resetProcessPriority() error {
	return windows.SetPriorityClass(windows.CurrentProcess(), windows.NORMAL_PRIORITY_CLASS)
}
