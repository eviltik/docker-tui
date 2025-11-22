package main

import (
	"sync"
	"time"

	"github.com/docker/docker/client"
)

// CPUStatsCache caches CPU stats with automatic refresh
type CPUStatsCache struct {
	dockerClient *client.Client
	mu           sync.RWMutex
	cpuCurrent   map[string]float64 // containerID -> CPU%
	lastRefresh  time.Time
	refreshRate  time.Duration
}

// NewCPUStatsCache creates a new CPU stats cache
func NewCPUStatsCache(dockerClient *client.Client, refreshRate time.Duration) *CPUStatsCache {
	return &CPUStatsCache{
		dockerClient: dockerClient,
		cpuCurrent:   make(map[string]float64),
		refreshRate:  refreshRate,
	}
}

// Update updates the cache with new CPU values (called by model when it receives cpuStatsMsg)
func (c *CPUStatsCache) Update(cpuValues map[string]float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Replace entire map with new values
	c.cpuCurrent = make(map[string]float64, len(cpuValues))
	for k, v := range cpuValues {
		c.cpuCurrent[k] = v
	}
	c.lastRefresh = time.Now()
}

// Get returns cached CPU stats (non-blocking, instant response)
func (c *CPUStatsCache) Get() map[string]float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[string]float64, len(c.cpuCurrent))
	for k, v := range c.cpuCurrent {
		result[k] = v
	}
	return result
}

// GetForContainer returns cached CPU for a specific container
func (c *CPUStatsCache) GetForContainer(containerID string) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cpuCurrent[containerID]
}

// GetLastRefresh returns when the cache was last refreshed
func (c *CPUStatsCache) GetLastRefresh() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastRefresh
}
