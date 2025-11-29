package main

import (
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// handleMouseEvent handles mouse events (clicks, scrolling)
func (m *model) handleMouseEvent(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Handle mouse wheel in logs view
	if m.view == logsView {
		// Get current log count from buffer
		var logCount int
		if m.bufferConsumer != nil {
			logCount = len(m.bufferConsumer.GetBuffer())
		} else {
			logCount = len(m.logsViewBuffer)
		}

		switch msg.Type {
		case tea.MouseWheelUp:
			if m.logsViewScroll > 0 {
				m.logsViewScroll -= 3
				if m.logsViewScroll < 0 {
					m.logsViewScroll = 0
				}
			}
			m.updateWasAtBottom()
		case tea.MouseWheelDown:
			maxScroll := max(0, logCount-(m.height-5))
			if m.logsViewScroll < maxScroll {
				m.logsViewScroll += 3
				if m.logsViewScroll > maxScroll {
					m.logsViewScroll = maxScroll
				}
			}
			m.updateWasAtBottom()
		}
		return m, nil
	}

	// Handle mouse in list view
	if m.view != listView {
		return m, nil
	}

	switch msg.Type {
	case tea.MouseLeft:
		// Calculate which container was clicked
		// Layout: title line + blank + box border + header + separator = 5 lines before first container
		// Box has: top border (1) + header (1) + separator (1) = 3 lines
		clickedLine := msg.Y

		// Calculate dynamic offset:
		// Line 0: title+stats
		// Line 1: blank
		// Line 2: box top border
		// Line 3: header line
		// Line 4: separator
		// Line 5+: containers (inside box)
		headerOffset := 5

		if clickedLine >= headerOffset && len(m.containers) > 0 {
			clickedIndex := clickedLine - headerOffset
			// CRITICAL FIX: Protect read of m.containers with mutex and bounds check
			m.containersMu.RLock()
			containersLen := len(m.containers)
			if clickedIndex >= 0 && clickedIndex < containersLen {
				// Check for double-click (within 500ms)
				now := time.Now()
				isDoubleClick := clickedIndex == m.lastClickIndex &&
					now.Sub(m.lastClickTime) < 500*time.Millisecond

				m.lastClickTime = now
				m.lastClickIndex = clickedIndex

				if isDoubleClick {
					// Extract container data while holding lock
					containerID := m.containers[clickedIndex].ID
					var containerName string
					if len(m.containers[clickedIndex].Names) > 0 {
						containerName = strings.TrimPrefix(m.containers[clickedIndex].Names[0], "/")
					} else {
						containerName = containerID[:12] // Fallback to short ID
					}
					m.containersMu.RUnlock()

					// CRITICAL FIX: Protect entire view transition with mutex (double-click path)
					m.viewTransitionMu.Lock()

					// Double-click: show logs for clicked container
					m.cursor = clickedIndex
					// CRITICAL FIX: Protect concurrent map write
					m.selectedMu.Lock()
					m.selected = make(map[string]bool)
					m.selected[containerID] = true
					m.selectedMu.Unlock()
					// Calculate max container name width (only one container, use cleaned name)
					name := m.cleanContainerName(containerName) // Apply demo mode cleaning
					m.logsViewMaxNameWidth = len(name)
					m.view = logsView
					m.logsViewScroll = 0
					m.logsViewBuffer = []string{} // Reset buffer

					// CRITICAL FIX: Cleanup old BufferConsumer if it exists (prevent leak)
					if m.bufferConsumer != nil {
						m.logBroker.UnregisterConsumer(m.bufferConsumer)
						m.bufferConsumer = nil
					}

					// CRITICAL FIX: Create NEW sync.Once instance when entering logs view (double-click)
					m.logChanCloseOnce = sync.Once{}
					m.logChanClosing.Store(false)

					// Create channel for new log notifications
					m.newLogChan = make(chan struct{}, 100)

					// Create and register BufferConsumer with callback
					m.bufferConsumer = NewBufferConsumer([]string{containerID}, m.logsBufferLength, func(entry LogEntry) {
						// Notify via channel (non-blocking)
						select {
						case m.newLogChan <- struct{}{}:
						default:
							// Channel full, ignore
						}
					}, &m.logChanClosing, &m.logChanWg)

					// Pre-load existing logs
					recentLogs := m.logBroker.FetchRecentLogs([]string{containerID}, "100")
					containerNames := make(map[string]string)
					containerNames[containerID] = containerName
					// Pass container ID slice to maintain stable ordering
					m.bufferConsumer.PreloadLogs([]string{containerID}, recentLogs, containerNames)

					// Register consumer AFTER preloading
					m.logBroker.RegisterConsumer(m.bufferConsumer)

					// Auto-scroll to bottom
					m.logsViewScroll = max(0, m.getFilteredLogCount()-(m.height-5))

					// We start at bottom, so wasAtBottom = true and not paused
					m.wasAtBottom = true
					m.logsViewPaused = false

					m.viewTransitionMu.Unlock()

					// Start listening for new logs
					return m, waitForNewLog(m.newLogChan)
				} else {
					// Single click: move cursor and toggle selection
					// Extract container ID while still holding lock
					containerID := m.containers[clickedIndex].ID
					m.containersMu.RUnlock()

					m.cursor = clickedIndex
					m.shiftStart = -1

					// Toggle selection
					m.selectedMu.Lock()
					m.selected[containerID] = !m.selected[containerID]
					m.selectedMu.Unlock()
				}
			} else {
				m.containersMu.RUnlock()
			}
		}

	case tea.MouseWheelUp:
		// Scroll up
		if m.cursor > 0 {
			m.cursor--
			m.shiftStart = -1
		}

	case tea.MouseWheelDown:
		// Scroll down
		// CRITICAL FIX: Protect read of len(m.containers) with mutex
		m.containersMu.RLock()
		containersLen := len(m.containers)
		m.containersMu.RUnlock()
		if m.cursor < containersLen-1 {
			m.cursor++
			m.shiftStart = -1
		}
	}

	return m, nil
}
