package main

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type viewMode int

const (
	listView viewMode = iota
	logsView
	confirmView
	exitConfirmView
	mcpLogsView
)

// Messages
type containerListMsg []types.Container
type errorMsg struct{ err error }
type tickMsg time.Time
type toastMsg struct {
	message         string
	isError         bool
	clearProcessing []string // IDs of containers to clear from processing map
}
type actionStartMsg struct {
	action string
	ids    []string
}
type newLogLineMsg struct{} // Notifies that a new log line has arrived

// Model
type model struct {
	dockerClient   *client.Client
	containers     []types.Container
	cursor         int
	selected       map[string]bool
	processing     map[string]bool
	view           viewMode
	err            error
	width          int
	height         int
	spinnerFrame   int
	shiftStart     int
	confirmMessage string
	pendingAction  string // Action to perform after confirmation
	lastClickTime  time.Time
	lastClickIndex int
	toastMessage   string
	toastIsError   bool
	toastTimer     *time.Time                          // Track current toast timer to cancel it
	cpuStats       map[string][]float64                // CPU history per container (last 10 values)
	cpuCurrent     map[string]float64                  // Current CPU percentage per container
	cpuPrevStats   map[string]*container.StatsResponse // Previous stats for delta calculation
	filterMode     bool                                // true when typing filter
	filterInput    string                              // filter text being typed
	filterActive   string                              // currently applied filter
	filterRegex    *regexp.Regexp                      // compiled regex (nil if invalid or empty)
	filterIsRegex     bool                                // true if it's a valid regex
	demoMode          bool                                // true when launched with --demo flag
	debugMonitor      bool                                // true when launched with --debug-monitor flag
	logsBufferLength  int                                 // Maximum log lines in buffer (default 10000)

	// LogBroker architecture (permanent streaming)
	logBroker   *LogBroker           // Central broker for all logs
	rateTracker *RateTrackerConsumer // Permanent tracker for L/S column
	mcpServer   *MCPServer           // MCP server instance (nil if not running)
	cpuCache    *CPUStatsCache       // Shared CPU cache for MCP instant responses

	// BufferConsumer for logsView (temporary)
	bufferConsumer       *BufferConsumer // Buffer for logsView (nil when not in logsView)
	logsViewBuffer       []string        // Formatted buffer for display (fallback)
	logsViewScroll       int             // Scroll offset for logsView
	logsViewMaxNameWidth int             // Maximum container name width for alignment
	logsColorEnabled     bool            // True to show colored backgrounds in logs (default: true)
	newLogChan           chan struct{}   // Channel to notify new log arrivals
	logChanClosing       atomic.Bool     // Atomic flag to prevent panic on closed channel
	logChanWg            sync.WaitGroup  // WaitGroup to ensure all callbacks complete before closing channel
	logChanCloseOnce     sync.Once       // Ensures channel is closed only once (CRITICAL FIX)
	wasAtBottom          bool            // True if we were scrolled to bottom before last log arrival

	// Mutexes for concurrent access
	containersMu     sync.RWMutex // CRITICAL FIX: Protects containers slice from concurrent access
	cpuStatsMu       sync.RWMutex // Protects cpuStats, cpuCurrent, cpuPrevStats maps
	processingMu     sync.RWMutex // Protects processing map
	selectedMu       sync.RWMutex // CRITICAL FIX: Protects selected map from concurrent access
	viewTransitionMu sync.Mutex   // CRITICAL FIX: Protects view mode transitions and log channel lifecycle
}

// getContainerName safely extracts container name from a Container struct
func getContainerName(c types.Container) string {
	if len(c.Names) == 0 {
		return c.ID[:12] // Fallback to short ID
	}
	return strings.TrimPrefix(c.Names[0], "/")
}

// cleanContainerName removes prefix up to first underscore in demo mode
func (m *model) cleanContainerName(name string) string {
	if !m.demoMode {
		return name
	}
	// Find first underscore
	if idx := strings.Index(name, "_"); idx >= 0 {
		// Return everything after the first underscore
		return name[idx+1:]
	}
	return name
}

// Init
func (m *model) Init() tea.Cmd {
	// LogBroker and RateTracker are already initialized in main.go
	// Streaming will start automatically in containerListMsg after loading

	return tea.Batch(
		loadContainers(m.dockerClient),
		tickCmd(),
		cpuTickCmd(),
		logRateTickCmd(),
		cleanupTickCmd(),
		cpuCleanupTickCmd(),
	)
}

// logRateTickMsg to refresh the display of log rates
type logRateTickMsg time.Time

func logRateTickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg {
		return logRateTickMsg(t)
	})
}

// cleanupTickMsg to periodically cleanup stale containers
type cleanupTickMsg time.Time

func cleanupTickCmd() tea.Cmd {
	return tea.Tick(time.Minute*1, func(t time.Time) tea.Msg {
		return cleanupTickMsg(t)
	})
}

// cpuCleanupTickMsg to periodically cleanup stale CPU stats
type cpuCleanupTickMsg time.Time

func cpuCleanupTickCmd() tea.Cmd {
	return tea.Tick(time.Minute*1, func(t time.Time) tea.Msg {
		return cpuCleanupTickMsg(t)
	})
}

// waitForNewLog listens on the channel for new logs
func waitForNewLog(ch chan struct{}) tea.Cmd {
	return func() tea.Msg {
		<-ch
		return newLogLineMsg{}
	}
}

// Update
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Force a full redraw after resize by clearing screen
		return m, tea.ClearScreen

	case tea.MouseMsg:
		return m.handleMouseEvent(msg)

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case containerListMsg:
		// CRITICAL FIX: Protect write to m.containers with mutex
		m.containersMu.Lock()
		m.containers = []types.Container(msg)

		// CRITICAL FIX: Adjust cursor if out of bounds after container removal
		if m.cursor >= len(m.containers) && len(m.containers) > 0 {
			m.cursor = len(m.containers) - 1
		} else if len(m.containers) == 0 {
			m.cursor = 0
		}
		m.containersMu.Unlock()

		// Cleanup removed containers from maps to prevent memory leak
		currentIDs := make(map[string]bool)
		m.containersMu.RLock()
		for _, c := range m.containers {
			currentIDs[c.ID] = true
		}
		m.containersMu.RUnlock()

		// Remove entries for containers that no longer exist (thread-safe)
		m.cpuStatsMu.Lock()
		for id := range m.cpuStats {
			if !currentIDs[id] {
				delete(m.cpuStats, id)
				delete(m.cpuCurrent, id)
				delete(m.cpuPrevStats, id)
			}
		}
		m.cpuStatsMu.Unlock()

		// CRITICAL FIX: Also cleanup m.processing to prevent memory leak
		m.processingMu.Lock()
		for id := range m.processing {
			if !currentIDs[id] {
				delete(m.processing, id)
			}
		}
		m.processingMu.Unlock()

		// Update log streaming with the new container list
		// CRITICAL FIX: Read containers with mutex protection
		m.containersMu.RLock()
		containersCopy := m.containers
		m.containersMu.RUnlock()

		if m.logBroker != nil {
			m.logBroker.StartStreaming(containersCopy)
		}

		// CRITICAL FIX: Also protect read of m.cpuPrevStats with mutex (prevents race condition)
		m.cpuStatsMu.RLock()
		prevStatsCopy := make(map[string]*container.StatsResponse, len(m.cpuPrevStats))
		for k, v := range m.cpuPrevStats {
			prevStatsCopy[k] = v
		}
		m.cpuStatsMu.RUnlock()

		// Fetch CPU stats after loading containers
		return m, fetchCPUStats(m.dockerClient, containersCopy, prevStatsCopy)

	case cpuStatsMsg:
		// Update CPU stats and store raw stats for next delta calculation (thread-safe)
		m.cpuStatsMu.Lock()
		for containerID, cpuPercent := range msg.stats {
			// Update current value
			m.cpuCurrent[containerID] = cpuPercent

			// Add to history (keep last 10 values)
			history := m.cpuStats[containerID]
			history = append(history, cpuPercent)
			if len(history) > 10 {
				history = history[1:]
			}
			m.cpuStats[containerID] = history
		}

		// Store raw stats as previous for next iteration
		for containerID, rawStats := range msg.rawStats {
			m.cpuPrevStats[containerID] = rawStats
		}

		// CRITICAL: Update shared CPU cache for instant MCP responses
		// Make a copy of cpuCurrent to avoid holding the lock while updating cache
		cpuCurrentCopy := make(map[string]float64, len(m.cpuCurrent))
		for k, v := range m.cpuCurrent {
			cpuCurrentCopy[k] = v
		}
		m.cpuStatsMu.Unlock()

		// Update cache outside of lock to avoid potential deadlock
		if m.cpuCache != nil {
			m.cpuCache.Update(cpuCurrentCopy)
		}

		return m, nil

	case tickMsg:
		m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
		return m, tea.Batch(
			loadContainers(m.dockerClient),
			tickCmd(),
		)

	case cpuTickMsg:
		// CRITICAL FIX: Protect read of m.containers with mutex to prevent race condition
		m.containersMu.RLock()
		containersCopy := make([]types.Container, len(m.containers))
		copy(containersCopy, m.containers)
		m.containersMu.RUnlock()

		// CRITICAL FIX: Also protect read of m.cpuPrevStats with mutex
		m.cpuStatsMu.RLock()
		prevStatsCopy := make(map[string]*container.StatsResponse, len(m.cpuPrevStats))
		for k, v := range m.cpuPrevStats {
			prevStatsCopy[k] = v
		}
		m.cpuStatsMu.RUnlock()

		// Refresh CPU stats every 5 seconds with safe copies
		return m, tea.Batch(
			fetchCPUStats(m.dockerClient, containersCopy, prevStatsCopy),
			cpuTickCmd(),
		)

	case logRateTickMsg:
		return m, logRateTickCmd()

	case cleanupTickMsg:
		// Cleanup stale containers from RateTracker
		if m.rateTracker != nil {
			m.rateTracker.CleanupStaleContainers()
		}
		return m, cleanupTickCmd()

	case cpuCleanupTickMsg:
		// CRITICAL FIX: Comprehensive cleanup across all CPU maps to prevent memory leaks
		// Build current container IDs with mutex protection
		currentIDs := make(map[string]bool)
		m.containersMu.RLock()
		for _, c := range m.containers {
			currentIDs[c.ID] = true
		}
		m.containersMu.RUnlock()

		// Collect stale IDs from ALL maps (not just cpuPrevStats)
		m.cpuStatsMu.Lock()
		toDelete := make(map[string]bool)

		// Check all three maps for stale entries
		for id := range m.cpuPrevStats {
			if !currentIDs[id] {
				toDelete[id] = true
			}
		}
		for id := range m.cpuStats {
			if !currentIDs[id] {
				toDelete[id] = true
			}
		}
		for id := range m.cpuCurrent {
			if !currentIDs[id] {
				toDelete[id] = true
			}
		}

		// Delete from all maps
		for id := range toDelete {
			delete(m.cpuStats, id)
			delete(m.cpuCurrent, id)
			delete(m.cpuPrevStats, id)
		}
		m.cpuStatsMu.Unlock()

		return m, cpuCleanupTickCmd()

	case actionStartMsg:
		// Set processing state for all containers in this action (thread-safe)
		m.processingMu.Lock()
		for _, id := range msg.ids {
			m.processing[id] = true
		}
		m.processingMu.Unlock()
		// Now trigger the actual action
		return m, performActionAsync(m.dockerClient, msg.action, msg.ids, m.containers)

	case toastMsg:
		// Clear processing state for containers (thread-safe)
		m.processingMu.Lock()
		for _, id := range msg.clearProcessing {
			delete(m.processing, id)
		}
		m.processingMu.Unlock()

		// Check if this is a clear message for an old timer
		if msg.message == "" && m.toastTimer != nil {
			// Only clear if this timer matches the current one
			now := time.Now()
			if now.Sub(*m.toastTimer) >= time.Second*3 {
				m.toastMessage = ""
				m.toastIsError = false
				m.toastTimer = nil
			}
			return m, nil
		}

		// New toast message
		m.toastMessage = msg.message
		m.toastIsError = msg.isError

		// Set timer for this toast
		now := time.Now()
		m.toastTimer = &now

		// Auto-clear toast after 3 seconds
		return m, tea.Tick(time.Second*3, func(t time.Time) tea.Msg {
			return toastMsg{message: "", isError: false}
		})

	case errorMsg:
		m.err = msg.err
		return m, nil

	case newLogLineMsg:
		// When a new log arrives, check if we were scrolled to bottom
		// If yes, stay at bottom after adding the new line
		if m.view == logsView && m.bufferConsumer != nil {
			if m.wasAtBottom {
				logCount := m.getFilteredLogCount()
				maxScroll := max(0, logCount-(m.height-5))
				m.logsViewScroll = maxScroll
			}

			// Continue listening for the next log
			return m, waitForNewLog(m.newLogChan)
		}
		return m, nil
	}

	return m, nil
}

// Filter helpers

// compileFilter compiles the filter input as a regex (only for listView)
// In logsView, regex is never used (always substring search)
func (m *model) compileFilter(input string) {
	if input == "" {
		m.filterRegex = nil
		m.filterIsRegex = false
		return
	}

	// Skip regex compilation in logsView (not used for log filtering)
	if m.view == logsView {
		m.filterRegex = nil
		m.filterIsRegex = false
		return
	}

	// Try to compile as case-insensitive regex for container name filtering
	re, err := regexp.Compile("(?i)" + input)
	if err != nil {
		m.filterRegex = nil
		m.filterIsRegex = false
	} else {
		m.filterRegex = re
		m.filterIsRegex = true
	}
}

// containerMatchesFilter checks if a container matches the active filter
func (m *model) containerMatchesFilter(c types.Container) bool {
	if m.filterActive == "" {
		return true
	}

	name := getContainerName(c)

	if m.filterIsRegex && m.filterRegex != nil {
		return m.filterRegex.MatchString(name)
	}

	// Fallback: case-insensitive substring search
	return strings.Contains(strings.ToLower(name), strings.ToLower(m.filterActive))
}

// stripAnsiCodes removes ANSI escape sequences from a string
func stripAnsiCodes(s string) string {
	// Match ANSI escape sequences: ESC [ ... m (and other variants)
	// This regex matches: \x1b\[ followed by any characters until 'm'
	// Also matches \x1b] and other escape sequences
	var result strings.Builder
	inEscape := false
	escapeStart := false

	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			inEscape = true
			escapeStart = true
			continue
		}

		if inEscape {
			if escapeStart && (s[i] == '[' || s[i] == ']' || s[i] == '(') {
				escapeStart = false
				continue
			}
			escapeStart = false

			// End of escape sequence (common terminators)
			if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') || s[i] == '~' {
				inEscape = false
				continue
			}
			continue
		}

		result.WriteByte(s[i])
	}

	return result.String()
}

// logLineMatchesFilter checks if a log line matches the active filter
func (m *model) logLineMatchesFilter(line string) bool {
	if m.filterActive == "" {
		return true
	}

	// Strip ANSI codes before searching to avoid false negatives/positives
	cleanLine := stripAnsiCodes(line)

	// ALWAYS use case-insensitive substring search for log filtering
	// (regex matching can be confusing for users who just want to search text)
	return strings.Contains(strings.ToLower(cleanLine), strings.ToLower(m.filterActive))
}

// updateWasAtBottom updates the wasAtBottom flag based on scroll position
func (m *model) updateWasAtBottom() {
	if m.view != logsView || m.bufferConsumer == nil {
		m.wasAtBottom = false
		return
	}

	logCount := m.getFilteredLogCount()
	maxScroll := max(0, logCount-(m.height-5))

	// We are at bottom if scroll >= maxScroll - 2 (2 lines tolerance)
	m.wasAtBottom = m.logsViewScroll >= maxScroll-2
}

// getFilteredLogCount returns the number of logs after filtering
func (m *model) getFilteredLogCount() int {
	var rawLogs []string

	if m.bufferConsumer != nil {
		entries := m.bufferConsumer.GetBuffer()
		rawLogs = make([]string, len(entries))
		for i, entry := range entries {
			displayName := m.cleanContainerName(entry.ContainerName)
			rawLogs[i] = fmt.Sprintf("[%s] %s", displayName, entry.Line)
		}
	} else {
		rawLogs = m.logsViewBuffer
	}

	// If no filter, return raw count
	if m.filterActive == "" {
		return len(rawLogs)
	}

	// Count filtered logs
	count := 0
	for _, line := range rawLogs {
		if m.logLineMatchesFilter(line) {
			count++
		}
	}
	return count
}

// View
func (m *model) View() string {
	switch m.view {
	case confirmView, exitConfirmView:
		return m.renderConfirm()
	case logsView:
		return m.renderLogs()
	case mcpLogsView:
		return m.renderMCPLogs()
	default:
		return m.renderList()
	}
}
