package service

import (
	"runtime"
	"sync/atomic"
	"time"
)

var activeAPIConnections int64

type SystemLoad struct {
	CPUPercent            float64 `json:"cpu_percent"`
	MemoryUsedPercent     float64 `json:"memory_used_percent"`
	MemoryUsedBytes       uint64  `json:"memory_used_bytes"`
	MemoryTotalBytes      uint64  `json:"memory_total_bytes"`
	ProcessMemoryBytes    uint64  `json:"process_memory_bytes"`
	Goroutines            int     `json:"goroutines"`
	GoRoutines            int     `json:"go_routines"`
	LoadAverage1          float64 `json:"load_average_1"`
	LoadAverage5          float64 `json:"load_average_5"`
	LoadAverage15         float64 `json:"load_average_15"`
	CPUCount              int     `json:"cpu_count"`
	SampledAt             string  `json:"sampled_at"`
	SystemMetricsProvider string  `json:"system_metrics_provider"`
}

func AddActiveAPIConnection(delta int64) int64 {
	next := atomic.AddInt64(&activeAPIConnections, delta)
	if next < 0 {
		atomic.StoreInt64(&activeAPIConnections, 0)
		return 0
	}
	return next
}

func ActiveAPIConnections() int64 {
	current := atomic.LoadInt64(&activeAPIConnections)
	if current < 0 {
		return 0
	}
	return current
}

func CurrentSystemLoad() SystemLoad {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	load := readPlatformSystemLoad()
	load.ProcessMemoryBytes = mem.Alloc
	load.Goroutines = runtime.NumGoroutine()
	load.GoRoutines = load.Goroutines
	load.CPUCount = runtime.NumCPU()
	load.SampledAt = time.Now().Format(time.RFC3339)
	return load
}
