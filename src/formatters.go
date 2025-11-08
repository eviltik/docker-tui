package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/api/types"
)

// Pre-compiled regex patterns for performance
var (
	uptimeRegex      = regexp.MustCompile(`Up (.+?)(?:\s+\(|$)`)
	exitRegex        = regexp.MustCompile(`Exited \(\d+\) (.+?) ago`)
	lessThanRegex    = regexp.MustCompile(`(?i)Less than\s+`)
	aboutMinuteRegex = regexp.MustCompile(`(?i)About a minute`)
	aboutRegex       = regexp.MustCompile(`(?i)About\s+`)
	aMinuteRegex     = regexp.MustCompile(`\ba minute\b`)
	anRegex          = regexp.MustCompile(`\ban?\s+`)
	secondsRegex     = regexp.MustCompile(`(\d+)\s*seconds?`)
	minutesRegex     = regexp.MustCompile(`(\d+)\s*minutes?`)
	hoursRegex       = regexp.MustCompile(`(\d+)\s*hours?`)
	daysRegex        = regexp.MustCompile(`(\d+)\s*days?`)
	weeksRegex       = regexp.MustCompile(`(\d+)\s*weeks?`)
)

func (m *model) formatState(c types.Container) string {
	m.processingMu.RLock()
	isProcessing := m.processing[c.ID]
	m.processingMu.RUnlock()

	if isProcessing {
		spinner := spinnerFrames[m.spinnerFrame]
		return processingStyle.Render(fmt.Sprintf("%s %-11s", spinner, "process"))
	}

	// Map states to icons and names
	var icon, stateName string
	switch c.State {
	case "running":
		icon = iconRunning
		stateName = "running"
	case "exited":
		icon = iconStopped
		stateName = "stopped"
	case "created":
		icon = iconStopped
		stateName = "created"
	case "restarting":
		icon = iconRestart
		stateName = "restart"
	case "paused":
		icon = iconPaused
		stateName = "paused"
	case "dead":
		icon = iconStopped
		stateName = "dead"
	default:
		icon = "?"
		stateName = c.State
	}

	stateText := fmt.Sprintf("%s %-11s", icon, stateName)
	if c.State == "running" {
		return runningStyle.Render(stateText)
	}
	// Gray color for stopped/non-running containers (same as uptime)
	grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	return grayStyle.Render(stateText)
}

func (m *model) formatUptime(status string, state string) string {
	// Parse uptime from status string
	uptime := ""
	isRunning := false

	// Match "Up About an hour" or "Up 2 hours" etc (use pre-compiled regex)
	uptimeMatch := uptimeRegex.FindStringSubmatch(status)
	if len(uptimeMatch) > 1 {
		uptime = uptimeMatch[1]
		isRunning = true
	} else {
		// Match "Exited (0) 5 minutes ago" (use pre-compiled regex)
		exitMatch := exitRegex.FindStringSubmatch(status)
		if len(exitMatch) > 1 {
			uptime = exitMatch[1]
		}
	}

	// Shorten uptime strings (use pre-compiled regex)
	uptime = lessThanRegex.ReplaceAllString(uptime, "<")
	uptime = aboutMinuteRegex.ReplaceAllString(uptime, "~1m")
	uptime = aboutRegex.ReplaceAllString(uptime, "~")
	uptime = aMinuteRegex.ReplaceAllString(uptime, "1m")
	uptime = anRegex.ReplaceAllString(uptime, "1")
	uptime = secondsRegex.ReplaceAllString(uptime, "${1}s")
	uptime = minutesRegex.ReplaceAllString(uptime, "${1}m")
	uptime = hoursRegex.ReplaceAllString(uptime, "${1}h")
	uptime = daysRegex.ReplaceAllString(uptime, "${1}d")
	uptime = weeksRegex.ReplaceAllString(uptime, "${1}w")

	// Apply color based on state - right-aligned with 7 chars total width
	if isRunning && state == "running" {
		return runningStyle.Render(fmt.Sprintf("%7s", uptime))
	} else if uptime != "" {
		// Gray color for stopped containers
		grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		return grayStyle.Render(fmt.Sprintf("%7s", uptime))
	}

	return fmt.Sprintf("%7s", uptime)
}

func (m *model) formatPorts(ports []types.Port) string {
	// Separate ports by IP address
	type portEntry struct {
		ip   string
		port uint16
	}

	var portEntries []portEntry
	portMap := make(map[string]bool) // To avoid duplicates

	for _, p := range ports {
		if p.PublicPort != 0 {
			ip := ""
			if p.IP != "" && p.IP != "0.0.0.0" && p.IP != "::" {
				ip = p.IP
			}

			key := fmt.Sprintf("%s:%d", ip, p.PublicPort)
			if !portMap[key] {
				portMap[key] = true
				portEntries = append(portEntries, portEntry{ip: ip, port: p.PublicPort})
			}
		}
	}

	if len(portEntries) == 0 {
		return ""
	}

	// Group by IP
	ipGroups := make(map[string][]uint16)
	for _, entry := range portEntries {
		ipGroups[entry.ip] = append(ipGroups[entry.ip], entry.port)
	}

	var result []string

	// Process each IP group
	for ip, portNums := range ipGroups {
		// Sort ports using sort.Slice
		sort.Slice(portNums, func(i, j int) bool {
			return portNums[i] < portNums[j]
		})

		// Detect ranges
		rangeStart := portNums[0]
		rangeEnd := portNums[0]

		for i := 1; i <= len(portNums); i++ {
			if i < len(portNums) && portNums[i] == rangeEnd+1 {
				// Consecutive port, extend range
				rangeEnd = portNums[i]
			} else {
				// Gap detected or end of list, finalize current range
				var portStr string
				if rangeStart == rangeEnd {
					portStr = fmt.Sprintf("%d", rangeStart)
				} else if rangeEnd == rangeStart+1 {
					// Only 2 ports, don't use range notation
					portStr = fmt.Sprintf("%d,%d", rangeStart, rangeEnd)
				} else {
					// 3+ ports, use range notation
					portStr = fmt.Sprintf("%d-%d", rangeStart, rangeEnd)
				}

				// Add IP prefix if present
				if ip != "" {
					result = append(result, fmt.Sprintf("%s:%s", ip, portStr))
				} else {
					result = append(result, portStr)
				}

				if i < len(portNums) {
					rangeStart = portNums[i]
					rangeEnd = portNums[i]
				}
			}
		}
	}

	return strings.Join(result, ",")
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// formatCPU formats CPU stats with percentage only
func (m *model) formatCPU(containerID string, state string) string {
	// Only show CPU for running containers
	if state != "running" {
		return fmt.Sprintf("%7s", "")
	}

	// Get current CPU - if not available, show N/A in red (thread-safe)
	m.cpuStatsMu.RLock()
	cpuPercent, hasCPU := m.cpuCurrent[containerID]
	m.cpuStatsMu.RUnlock()

	if !hasCPU {
		// No CPU data available (timeout or error)
		redStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f48771"))
		return redStyle.Render(fmt.Sprintf("%7s", "N/A"))
	}

	// Format: 12.3% (right-aligned for better number alignment)
	cpuText := fmt.Sprintf("%6.1f%%", cpuPercent)

	// Apply color based on CPU usage
	var style lipgloss.Style
	if cpuPercent < 30 {
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4ec9b0")) // Teal (low)
	} else if cpuPercent < 70 {
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#dcdcaa")) // Yellow (medium)
	} else {
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#f48771")) // Red (high)
	}

	return style.Render(cpuText)
}

// formatLogRate formats log rate (lines per second)
func (m *model) formatLogRate(containerID string, state string) string {
	// Only show logs rate for running containers
	if state != "running" {
		return fmt.Sprintf("%6s", "")
	}

	// Get rate from tracker
	if m.rateTracker == nil {
		return fmt.Sprintf("%6s", "0")
	}

	rate := m.rateTracker.GetRate(containerID)

	// Cap at 9999 l/s maximum display
	if rate > 9999 {
		rate = 9999
	}

	// Format according to rate (BEFORE applying color)
	var formatted string
	if rate == 0 {
		formatted = "0"
	} else if rate < 1 {
		formatted = fmt.Sprintf("%.1f", rate)
	} else if rate < 10 {
		formatted = fmt.Sprintf("%.0f", rate)
	} else if rate < 100 {
		formatted = fmt.Sprintf("%.0f", rate)
	} else if rate < 1000 {
		formatted = fmt.Sprintf("%.0f", rate)
	} else {
		// Over 1000 lines/s, show as "k"
		formatted = fmt.Sprintf("%.1fk", rate/1000)
	}

	// Pad to exactly 6 characters BEFORE applying color (right-aligned)
	formatted = fmt.Sprintf("%6s", formatted)

	// Apply color based on activity
	var style lipgloss.Style
	if rate == 0 {
		// Gray for inactive
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	} else if rate < 10 {
		// Green for low activity
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4ec9b0"))
	} else if rate < 100 {
		// Yellow for medium activity
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#dcdcaa"))
	} else {
		// Orange for high activity
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff9966"))
	}

	return style.Render(formatted)
}
