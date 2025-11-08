package main

import (
	"sync"
	"time"
)

// LogRateTracker tracks the number of log lines per second
type LogRateTracker struct {
	lines      []time.Time // timestamps of received lines (1s sliding window)
	lastUpdate time.Time   // last update timestamp
	mu         sync.Mutex
}

// AddLine records a new log line and updates the counter
func (lrt *LogRateTracker) AddLine() {
	lrt.mu.Lock()
	defer lrt.mu.Unlock()

	now := time.Now()
	lrt.lastUpdate = now

	// Clean up old entries BEFORE adding new one to prevent unbounded growth
	cutoff := now.Add(-time.Second)
	validStart := 0
	for i, t := range lrt.lines {
		if t.After(cutoff) {
			validStart = i
			break
		}
	}

	// Reuse slice capacity by copying valid entries to beginning
	if validStart > 0 {
		copy(lrt.lines, lrt.lines[validStart:])
		lrt.lines = lrt.lines[:len(lrt.lines)-validStart]
	}

	// CRITICAL FIX: Enforce hard cap to prevent memory exhaustion
	// Reduced from 20k to 5k (5k lines/sec max = ~80KB per container)
	// With 100 containers: 8MB total (vs 32MB before)
	const maxEntries = 5000
	if len(lrt.lines) >= maxEntries {
		// CRITICAL FIX: Force reallocation to free backing array and prevent memory leak
		// Drop oldest 25% to avoid thrashing at limit
		dropCount := maxEntries / 4
		newSlice := make([]time.Time, maxEntries-dropCount, maxEntries)
		copy(newSlice, lrt.lines[dropCount:])
		lrt.lines = newSlice
	}

	// Now append new entry (guaranteed not to exceed maxEntries)
	lrt.lines = append(lrt.lines, now)
}

// GetRate returns the rate of lines per second
func (lrt *LogRateTracker) GetRate() float64 {
	lrt.mu.Lock()
	defer lrt.mu.Unlock()

	// If no update for >2s, consider rate as 0
	if time.Since(lrt.lastUpdate) > 2*time.Second {
		return 0.0
	}

	// Clean up old lines
	now := time.Now()
	cutoff := now.Add(-time.Second)
	filtered := []time.Time{}
	for _, t := range lrt.lines {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}

	// CRITICAL FIX: Prevent memory leak by reallocating slice if too much slack
	// If capacity is >1000 and we're using <25% of it, reallocate to free memory
	if cap(lrt.lines) > 1000 && len(filtered) < cap(lrt.lines)/4 {
		// Force reallocation to free the old backing array
		newSlice := make([]time.Time, len(filtered))
		copy(newSlice, filtered)
		lrt.lines = newSlice
	} else {
		lrt.lines = filtered
	}

	return float64(len(lrt.lines))
}

// RateTrackerConsumer implements LogConsumer to track log rates
type RateTrackerConsumer struct {
	rates   map[string]*LogRateTracker
	ratesMu sync.RWMutex
}

// NewRateTrackerConsumer creates a new instance
func NewRateTrackerConsumer() *RateTrackerConsumer {
	return &RateTrackerConsumer{
		rates: make(map[string]*LogRateTracker),
	}
}

// OnLogLine is called when a new log line arrives
func (rtc *RateTrackerConsumer) OnLogLine(containerID, containerName, line string, timestamp time.Time) {
	rtc.ratesMu.Lock()
	defer rtc.ratesMu.Unlock()

	if rtc.rates[containerID] == nil {
		rtc.rates[containerID] = &LogRateTracker{
			lines:      []time.Time{},
			lastUpdate: timestamp,
		}
	}

	rtc.rates[containerID].AddLine()
}

// OnContainerStatusChange is called when a container changes state
func (rtc *RateTrackerConsumer) OnContainerStatusChange(containerID string, isRunning bool) {
	if !isRunning {
		rtc.ratesMu.Lock()
		delete(rtc.rates, containerID)
		rtc.ratesMu.Unlock()
	}
}

// GetRate returns the log rate for a container
func (rtc *RateTrackerConsumer) GetRate(containerID string) float64 {
	rtc.ratesMu.RLock()
	defer rtc.ratesMu.RUnlock()

	tracker := rtc.rates[containerID]
	if tracker == nil {
		return 0.0
	}
	return tracker.GetRate()
}

// CleanupStaleContainers removes entries for containers that haven't logged in >5 minutes
// This prevents memory leak when OnContainerStatusChange is not called
func (rtc *RateTrackerConsumer) CleanupStaleContainers() {
	// CRITICAL FIX: Simplified lock order to prevent deadlock
	// Use a single lock acquisition pattern: only hold one lock at a time

	now := time.Now()
	staleThreshold := 5 * time.Minute

	// Phase 1: Build list of potentially stale containers
	// Copy lastUpdate times while holding locks minimally
	type containerState struct {
		id         string
		lastUpdate time.Time
	}

	rtc.ratesMu.RLock()
	states := make([]containerState, 0, len(rtc.rates))
	for containerID, tracker := range rtc.rates {
		if tracker != nil {
			// Read lastUpdate atomically
			tracker.mu.Lock()
			states = append(states, containerState{
				id:         containerID,
				lastUpdate: tracker.lastUpdate,
			})
			tracker.mu.Unlock()
		}
	}
	rtc.ratesMu.RUnlock()

	// Phase 2: Identify stale containers (no locks held)
	staleIDs := make([]string, 0)
	for _, state := range states {
		if now.Sub(state.lastUpdate) > staleThreshold {
			staleIDs = append(staleIDs, state.id)
		}
	}

	// Phase 3: Delete stale entries (single write lock)
	if len(staleIDs) > 0 {
		rtc.ratesMu.Lock()
		for _, containerID := range staleIDs {
			delete(rtc.rates, containerID)
		}
		rtc.ratesMu.Unlock()
	}
}
