package main

import (
	"os"
	"runtime"
)

// countOpenFDs returns the number of open file descriptors
// Linux only - returns 0 on other platforms
func countOpenFDs() int {
	// Try to read /proc/self/fd/
	fdDir := "/proc/self/fd"

	entries, err := os.ReadDir(fdDir)
	if err != nil {
		return 0 // Not Linux or no access
	}

	return len(entries)
}

// getGoroutineCount returns the current number of goroutines
func getGoroutineCount() int {
	return runtime.NumGoroutine()
}
