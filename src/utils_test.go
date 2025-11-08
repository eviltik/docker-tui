package main

import (
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
)

// Test tick command functions and cleanup functions

func TestTickCmd_ReturnsCommand(t *testing.T) {
	cmd := tickCmd()

	if cmd == nil {
		t.Fatal("Expected tickCmd() to return a command")
	}

	// Don't execute the command (it waits 5 seconds)
	// Just verify it was created successfully
}

func TestCPUTickCmd_ReturnsCommand(t *testing.T) {
	cmd := cpuTickCmd()

	if cmd == nil {
		t.Fatal("Expected cpuTickCmd() to return a command")
	}

	// Don't execute the command (it waits 5 seconds)
	// Just verify it was created successfully
}

func TestLogRateTickCmd_ReturnsMessage(t *testing.T) {
	cmd := logRateTickCmd()

	if cmd == nil {
		t.Fatal("Expected logRateTickCmd() to return a command")
	}

	// Don't wait for the tick, just verify command creation
}

func TestCleanupTickCmd_ReturnsMessage(t *testing.T) {
	cmd := cleanupTickCmd()

	if cmd == nil {
		t.Fatal("Expected cleanupTickCmd() to return a command")
	}

	// Don't wait for the tick, just verify command creation
}

func TestCPUCleanupTickCmd_ReturnsMessage(t *testing.T) {
	cmd := cpuCleanupTickCmd()

	if cmd == nil {
		t.Fatal("Expected cpuCleanupTickCmd() to return a command")
	}

	// Don't wait for the tick, just verify command creation
}

// Test performAction edge cases

func TestModel_PerformAction_NoSelection_UsesCursor(t *testing.T) {
	m := &model{
		cursor:       0,
		containers:   []types.Container{{ID: "c1"}},
		containersMu: sync.RWMutex{},
		selected:     make(map[string]bool), // Empty selection
		selectedMu:   sync.RWMutex{},
	}

	cmd := m.performAction("start")

	// Without selection, should use cursor container
	if cmd == nil {
		t.Fatal("Expected command (should use cursor container)")
	}

	msg := cmd()
	actionMsg, ok := msg.(actionStartMsg)
	if !ok {
		t.Fatalf("Expected actionStartMsg, got %T", msg)
	}

	if len(actionMsg.ids) != 1 || actionMsg.ids[0] != "c1" {
		t.Errorf("Expected cursor container c1, got %v", actionMsg.ids)
	}
}

func TestModel_PerformAction_WithSelection(t *testing.T) {
	m := &model{
		cursor: 0,
		containers: []types.Container{
			{ID: "c1"},
			{ID: "c2"},
		},
		containersMu: sync.RWMutex{},
		selected: map[string]bool{
			"c1": true,
			"c2": true,
		},
		selectedMu: sync.RWMutex{},
	}

	cmd := m.performAction("restart")

	if cmd == nil {
		t.Fatal("Expected command when containers are selected")
	}

	// Execute command to get actionStartMsg
	msg := cmd()

	actionMsg, ok := msg.(actionStartMsg)
	if !ok {
		t.Fatalf("Expected actionStartMsg, got %T", msg)
	}

	if actionMsg.action != "restart" {
		t.Errorf("Expected action='restart', got %s", actionMsg.action)
	}

	if len(actionMsg.ids) != 2 {
		t.Errorf("Expected 2 IDs, got %d", len(actionMsg.ids))
	}
}

func TestModel_PerformAction_DifferentActions(t *testing.T) {
	actions := []string{"start", "stop", "restart", "remove", "pause"}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			m := &model{
				cursor: 0,
				containers: []types.Container{
					{ID: "c1"},
				},
				containersMu: sync.RWMutex{},
				selected: map[string]bool{
					"c1": true,
				},
				selectedMu: sync.RWMutex{},
			}

			cmd := m.performAction(action)

			if cmd == nil {
				t.Fatal("Expected command")
			}

			msg := cmd()
			actionMsg, ok := msg.(actionStartMsg)
			if !ok {
				t.Fatalf("Expected actionStartMsg, got %T", msg)
			}

			if actionMsg.action != action {
				t.Errorf("Expected action=%s, got %s", action, actionMsg.action)
			}
		})
	}
}

// Test CleanupStaleContainers

func TestRateTrackerConsumer_CleanupStaleContainers_NoStale(t *testing.T) {
	rtc := NewRateTrackerConsumer()

	// Add some recent logs
	rtc.OnLogLine("c1", "container1", "log", time.Now())
	rtc.OnLogLine("c2", "container2", "log", time.Now())

	// Run cleanup
	rtc.CleanupStaleContainers()

	// Both should still exist (recent activity)
	if rtc.GetRate("c1") == 0 && rtc.GetRate("c2") == 0 {
		t.Error("Expected containers to remain (no stale)")
	}
}

func TestRateTrackerConsumer_CleanupStaleContainers_WithStale(t *testing.T) {
	rtc := NewRateTrackerConsumer()

	// Add a container with old timestamp
	oldTime := time.Now().Add(-10 * time.Minute)
	rtc.OnLogLine("stale", "stale-container", "log", oldTime)

	// Add a recent container
	rtc.OnLogLine("active", "active-container", "log", time.Now())

	// Manually set the lastUpdate to be old for stale container
	rtc.ratesMu.Lock()
	if tracker := rtc.rates["stale"]; tracker != nil {
		tracker.mu.Lock()
		tracker.lastUpdate = oldTime
		tracker.mu.Unlock()
	}
	rtc.ratesMu.Unlock()

	// Run cleanup
	rtc.CleanupStaleContainers()

	// Stale should be removed, active should remain
	if rtc.GetRate("stale") != 0 {
		t.Error("Expected stale container to be cleaned up")
	}
	if rtc.GetRate("active") == 0 {
		t.Error("Expected active container to remain")
	}
}

func TestRateTrackerConsumer_CleanupStaleContainers_EmptyRates(t *testing.T) {
	rtc := NewRateTrackerConsumer()

	// Should not panic on empty rates
	rtc.CleanupStaleContainers()
}

func TestRateTrackerConsumer_CleanupStaleContainers_AllStale(t *testing.T) {
	rtc := NewRateTrackerConsumer()

	// Add containers with old timestamps
	oldTime := time.Now().Add(-10 * time.Minute)

	for i := 0; i < 5; i++ {
		containerID := string('a' + rune(i))
		rtc.OnLogLine(containerID, "container", "log", oldTime)

		// Manually set old lastUpdate
		rtc.ratesMu.Lock()
		if tracker := rtc.rates[containerID]; tracker != nil {
			tracker.mu.Lock()
			tracker.lastUpdate = oldTime
			tracker.mu.Unlock()
		}
		rtc.ratesMu.Unlock()
	}

	// Run cleanup
	rtc.CleanupStaleContainers()

	// All should be cleaned
	rtc.ratesMu.RLock()
	count := len(rtc.rates)
	rtc.ratesMu.RUnlock()

	if count != 0 {
		t.Errorf("Expected all containers to be cleaned, got %d remaining", count)
	}
}

// Test BufferConsumer OnContainerStatusChange (it's a no-op but should be covered)

func TestBufferConsumer_OnContainerStatusChange_Covered(t *testing.T) {
	bc := &BufferConsumer{
		buffer:       make([]LogEntry, 100),
		bufferMu:     sync.RWMutex{},
		containerIDs: map[string]bool{"c1": true},
	}

	// Call it to get coverage (it's a no-op)
	bc.OnContainerStatusChange("c1", true)
	bc.OnContainerStatusChange("c1", false)

	// Should not affect buffer
	if len(bc.GetBuffer()) != 0 {
		t.Error("OnContainerStatusChange should not modify buffer (it's a no-op)")
	}
}
