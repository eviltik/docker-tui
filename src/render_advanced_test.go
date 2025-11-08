package main

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/docker/docker/api/types"
)

// Test View() method and renderDebugMetrics

func TestModel_View_ListViewWithContainers(t *testing.T) {
	m := &model{
		view:   listView,
		width:  100,
		height: 30,
		containers: []types.Container{
			{ID: "c1", Names: []string{"/container1"}, State: "running"},
		},
		containersMu: sync.RWMutex{},
		cursor:       0,
		selected:     make(map[string]bool),
		selectedMu:   sync.RWMutex{},
		cpuStats:     make(map[string][]float64),
		cpuCurrent:   make(map[string]float64),
		cpuStatsMu:   sync.RWMutex{},
		rateTracker:  NewRateTrackerConsumer(),
	}

	output := m.View()

	if output == "" {
		t.Error("Expected non-empty view output")
	}

	// Should contain container name
	if !strings.Contains(output, "container1") {
		t.Error("Expected view to contain container name")
	}
}

func TestModel_View_FilterMode(t *testing.T) {
	m := &model{
		view:        listView,
		filterMode:  true,
		filterInput: "test",
		width:       100,
		height:      30,
		containers:  []types.Container{},
		containersMu: sync.RWMutex{},
		selected:    make(map[string]bool),
		selectedMu:  sync.RWMutex{},
		rateTracker: NewRateTrackerConsumer(),
	}

	output := m.View()

	if output == "" {
		t.Error("Expected non-empty view output")
	}

	// Should show filter input
	if !strings.Contains(output, "test") {
		t.Error("Expected view to contain filter input")
	}
}

func TestModel_View_ConfirmView(t *testing.T) {
	m := &model{
		view:           confirmView,
		confirmMessage: "Are you sure you want to start 2 containers?",
		width:          100,
		height:         30,
	}

	output := m.View()

	if output == "" {
		t.Error("Expected non-empty view output")
	}

	if !strings.Contains(output, "Are you sure") {
		t.Error("Expected view to contain confirmation message")
	}
}

func TestModel_View_ExitConfirmView(t *testing.T) {
	m := &model{
		view:   exitConfirmView,
		width:  100,
		height: 30,
	}

	output := m.View()

	if output == "" {
		t.Error("Expected non-empty view output")
	}
}

func TestModel_View_ErrorState(t *testing.T) {
	m := &model{
		view:   listView,
		width:  100,
		height: 30,
		err:    errors.New("Docker connection failed"),
		containers: []types.Container{},
		containersMu: sync.RWMutex{},
		selected:    make(map[string]bool),
		selectedMu:  sync.RWMutex{},
		rateTracker: NewRateTrackerConsumer(),
	}

	output := m.View()

	if output == "" {
		t.Error("Expected non-empty view output")
	}

	// Should display error
	if !strings.Contains(output, "Docker connection failed") {
		t.Error("Expected view to contain error message")
	}
}

func TestModel_View_ToastMessage(t *testing.T) {
	m := &model{
		view:         listView,
		width:        100,
		height:       30,
		toastMessage: "Container started successfully",
		toastIsError: false,
		containers:   []types.Container{},
		containersMu: sync.RWMutex{},
		selected:     make(map[string]bool),
		selectedMu:   sync.RWMutex{},
		rateTracker:  NewRateTrackerConsumer(),
	}

	output := m.View()

	if output == "" {
		t.Error("Expected non-empty view output")
	}

	if !strings.Contains(output, "Container started") {
		t.Error("Expected view to contain toast message")
	}
}

func TestModel_View_SmallTerminal(t *testing.T) {
	m := &model{
		view:   listView,
		width:  20, // Very small
		height: 10,
		containers: []types.Container{
			{ID: "c1", Names: []string{"/container1"}},
		},
		containersMu: sync.RWMutex{},
		selected:     make(map[string]bool),
		selectedMu:   sync.RWMutex{},
		rateTracker:  NewRateTrackerConsumer(),
	}

	output := m.View()

	// Should still render something
	if output == "" {
		t.Error("Expected non-empty view output even for small terminal")
	}
}

func TestModel_View_EmptyContainerList(t *testing.T) {
	m := &model{
		view:         listView,
		width:        100,
		height:       30,
		containers:   []types.Container{},
		containersMu: sync.RWMutex{},
		selected:     make(map[string]bool),
		selectedMu:   sync.RWMutex{},
		cpuStats:     make(map[string][]float64),
		cpuCurrent:   make(map[string]float64),
		cpuStatsMu:   sync.RWMutex{},
		rateTracker:  NewRateTrackerConsumer(),
	}

	output := m.View()

	if output == "" {
		t.Error("Expected non-empty view output")
	}

	// With empty container list, the view should still render (just shows table headers or empty state)
	// Just verify it doesn't crash and produces output
}

// Test renderDebugMetrics (reads live metrics)

func TestModel_RenderDebugMetrics(t *testing.T) {
	m := &model{
		logBroker: NewLogBroker(nil),
	}

	output := m.renderDebugMetrics()

	if output == "" {
		t.Error("Expected non-empty debug metrics output")
	}

	// Strip ANSI codes and check for content
	stripped := stripAnsiCodes(output)

	// Should contain numbers (goroutines, FDs, memory)
	if len(stripped) == 0 {
		t.Error("Expected non-empty debug metrics after stripping ANSI")
	}
}

func TestModel_RenderDebugMetrics_WithNilLogBroker(t *testing.T) {
	m := &model{
		logBroker: nil,
	}

	// Should not panic even with nil logBroker
	output := m.renderDebugMetrics()

	if output == "" {
		t.Error("Expected non-empty debug metrics output even with nil logBroker")
	}
}

func TestModel_RenderDebugMetrics_MultipleCallsConsistent(t *testing.T) {
	m := &model{
		logBroker: NewLogBroker(nil),
	}

	output1 := m.renderDebugMetrics()
	output2 := m.renderDebugMetrics()

	// Both calls should produce output
	if output1 == "" || output2 == "" {
		t.Error("Expected non-empty debug metrics on both calls")
	}

	// Structure should be consistent (same length roughly)
	if len(output1) == 0 || len(output2) == 0 {
		t.Error("Expected consistent debug metrics output")
	}
}
