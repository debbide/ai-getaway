//go:build linux

package service

import (
	"os"
	"strconv"
	"strings"
	"sync"
)

var (
	linuxCPUSampleLock sync.Mutex
	linuxPreviousIdle  uint64
	linuxPreviousTotal uint64
)

func readPlatformSystemLoad() SystemLoad {
	load := SystemLoad{SystemMetricsProvider: "linux"}
	load.CPUPercent = readLinuxCPUPercent()
	load.LoadAverage1, load.LoadAverage5, load.LoadAverage15 = readLinuxLoadAverage()
	total, used, percent := readLinuxMemory()
	load.MemoryTotalBytes = total
	load.MemoryUsedBytes = used
	load.MemoryUsedPercent = percent
	return load
}

func readLinuxCPUPercent() float64 {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}
	line := strings.SplitN(string(data), "\n", 2)[0]
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0
	}

	var values []uint64
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return 0
		}
		values = append(values, value)
	}

	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	var total uint64
	for _, value := range values {
		total += value
	}

	linuxCPUSampleLock.Lock()
	defer linuxCPUSampleLock.Unlock()
	if linuxPreviousTotal == 0 {
		linuxPreviousIdle = idle
		linuxPreviousTotal = total
		return 0
	}

	totalDelta := total - linuxPreviousTotal
	idleDelta := idle - linuxPreviousIdle
	linuxPreviousIdle = idle
	linuxPreviousTotal = total

	if totalDelta == 0 || idleDelta > totalDelta {
		return 0
	}
	return roundOneDecimal(float64(totalDelta-idleDelta) * 100 / float64(totalDelta))
}

func readLinuxLoadAverage() (float64, float64, float64) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0
	}
	return parseFloat(fields[0]), parseFloat(fields[1]), parseFloat(fields[2])
}

func readLinuxMemory() (uint64, uint64, float64) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, 0
	}
	var totalKB, availableKB uint64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			totalKB = value
		case "MemAvailable":
			availableKB = value
		}
	}
	if totalKB == 0 {
		return 0, 0, 0
	}
	usedKB := totalKB - availableKB
	total := totalKB * 1024
	used := usedKB * 1024
	return total, used, roundOneDecimal(float64(used) * 100 / float64(total))
}

func parseFloat(value string) float64 {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}
