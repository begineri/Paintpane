//go:build windows

package wallpaper

import (
	"encoding/binary"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	wtsapi32                 = windows.NewLazySystemDLL("wtsapi32.dll")
	procWTSQuerySessionInfoW = wtsapi32.NewProc("WTSQuerySessionInformationW")
	procWTSFreeMemory        = wtsapi32.NewProc("WTSFreeMemory")
)

const (
	wtsCurrentSession = 0xFFFFFFFF
	wtsSessionInfo    = 24 // WTS_INFO_CLASS WTSSessionInfo: a WTSINFOW

	// Offsets in WTSINFOW (x64 and x86 alike, its fields are naturally aligned): State, SessionId
	// and six counters of 4 bytes, WinStationName[32], Domain[17], UserName[21] as UTF-16, padding
	// to 8, then the times as 64-bit FILETIMEs.
	wtsInfoLogonTime   = 200
	wtsInfoCurrentTime = 208
	wtsInfoSize        = 216
)

// startedSoonAfterLogon reports whether the current Windows session began less than within ago:
// a program in the Run key starts a few seconds after logon, a program the person starts does so
// later. It reports false when Windows does not say.
func startedSoonAfterLogon(within time.Duration) bool {
	since, ok := timeSinceLogon()
	return ok && since >= 0 && since < within
}

func timeSinceLogon() (time.Duration, bool) {
	var buf unsafe.Pointer
	var size uint32
	r, _, _ := procWTSQuerySessionInfoW.Call(0, wtsCurrentSession, wtsSessionInfo, uintptr(unsafe.Pointer(&buf)), uintptr(unsafe.Pointer(&size)))
	if r == 0 || buf == nil {
		return 0, false
	}
	defer procWTSFreeMemory.Call(uintptr(buf)) //nolint:errcheck
	if size < wtsInfoSize {
		return 0, false
	}
	info := unsafe.Slice((*byte)(buf), size)
	logon := int64(binary.LittleEndian.Uint64(info[wtsInfoLogonTime:]))
	now := int64(binary.LittleEndian.Uint64(info[wtsInfoCurrentTime:]))
	if logon <= 0 || now < logon {
		return 0, false
	}
	return time.Duration(now-logon) * 100 * time.Nanosecond, true // FILETIME counts 100 ns
}
