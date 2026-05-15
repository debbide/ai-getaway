//go:build !linux && !windows

package service

func readPlatformSystemLoad() SystemLoad {
	return SystemLoad{SystemMetricsProvider: "runtime"}
}
