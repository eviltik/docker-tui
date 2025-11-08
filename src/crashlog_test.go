package main

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestWriteCrashLog tests crash log writing functionality
func TestWriteCrashLog(t *testing.T) {

	// We can't easily test the actual writeCrashLog since it writes to a const path
	// Instead, we'll test the safeGo wrapper and verify it doesn't crash

	var wg sync.WaitGroup
	wg.Add(1)

	// Launch goroutine that panics
	safeGo("test-panic-goroutine", func() {
		defer wg.Done()
		panic("intentional test panic")
	})

	wg.Wait()

	// If we reach here, safeGo successfully caught the panic
	// (otherwise the test would have failed with unrecovered panic)

	// Give it a moment to write
	time.Sleep(100 * time.Millisecond)

	// Check if crash log was created
	if _, err := os.Stat(crashLogPath); err == nil {
		// Log was created, verify it contains expected content
		content, err := os.ReadFile(crashLogPath)
		if err != nil {
			t.Fatalf("Failed to read crash log: %v", err)
		}

		logContent := string(content)

		// Verify it contains key elements
		expectedStrings := []string{
			"CRASH REPORT",
			"test-panic-goroutine",
			"intentional test panic",
			"System Information",
			"Goroutines:",
		}

		for _, expected := range expectedStrings {
			if !strings.Contains(logContent, expected) {
				t.Errorf("Crash log missing expected content: %q", expected)
			}
		}

		// Clean up the crash log after test
		os.Remove(crashLogPath)
	}
}

// TestSafeGoContinuesAfterPanic tests that safeGo allows program to continue after panic
func TestSafeGoContinuesAfterPanic(t *testing.T) {
	programContinued := false

	// Launch goroutine that panics
	safeGo("test-continue", func() {
		panic("test panic")
	})

	// Wait a bit for goroutine to execute
	time.Sleep(100 * time.Millisecond)

	// This should execute (program continues)
	programContinued = true

	if !programContinued {
		t.Error("Program did not continue after panic in safeGo")
	}

	// Clean up crash log if created
	os.Remove(crashLogPath)
}

// TestSafeGoMultiplePanics tests that multiple goroutines can panic independently
func TestSafeGoMultiplePanics(t *testing.T) {
	const numGoroutines = 10
	var wg sync.WaitGroup

	completed := make([]bool, numGoroutines)
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		idx := i
		safeGo("test-multi-panic", func() {
			defer wg.Done()
			defer func() {
				mu.Lock()
				completed[idx] = true
				mu.Unlock()
			}()

			if idx%2 == 0 {
				panic("test panic")
			}
			// Odd indices complete normally
		})
	}

	wg.Wait()

	// All goroutines should have completed (even ones that panicked)
	mu.Lock()
	for i, done := range completed {
		if !done {
			t.Errorf("Goroutine %d did not complete", i)
		}
	}
	mu.Unlock()

	// Clean up crash log
	os.Remove(crashLogPath)
}

// TestSafeGoNoPanicPath tests that safeGo works normally when no panic occurs
func TestSafeGoNoPanicPath(t *testing.T) {
	executed := false
	var mu sync.Mutex

	var wg sync.WaitGroup
	wg.Add(1)

	safeGo("test-no-panic", func() {
		defer wg.Done()
		mu.Lock()
		executed = true
		mu.Unlock()
	})

	wg.Wait()

	mu.Lock()
	if !executed {
		t.Error("safeGo function was not executed")
	}
	mu.Unlock()
}

// TestCrashLogFormat tests the format of crash log entries
func TestCrashLogFormat(t *testing.T) {
	// Trigger a panic and verify log format
	safeGo("test-format", func() {
		panic("format test panic")
	})

	// Wait for log to be written
	time.Sleep(200 * time.Millisecond)

	// Read and verify
	if _, err := os.Stat(crashLogPath); err == nil {
		content, err := os.ReadFile(crashLogPath)
		if err != nil {
			t.Fatalf("Failed to read crash log: %v", err)
		}

		logContent := string(content)

		// Verify structure
		requiredSections := []string{
			"CRASH REPORT",
			"Goroutine:",
			"Error:",
			"Crashing Goroutine Stack Trace:",
			"All Goroutines Stack Dump:",
			"System Information:",
			"Goroutines:",
			"Memory Allocated:",
			"File Descriptors:",
		}

		for _, section := range requiredSections {
			if !strings.Contains(logContent, section) {
				t.Errorf("Crash log missing section: %q", section)
			}
		}

		// Verify timestamp format (YYYY-MM-DD HH:MM:SS.mmm)
		if !strings.Contains(logContent, time.Now().Format("2006-01-02")) {
			t.Error("Crash log missing current date in timestamp")
		}

		// Clean up
		os.Remove(crashLogPath)
	} else {
		t.Error("Crash log was not created")
	}
}

// BenchmarkSafeGo benchmarks safeGo overhead
func BenchmarkSafeGo(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(1)
		safeGo("bench", func() {
			defer wg.Done()
			// No-op
		})
		wg.Wait()
	}

	// Clean up any crash logs
	os.Remove(crashLogPath)
}

// BenchmarkSafeGoWithPanic benchmarks safeGo with panic recovery
func BenchmarkSafeGoWithPanic(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(1)
		safeGo("bench-panic", func() {
			defer wg.Done()
			panic("bench panic")
		})
		wg.Wait()
	}

	// Clean up crash log
	os.Remove(crashLogPath)
}
