package main

import (
	"sync"
	"testing"
	"time"

)

// Test lifecycle functions: Init, tick commands, waitForNewLog

func TestModel_Init(t *testing.T) {
	// Create a mock Docker client (nil is ok for this test)
	m := &model{
		dockerClient: nil, // Init doesn't use it directly
		logBroker:    NewLogBroker(nil),
		rateTracker:  NewRateTrackerConsumer(),
	}

	cmd := m.Init()

	if cmd == nil {
		t.Fatal("Expected Init() to return a command")
	}

	// Execute the batch command to ensure it doesn't panic
	msg := cmd()
	if msg == nil {
		t.Error("Expected Init() batch command to return a message")
	}
}

func TestLogRateTickCmd(t *testing.T) {
	cmd := logRateTickCmd()

	if cmd == nil {
		t.Fatal("Expected logRateTickCmd() to return a command")
	}

	// The command returns a tea.Cmd that will emit logRateTickMsg after 500ms
	// We can't easily test the timing, but we can verify it's created
}

func TestCleanupTickCmd(t *testing.T) {
	cmd := cleanupTickCmd()

	if cmd == nil {
		t.Fatal("Expected cleanupTickCmd() to return a command")
	}
}

func TestCPUCleanupTickCmd(t *testing.T) {
	cmd := cpuCleanupTickCmd()

	if cmd == nil {
		t.Fatal("Expected cpuCleanupTickCmd() to return a command")
	}
}

func TestWaitForNewLog(t *testing.T) {
	ch := make(chan struct{}, 1)
	cmd := waitForNewLog(ch)

	if cmd == nil {
		t.Fatal("Expected waitForNewLog() to return a command")
	}

	// Send a signal and verify the command returns newLogLineMsg
	go func() {
		time.Sleep(10 * time.Millisecond)
		ch <- struct{}{}
	}()

	msg := cmd()
	if _, ok := msg.(newLogLineMsg); !ok {
		t.Errorf("Expected newLogLineMsg, got %T", msg)
	}
}

func TestWaitForNewLog_Blocking(t *testing.T) {
	ch := make(chan struct{})
	cmd := waitForNewLog(ch)

	done := make(chan bool)
	go func() {
		msg := cmd()
		if _, ok := msg.(newLogLineMsg); !ok {
			t.Errorf("Expected newLogLineMsg, got %T", msg)
		}
		done <- true
	}()

	// Wait a bit to ensure goroutine is waiting
	time.Sleep(50 * time.Millisecond)

	// Now send signal
	ch <- struct{}{}

	// Wait for completion
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("waitForNewLog() did not complete after signal")
	}
}

func TestCPUTickCmd(t *testing.T) {
	cmd := cpuTickCmd()

	if cmd == nil {
		t.Fatal("Expected cpuTickCmd() to return a command")
	}

	// cpuTickCmd returns a tea.Tick that will emit cpuTickMsg after 5 seconds
	// We can verify the command is created without waiting 5 seconds
}

// Test OnContainerStatusChange for RateTrackerConsumer

func TestRateTrackerConsumer_OnContainerStatusChange_Running(t *testing.T) {
	rtc := NewRateTrackerConsumer()

	// Add a container with some rate data
	rtc.OnLogLine("c1", "container1", "log line", time.Now())

	if rtc.GetRate("c1") == 0 {
		t.Error("Expected non-zero rate after adding log line")
	}

	// Notify that container is still running (should do nothing)
	rtc.OnContainerStatusChange("c1", true)

	// Rate should still be there
	if rtc.GetRate("c1") == 0 {
		t.Error("Expected rate to persist when container is running")
	}
}

func TestRateTrackerConsumer_OnContainerStatusChange_Stopped(t *testing.T) {
	rtc := NewRateTrackerConsumer()

	// Add a container with some rate data
	rtc.OnLogLine("c1", "container1", "log line", time.Now())
	rtc.OnLogLine("c1", "container1", "log line 2", time.Now())

	if rtc.GetRate("c1") == 0 {
		t.Error("Expected non-zero rate after adding log lines")
	}

	// Notify that container stopped
	rtc.OnContainerStatusChange("c1", false)

	// Rate should be cleared
	if rtc.GetRate("c1") != 0 {
		t.Error("Expected rate to be cleared when container stops")
	}
}

func TestRateTrackerConsumer_OnContainerStatusChange_MultipleContainers(t *testing.T) {
	rtc := NewRateTrackerConsumer()

	// Add multiple containers
	rtc.OnLogLine("c1", "container1", "log", time.Now())
	rtc.OnLogLine("c2", "container2", "log", time.Now())
	rtc.OnLogLine("c3", "container3", "log", time.Now())

	// Stop c2
	rtc.OnContainerStatusChange("c2", false)

	// c2 should be cleared, c1 and c3 should remain
	if rtc.GetRate("c2") != 0 {
		t.Error("Expected c2 rate to be 0 after stopping")
	}
	if rtc.GetRate("c1") == 0 {
		t.Error("Expected c1 rate to persist")
	}
	if rtc.GetRate("c3") == 0 {
		t.Error("Expected c3 rate to persist")
	}
}

// Test OnContainerStatusChange for BufferConsumer

func TestBufferConsumer_OnContainerStatusChange_DoesNotPanic(t *testing.T) {
	bc := &BufferConsumer{
		buffer:       make([]LogEntry, 100),
		bufferMu:     sync.RWMutex{},
		containerIDs: map[string]bool{"c1": true},
	}

	// Should not panic (it's a no-op)
	bc.OnContainerStatusChange("c1", false)
	bc.OnContainerStatusChange("c1", true)
	bc.OnContainerStatusChange("unknown", false)
}

func TestBufferConsumer_OnContainerStatusChange_MultipleCallsSafe(t *testing.T) {
	bc := &BufferConsumer{
		buffer:       make([]LogEntry, 100),
		bufferMu:     sync.RWMutex{},
		containerIDs: map[string]bool{"c1": true, "c2": true},
	}

	// Call multiple times concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			bc.OnContainerStatusChange(id, true)
			bc.OnContainerStatusChange(id, false)
		}("c1")
	}

	wg.Wait()
	// If we reach here without panic, test passes
}
