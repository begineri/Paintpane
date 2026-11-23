//go:build windows

package wallpaper

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	layoutUser32            = windows.NewLazySystemDLL("user32.dll")
	procEnumDisplayMonitors = layoutUser32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW     = layoutUser32.NewProc("GetMonitorInfoW")

	// The runtime allows only about 2000 callbacks per process, so there is one, made once, and
	// layoutMu serialises its use of layoutParts.
	layoutMu       sync.Mutex
	layoutParts    []string
	layoutCallback = windows.NewCallback(addMonitorToLayout)
)

// monitorInfoEx is MONITORINFOEXW.
type monitorInfoEx struct {
	Size    uint32
	Monitor struct{ Left, Top, Right, Bottom int32 }
	Work    struct{ Left, Top, Right, Bottom int32 }
	Flags   uint32
	Device  [32]uint16
}

// displayLayoutKey describes the connected displays (each one's device name and rectangle) with
// a few user32 calls. The monitor watcher compares it with the last one to skip the full check,
// which creates the shell's IDesktopWallpaper COM object and took about 50 ms of CPU every 90
// seconds. It returns "" when the layout cannot be read, and the watcher then checks fully.
func displayLayoutKey() string {
	layoutMu.Lock()
	defer layoutMu.Unlock()
	layoutParts = layoutParts[:0]
	if r, _, _ := procEnumDisplayMonitors.Call(0, 0, layoutCallback, 0); r == 0 || len(layoutParts) == 0 {
		return ""
	}
	sort.Strings(layoutParts)
	return strings.Join(layoutParts, ";")
}

// addMonitorToLayout is the MONITORENUMPROC for displayLayoutKey; layoutMu is held.
func addMonitorToLayout(hMonitor, _, _, _ uintptr) uintptr {
	var mi monitorInfoEx
	mi.Size = uint32(unsafe.Sizeof(mi))
	if r, _, _ := procGetMonitorInfoW.Call(hMonitor, uintptr(unsafe.Pointer(&mi))); r == 0 {
		layoutParts = append(layoutParts, "?")
		return 1
	}
	m := mi.Monitor
	layoutParts = append(layoutParts, fmt.Sprintf("%s=%d,%d,%d,%d",
		windows.UTF16ToString(mi.Device[:]), m.Left, m.Top, m.Right, m.Bottom))
	return 1
}
