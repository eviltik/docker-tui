package main

import (
	"runtime"
	"testing"
)

// TestCountOpenFDs tests file descriptor counting
func TestCountOpenFDs(t *testing.T) {
	// countOpenFDs should work on Linux and return 0 on other platforms
	count := countOpenFDs()

	if runtime.GOOS == "linux" {
		// On Linux, should have at least a few FDs open (stdin, stdout, stderr + test harness)
		if count < 3 {
			t.Errorf("countOpenFDs() = %d, want >= 3 on Linux", count)
		}

		// Sanity check: shouldn't have thousands of FDs open in a test
		if count > 1000 {
			t.Errorf("countOpenFDs() = %d, suspiciously high number of FDs", count)
		}
	} else {
		// On non-Linux platforms, should return 0 (no /proc/self/fd)
		if count != 0 {
			t.Errorf("countOpenFDs() = %d, want 0 on non-Linux platforms", count)
		}
	}
}

// TestCountOpenFDsRepeatable tests that FD count is consistent
func TestCountOpenFDsRepeatable(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping on non-Linux platform")
	}

	// Count FDs twice - should be roughly the same
	count1 := countOpenFDs()
	count2 := countOpenFDs()

	// Allow small variation (GC might open/close files)
	diff := count1 - count2
	if diff < 0 {
		diff = -diff
	}

	if diff > 5 {
		t.Errorf("countOpenFDs() varied too much: %d vs %d (diff %d)", count1, count2, diff)
	}
}

// TestGetGoroutineCount tests goroutine counting
func TestGetGoroutineCount(t *testing.T) {
	count := getGoroutineCount()

	// Should have at least the test goroutine
	if count < 1 {
		t.Errorf("getGoroutineCount() = %d, want >= 1", count)
	}

	// Shouldn't have an unreasonable number in a simple test
	if count > 100 {
		t.Errorf("getGoroutineCount() = %d, suspiciously high for a test", count)
	}
}

// TestGetGoroutineCountConsistent tests goroutine count stability
func TestGetGoroutineCountConsistent(t *testing.T) {
	// Force GC to clean up any lingering goroutines
	runtime.GC()

	count1 := getGoroutineCount()
	count2 := getGoroutineCount()

	// Should be very stable (might vary by 1-2 due to test framework)
	diff := count1 - count2
	if diff < 0 {
		diff = -diff
	}

	if diff > 5 {
		t.Errorf("getGoroutineCount() varied too much: %d vs %d (diff %d)", count1, count2, diff)
	}
}
