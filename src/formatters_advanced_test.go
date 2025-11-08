package main

import (
	"strings"
	"testing"
	"time"

)

// Test formatLogRate method edge cases (it's a model method, needs rateTracker)

func TestModel_FormatLogRate_NilRateTracker(t *testing.T) {
	m := &model{
		rateTracker: nil,
	}

	result := m.formatLogRate("c1", "running")
	stripped := stripAnsiCodes(result)
	stripped = strings.TrimSpace(stripped)

	if stripped != "0" {
		t.Errorf("Expected '0' when rateTracker is nil, got '%s'", stripped)
	}
}

func TestModel_FormatLogRate_StoppedContainer(t *testing.T) {
	m := &model{
		rateTracker: NewRateTrackerConsumer(),
	}

	result := m.formatLogRate("c1", "exited")
	// Should return empty/spaces for stopped container
	if result == "" {
		t.Error("Expected non-empty result (padding)")
	}
}

func TestModel_FormatLogRate_HighRate(t *testing.T) {
	m := &model{
		rateTracker: NewRateTrackerConsumer(),
	}

	// Add many log lines to simulate high rate via OnLogLine
	for i := 0; i < 15000; i++ {
		m.rateTracker.OnLogLine("c1", "container1", "log line", time.Now())
	}

	result := m.formatLogRate("c1", "running")
	if result == "" {
		t.Error("Expected non-empty result for high rate")
	}
}

// Test max/min helper functions

func TestMaxHelper(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{5, 3, 5},
		{3, 5, 5},
		{5, 5, 5},
		{-5, 3, 3},
		{-5, -3, -3},
		{0, 0, 0},
	}

	for _, tt := range tests {
		got := max(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("max(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestMinHelper(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{5, 3, 3},
		{3, 5, 3},
		{5, 5, 5},
		{-5, 3, -5},
		{-5, -3, -5},
		{0, 0, 0},
	}

	for _, tt := range tests {
		got := min(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("min(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

// Test formatUptime additional edge cases (it's a model method)

func TestModel_FormatUptime_ExitedRecently(t *testing.T) {
	m := &model{}
	result := m.formatUptime("Exited (0) 30 seconds ago", "exited")

	if result == "" {
		t.Error("Expected non-empty result for recently exited")
	}
}

func TestModel_FormatUptime_VeryLongStatus(t *testing.T) {
	m := &model{}
	result := m.formatUptime("Up 365 days and 12 hours and 30 minutes", "running")

	if result == "" {
		t.Error("Expected non-empty result for very long status")
	}
}
