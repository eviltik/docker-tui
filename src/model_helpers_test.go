package main

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/docker/docker/api/types"
)

// Test model.go helper functions

func TestCountSelected(t *testing.T) {
	tests := []struct {
		name     string
		selected map[string]bool
		want     int
	}{
		{"no selection", map[string]bool{}, 0},
		{"one selected", map[string]bool{"c1": true}, 1},
		{"multiple selected", map[string]bool{"c1": true, "c2": true, "c3": true}, 3},
		{"with false values", map[string]bool{"c1": true, "c2": false, "c3": true}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{
				selected:   tt.selected,
				selectedMu: sync.RWMutex{},
			}
			got := m.countSelected()
			if got != tt.want {
				t.Errorf("countSelected() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestShowActionConfirmation(t *testing.T) {
	m := &model{
		containers: []types.Container{
			{ID: "c1", Names: []string{"/container1"}},
			{ID: "c2", Names: []string{"/container2"}},
		},
		containersMu: sync.RWMutex{},
	}

	m.showActionConfirmation("start", []string{"c1", "c2"})

	if m.view != confirmView {
		t.Errorf("Expected view=confirmView, got %v", m.view)
	}
	if m.pendingAction != "start" {
		t.Errorf("Expected pendingAction=start, got %s", m.pendingAction)
	}
	if m.confirmMessage == "" {
		t.Error("Expected confirmMessage to be set")
	}
}

// TestUpdateWasAtBottom and TestGetFilteredLogCount already exist in model_test.go
// TestCleanContainerName already exists in formatters_test.go

// Test bufferconsumer.go functions

func TestBufferConsumer_OnContainerStatusChange(t *testing.T) {
	bc := &BufferConsumer{
		containerIDs: map[string]bool{"c1": true},
	}

	// Should not panic
	bc.OnContainerStatusChange("c1", true)
	bc.OnContainerStatusChange("c1", false)
}

func TestBufferConsumer_Clear(t *testing.T) {
	bc := &BufferConsumer{
		buffer: []LogEntry{
			{Line: "line1"},
			{Line: "line2"},
		},
		size:     2,
		head:     2,
		maxLines: 10,
	}

	bc.Clear()

	if bc.size != 0 {
		t.Errorf("Expected size=0, got %d", bc.size)
	}
	if bc.head != 0 {
		t.Errorf("Expected head=0, got %d", bc.head)
	}
}

func TestBufferConsumer_InsertSeparator(t *testing.T) {
	closing := atomic.Bool{}
	wg := sync.WaitGroup{}

	bc := NewBufferConsumer(
		[]string{"c1"},
		100,
		func(entry LogEntry) {
			// Callback may or may not be called synchronously
			// depending on goroutine scheduling
		},
		&closing,
		&wg,
	)

	bc.InsertSeparator()

	if bc.size != 1 {
		t.Errorf("Expected size=1 after insert, got %d", bc.size)
	}

	// Verify separator was added to buffer
	buffer := bc.GetBuffer()
	if len(buffer) == 0 {
		t.Fatal("Expected at least one entry in buffer")
	}
	if !buffer[0].IsSeparator {
		t.Error("Expected first entry to be a separator")
	}
}

func TestBufferConsumer_PreloadLogs(t *testing.T) {
	closing := atomic.Bool{}
	wg := sync.WaitGroup{}

	bc := NewBufferConsumer(
		[]string{"c1", "c2"},
		100,
		func(entry LogEntry) {},
		&closing,
		&wg,
	)

	recentLogs := map[string][]string{
		"c1": {"log1", "log2"},
		"c2": {"log3"},
	}

	containerNames := map[string]string{
		"c1": "container1",
		"c2": "container2",
	}

	bc.PreloadLogs([]string{"c1", "c2"}, recentLogs, containerNames)

	if bc.size != 3 {
		t.Errorf("Expected size=3 after preload, got %d", bc.size)
	}

	buffer := bc.GetBuffer()
	if len(buffer) != 3 {
		t.Errorf("Expected 3 entries in buffer, got %d", len(buffer))
	}
}

func TestBufferConsumer_GetBuffer(t *testing.T) {
	bc := &BufferConsumer{
		buffer: []LogEntry{
			{Line: "line1"},
			{Line: "line2"},
			{Line: "line3"},
		},
		size:     3,
		head:     3,
		maxLines: 100,
	}

	buffer := bc.GetBuffer()

	if len(buffer) != 3 {
		t.Errorf("Expected buffer length=3, got %d", len(buffer))
	}
}

func TestBufferConsumer_GetBuffer_CircularWrap(t *testing.T) {
	bc := &BufferConsumer{
		buffer: []LogEntry{
			{Line: "line3"},
			{Line: "line4"},
			{Line: "line1"}, // Oldest
			{Line: "line2"},
		},
		size:     4,
		head:     2, // Points to index 2 (next write position)
		maxLines: 4,
	}

	buffer := bc.GetBuffer()

	if len(buffer) != 4 {
		t.Errorf("Expected buffer length=4, got %d", len(buffer))
	}

	// Should return in correct order: oldest to newest
	expected := []string{"line1", "line2", "line3", "line4"}
	for i, entry := range buffer {
		if entry.Line != expected[i] {
			t.Errorf("Expected buffer[%d].Line=%s, got %s", i, expected[i], entry.Line)
		}
	}
}
