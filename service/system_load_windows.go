//go:build windows

package service

import (
	"sync"
	"syscall"
	"unsafe"
)

var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemTimes        = kernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx  = kernel32.NewProc("GlobalMemoryStatusEx")
	windowsCPUSampleLock      sync.Mutex
	windowsPreviousIdleTime   uint64
	windowsPreviousKernelTime uint64
	windowsPreviousUserTime   uint64
)

type filetime struct {
	LowDateTime  uint32
	HighDateTime uint32
}

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

func readPlatformSystemLoad() SystemLoad {
	load := SystemLoad{SystemMetricsProvider: "windows"}
	load.CPUPercent = readWindowsCPUPercent()
	total, used, percent := readWindowsMemory()
	load.MemoryTotalBytes = total
	load.MemoryUsedBytes = used
	load.MemoryUsedPercent = percent
	return load
}

func readWindowsCPUPercent() float64 {
	var idleTime, kernelTime, userTime filetime
	ret, _, _ := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idleTime)),
		uintptr(unsafe.Pointer(&kernelTime)),
		uintptr(unsafe.Pointer(&userTime)),
	)
	if ret == 0 {
		return 0
	}

	idle := filetimeToUint64(idleTime)
	kernel := filetimeToUint64(kernelTime)
	user := filetimeToUint64(userTime)

	windowsCPUSampleLock.Lock()
	defer windowsCPUSampleLock.Unlock()
	if windowsPreviousKernelTime == 0 && windowsPreviousUserTime == 0 {
		windowsPreviousIdleTime = idle
		windowsPreviousKernelTime = kernel
		windowsPreviousUserTime = user
		return 0
	}

	idleDelta := idle - windowsPreviousIdleTime
	kernelDelta := kernel - windowsPreviousKernelTime
	userDelta := user - windowsPreviousUserTime
	totalDelta := kernelDelta + userDelta

	windowsPreviousIdleTime = idle
	windowsPreviousKernelTime = kernel
	windowsPreviousUserTime = user

	if totalDelta == 0 || idleDelta > totalDelta {
		return 0
	}
	return roundOneDecimal(float64(totalDelta-idleDelta) * 100 / float64(totalDelta))
}

func readWindowsMemory() (uint64, uint64, float64) {
	status := memoryStatusEx{Length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	ret, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status)))
	if ret == 0 || status.TotalPhys == 0 {
		return 0, 0, 0
	}
	used := status.TotalPhys - status.AvailPhys
	return status.TotalPhys, used, roundOneDecimal(float64(used) * 100 / float64(status.TotalPhys))
}

func filetimeToUint64(value filetime) uint64 {
	return uint64(value.HighDateTime)<<32 + uint64(value.LowDateTime)
}
