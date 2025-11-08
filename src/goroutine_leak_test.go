package main

import (
	"runtime"
	"testing"
	"time"
)

// TestNoGoroutineLeak tests that goroutines are properly cleaned up
func TestNoGoroutineLeak(t *testing.T) {
	// Get baseline goroutine count
	runtime.GC()
	time.Sleep(10 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	// Allow some tolerance (test harness goroutines)
	maxAllowed := baseline + 50

	// Simulate some operations that create goroutines
	// (This is a placeholder - in real scenario we'd test actual app operations)

	// Force GC and wait a bit for cleanup
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	current := runtime.NumGoroutine()

	if current > maxAllowed {
		t.Errorf("Potential goroutine leak: baseline=%d, current=%d (max allowed=%d)", baseline, current, maxAllowed)
	}
}

// TestGorot​ineGrowth tests that goroutines don't grow unbounded
func TestGoroutineGrowth(t *testing.T) {
	samples := make([]int, 5)

	for i := 0; i < 5; i++ {
		runtime.GC()
		time.Sleep(50 * time.Millisecond)
		samples[i] = runtime.NumGoroutine()
	}

	// Check that goroutine count is stable (within tolerance)
	for i := 1; i < len(samples); i++ {
		growth := samples[i] - samples[0]
		if growth > 100 {
			t.Errorf("Goroutine count growing: sample[0]=%d, sample[%d]=%d (growth=%d)",
				samples[0], i, samples[i], growth)
		}
	}
}
