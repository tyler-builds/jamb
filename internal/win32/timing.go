//go:build windows

package win32

import (
	"encoding/binary"
	"syscall"
	"unsafe"
)

// This file binds the performance counter, the refresh rate of a monitor and the
// memory counters of the process. The clock is for measurements: time.Now advances
// in steps of about 0.5 ms on Windows, which the counter does not.

var (
	psapi                    = syscall.NewLazyDLL("psapi.dll")
	procGetProcessMemoryInfo = psapi.NewProc("GetProcessMemoryInfo")
	procGetCurrentProcess    = kernel32.NewProc("GetCurrentProcess")
	procQueryPerfCounter     = kernel32.NewProc("QueryPerformanceCounter")
	procQueryPerfFrequency   = kernel32.NewProc("QueryPerformanceFrequency")
	procEnumDisplaySettingsW = user32.NewProc("EnumDisplaySettingsW")
)

// perfFrequency is the performance counter's frequency in ticks per second.
var perfFrequency = func() int64 {
	var f int64
	procQueryPerfFrequency.Call(uintptr(unsafe.Pointer(&f)))
	return max(f, 1)
}()

// PerformanceNanos returns the performance counter in nanoseconds. The origin is
// arbitrary, so only differences mean anything. The counter is split into seconds
// and a remainder before scaling, so a long uptime does not overflow.
func PerformanceNanos() int64 {
	var c int64
	procQueryPerfCounter.Call(uintptr(unsafe.Pointer(&c)))
	f := perfFrequency
	return c/f*1e9 + c%f*1e9/f
}

// monitorInfoEx is MONITORINFOEXW: a MONITORINFO and the monitor's device name.
type monitorInfoEx struct {
	MONITORINFO
	Device [32]uint16
}

// MonitorRefreshRate returns the refresh rate, in hertz, of the monitor nearest to
// hwnd, or 0 when the system does not report one.
func MonitorRefreshRate(hwnd uintptr) float64 {
	mon, _, _ := procMonitorFromWindow.Call(hwnd, MONITOR_DEFAULTTONEAREST)
	mi := monitorInfoEx{}
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	if r, _, _ := procGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi))); r == 0 {
		return 0
	}
	// DEVMODEW, from wingdi.h, is 220 bytes; dmDisplayFrequency is at offset 184. The
	// struct is read by offset here, since only that one field is used.
	const devModeSize, freqOffset = 220, 184
	var dm [devModeSize]byte
	const enumCurrentSettings = 0xFFFFFFFF // ENUM_CURRENT_SETTINGS, a DWORD
	if r, _, _ := procEnumDisplaySettingsW.Call(uintptr(unsafe.Pointer(&mi.Device[0])), enumCurrentSettings,
		uintptr(unsafe.Pointer(&dm[0]))); r == 0 {
		return 0
	}
	// 0 and 1 mean the adapter's default, which is not a rate.
	if hz := binary.LittleEndian.Uint32(dm[freqOffset : freqOffset+4]); hz > 1 {
		return float64(hz)
	}
	return 0
}

// processMemoryCounters is PROCESS_MEMORY_COUNTERS_EX, as GetProcessMemoryInfo fills it.
type processMemoryCounters struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
	PrivateUsage               uintptr
}

// ProcessMemoryUsage returns the process's private bytes (the memory committed to it
// alone) and its working set, in bytes. Both are 0 when the system does not report them.
func ProcessMemoryUsage() (privateBytes, workingSet uint64) {
	var c processMemoryCounters
	c.CB = uint32(unsafe.Sizeof(c))
	self, _, _ := procGetCurrentProcess.Call()
	if r, _, _ := procGetProcessMemoryInfo.Call(self, uintptr(unsafe.Pointer(&c)), uintptr(c.CB)); r == 0 {
		return 0, 0
	}
	return uint64(c.PrivateUsage), uint64(c.WorkingSetSize)
}
