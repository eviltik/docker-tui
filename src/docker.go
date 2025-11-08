package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

func (m *model) countSelected() int {
	// CRITICAL FIX: Protect concurrent map read
	m.selectedMu.RLock()
	defer m.selectedMu.RUnlock()

	count := 0
	for _, sel := range m.selected {
		if sel {
			count++
		}
	}
	return count
}

func (m *model) getSelectedIDs() []string {
	// CRITICAL FIX: Protect concurrent map read
	m.selectedMu.RLock()
	selectedMap := make(map[string]bool, len(m.selected))
	for id, sel := range m.selected {
		selectedMap[id] = sel
	}
	hasSelection := len(m.selected) > 0
	m.selectedMu.RUnlock()

	ids := []string{}
	if hasSelection {
		// IMPORTANT: Return IDs in the same order as m.containers to maintain consistent ordering
		// Maps in Go have random iteration order, which causes logs view to show containers
		// in different order each time, even with the same selection
		m.containersMu.RLock()
		for _, c := range m.containers {
			if selectedMap[c.ID] {
				ids = append(ids, c.ID)
			}
		}
		m.containersMu.RUnlock()
	} else {
		// No selection: return current cursor container
		// CRITICAL FIX: Protect concurrent slice read
		m.containersMu.RLock()
		if m.cursor >= 0 && m.cursor < len(m.containers) {
			ids = append(ids, m.containers[m.cursor].ID)
		}
		m.containersMu.RUnlock()
	}
	return ids
}

func (m *model) performAction(action string) tea.Cmd {
	ids := m.getSelectedIDs()
	if len(ids) == 0 {
		return nil
	}

	// Return actionStartMsg to set processing state via Update()
	return func() tea.Msg {
		return actionStartMsg{action: action, ids: ids}
	}
}

// performActionAsync executes the actual Docker action in parallel goroutines
func performActionAsync(dockerClient *client.Client, action string, ids []string, containers []types.Container) tea.Cmd {
	return func() tea.Msg {
		// CRITICAL FIX: Add timeout to prevent indefinite hang on Docker daemon issues
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var errors []string
		var errorsMu sync.Mutex
		successCount := 0
		var successMu sync.Mutex

		// CRITICAL FIX: Create a safe copy of containers slice to prevent data races
		// If containerListMsg updates m.containers during iteration, goroutines could access invalid data
		containersCopy := make([]types.Container, len(containers))
		copy(containersCopy, containers)

		// Use WaitGroup for parallel execution
		var wg sync.WaitGroup

		for _, id := range ids {
			wg.Add(1)
			// Get container name for crash logging
			containerName := id[:12]
			for _, cont := range containersCopy {
				if cont.ID == id {
					if len(cont.Names) > 0 {
						containerName = strings.TrimPrefix(cont.Names[0], "/")
					}
					break
				}
			}

			safeGo(fmt.Sprintf("performAction-%s-%s", action, containerName), func() {
				containerID := id
				defer wg.Done()

				var err error

				switch action {
				case "start":
					err = dockerClient.ContainerStart(ctx, containerID, container.StartOptions{})
				case "stop":
					timeout := 10
					err = dockerClient.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})
				case "restart":
					timeout := 10
					err = dockerClient.ContainerRestart(ctx, containerID, container.StopOptions{Timeout: &timeout})
				case "remove":
					err = dockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
				case "pause":
					// Smart pause/unpause: pause running containers, unpause paused ones
					var containerState string
					for _, cont := range containersCopy { // CRITICAL FIX: Use safe copy
						if cont.ID == containerID {
							containerState = cont.State
							break
						}
					}
					if containerState == "paused" {
						err = dockerClient.ContainerUnpause(ctx, containerID)
					} else if containerState == "running" {
						err = dockerClient.ContainerPause(ctx, containerID)
					}
					// Skip containers that are not running or paused
				}

				if err != nil {
					// Get container name for error message
					containerName := containerID[:12]
					for _, cont := range containersCopy { // CRITICAL FIX: Use safe copy
						if cont.ID == containerID {
							// CRITICAL FIX: Protect against empty Names slice
							if len(cont.Names) > 0 {
								containerName = strings.TrimPrefix(cont.Names[0], "/")
							}
							break
						}
					}
					errorsMu.Lock()
					errors = append(errors, fmt.Sprintf("%s: %v", containerName, err))
					errorsMu.Unlock()
				} else {
					successMu.Lock()
					successCount++
					successMu.Unlock()
				}
			})
		}

		// Wait for all operations to complete
		wg.Wait()

		// Return appropriate toast message
		if len(errors) > 0 {
			if successCount > 0 {
				// Partial success
				return tea.Batch(
					loadContainers(dockerClient),
					func() tea.Msg {
						return toastMsg{
							message:         fmt.Sprintf("%s: %d succeeded, %d failed", action, successCount, len(errors)),
							isError:         true,
							clearProcessing: ids,
						}
					},
				)()
			} else {
				// All failed
				return tea.Batch(
					loadContainers(dockerClient),
					func() tea.Msg {
						return toastMsg{
							message:         fmt.Sprintf("%s failed: %s", action, errors[0]),
							isError:         true,
							clearProcessing: ids,
						}
					},
				)()
			}
		} else {
			// All succeeded
			return tea.Batch(
				loadContainers(dockerClient),
				func() tea.Msg {
					return toastMsg{
						message:         fmt.Sprintf("%s: %d container(s) succeeded", action, successCount),
						isError:         false,
						clearProcessing: ids,
					}
				},
			)()
		}
	}
}

// Commands
func loadContainers(cli *client.Client) tea.Cmd {
	return func() tea.Msg {
		// CRITICAL FIX: Add timeout to prevent indefinite hang on Docker daemon issues
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
		if err != nil {
			return errorMsg{err}
		}
		// Sort by name
		sort.Slice(containers, func(i, j int) bool {
			// CRITICAL FIX: Protect against empty Names slice (Docker edge case)
			nameI := ""
			if len(containers[i].Names) > 0 {
				nameI = containers[i].Names[0]
			}
			nameJ := ""
			if len(containers[j].Names) > 0 {
				nameJ = containers[j].Names[0]
			}
			return nameI < nameJ
		})
		return containerListMsg(containers)
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*5, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// CPU tick every 5 seconds
type cpuTickMsg time.Time

func cpuTickCmd() tea.Cmd {
	return tea.Tick(time.Second*5, func(t time.Time) tea.Msg {
		return cpuTickMsg(t)
	})
}

// CPU stats message
type cpuStatsMsg struct {
	stats    map[string]float64                  // Container ID -> CPU percentage
	rawStats map[string]*container.StatsResponse // Raw stats for storing
}

// fetchCPUStats fetches CPU stats for running containers (in parallel)
func fetchCPUStats(cli *client.Client, containers []types.Container, prevStats map[string]*container.StatsResponse) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		cpuStats := make(map[string]float64)
		rawStats := make(map[string]*container.StatsResponse)

		// Use channels for parallel fetching
		type statsResult struct {
			id    string
			name  string
			stats *container.StatsResponse
			err   error
		}
		resultsChan := make(chan statsResult, len(containers))

		// Launch goroutines to fetch stats in parallel
		runningCount := 0
		for _, c := range containers {
			if c.State != "running" {
				continue
			}
			// CRITICAL FIX: Protect against empty Names slice
			if len(c.Names) == 0 {
				continue
			}
			runningCount++

			// Launch goroutine for this container with crash protection
			// CRITICAL FIX: Check context before sending to prevent goroutine leak
			safeGo(fmt.Sprintf("fetchCPUStats-%s", c.Names[0]), func() {
				containerID := c.ID
				containerName := c.Names[0]
				stats, err := cli.ContainerStats(ctx, containerID, false)
				// CRITICAL FIX: Close body IMMEDIATELY after call, even on error
				// Must happen before any early returns to prevent file descriptor leak
				if stats.Body != nil {
					defer stats.Body.Close()
				}
				if err != nil {
					// Non-blocking send with context check
					select {
					case resultsChan <- statsResult{id: containerID, name: containerName, err: err}:
					case <-ctx.Done():
						return // Context cancelled, don't block
					}
					return
				}

				var v container.StatsResponse
				if err := json.NewDecoder(stats.Body).Decode(&v); err != nil {
					// Non-blocking send with context check
					select {
					case resultsChan <- statsResult{id: containerID, name: containerName, err: err}:
					case <-ctx.Done():
						return // Context cancelled, don't block
					}
					return
				}

				// Non-blocking send with context check
				select {
				case resultsChan <- statsResult{id: containerID, name: containerName, stats: &v}:
				case <-ctx.Done():
					return // Context cancelled, don't block
				}
			})
		}

		// CRITICAL FIX: Collect results with timeout to prevent goroutine leak
		// If a goroutine panics or hangs, we must not block forever
		for i := 0; i < runningCount; i++ {
			select {
			case result := <-resultsChan:
				if result.err != nil {
					// Skip containers with errors
					continue
				}

				// Store raw stats for next calculation
				rawStats[result.id] = result.stats

				// Calculate CPU percentage using OUR previous stats (not PreCPUStats)
				var cpuPercent float64
				if prev := prevStats[result.id]; prev != nil {
					cpuPercent = calculateCPUPercentWithHistory(result.stats, prev)
				}
				cpuStats[result.id] = cpuPercent

			case <-ctx.Done():
				// Context timeout - abandon remaining results to prevent hang
				// Goroutines will complete and send to buffered channel (won't leak)
				return cpuStatsMsg{stats: cpuStats, rawStats: rawStats}
			}
		}

		return cpuStatsMsg{stats: cpuStats, rawStats: rawStats}
	}
}

// calculateCPUPercentWithHistory calculates CPU percentage using our manually tracked previous stats
// (PreCPUStats in oneshot mode is unreliable - contains data from container start)
func calculateCPUPercentWithHistory(current, previous *container.StatsResponse) float64 {
	// Nil safety checks for Windows containers and initialization
	if current == nil || previous == nil {
		return 0.0
	}
	// Check if CPUUsage data is available (TotalUsage == 0 means no data)
	if current.CPUStats.CPUUsage.TotalUsage == 0 || previous.CPUStats.CPUUsage.TotalUsage == 0 {
		return 0.0
	}

	// Use our manually tracked previous stats (from ~5 seconds ago)
	cpuDelta := float64(current.CPUStats.CPUUsage.TotalUsage - previous.CPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(current.CPUStats.SystemUsage - previous.CPUStats.SystemUsage)

	// Number of CPUs available to the container
	numCPUs := float64(current.CPUStats.OnlineCPUs)
	if numCPUs == 0 {
		// Fallback to counting per-cpu usage entries
		numCPUs = float64(len(current.CPUStats.CPUUsage.PercpuUsage))
		if numCPUs == 0 {
			numCPUs = 1
		}
	}

	var result float64
	if systemDelta > 0.0 && cpuDelta >= 0.0 {
		// Docker's formula: cpuPercent = (cpuDelta / systemDelta) * numCPU * 100.0
		//
		// systemDelta: Total CPU time across ALL system CPUs during measurement period
		// cpuDelta: CPU time consumed by container during that same period
		// If systemDelta = 120 seconds and we have 24 CPUs, that means ~5 real seconds passed
		//
		// Result can exceed 100% (e.g., 250% = 2.5 cores worth of work)
		result = (cpuDelta / systemDelta) * numCPUs * 100.0

		// Cap at 999% to avoid aberrant values during Docker restarts
		if result > 999.0 {
			result = 999.0
		}
	} else {
		result = 0.0
	}

	return result
}
