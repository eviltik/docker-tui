package main

import (
	"fmt"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

// handleListViewKeys handles keyboard input in container list view
func (m *model) handleListViewKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	// Handle Shift+Up/Down for range selection FIRST
	case "shift+up", "shift+k":
		// CRITICAL FIX: Protect read of m.containers during range selection
		m.containersMu.RLock()
		containersLen := len(m.containers)
		if m.cursor > 0 && containersLen > 0 {
			// Start shift selection if not already started
			if m.shiftStart == -1 {
				m.shiftStart = m.cursor
			}
			// Move cursor
			m.cursor--
			// Calculate range
			start := min(m.shiftStart, m.cursor)
			end := max(m.shiftStart, m.cursor)

			// Collect IDs to select (while holding containers lock)
			idsToSelect := make([]string, 0, end-start+1)
			for i := start; i <= end; i++ {
				if i >= 0 && i < containersLen {
					idsToSelect = append(idsToSelect, m.containers[i].ID)
				}
			}
			m.containersMu.RUnlock()

			// Now update selection map (separate lock)
			m.selectedMu.Lock()
			m.selected = make(map[string]bool)
			for _, id := range idsToSelect {
				m.selected[id] = true
			}
			m.selectedMu.Unlock()
		} else {
			m.containersMu.RUnlock()
		}
		return m, nil

	case "shift+down", "shift+j":
		// CRITICAL FIX: Protect read of m.containers during range selection
		m.containersMu.RLock()
		containersLen := len(m.containers)
		if m.cursor < containersLen-1 && containersLen > 0 {
			// Start shift selection if not already started
			if m.shiftStart == -1 {
				m.shiftStart = m.cursor
			}
			// Move cursor
			m.cursor++
			// Calculate range
			start := min(m.shiftStart, m.cursor)
			end := max(m.shiftStart, m.cursor)

			// Collect IDs to select (while holding containers lock)
			idsToSelect := make([]string, 0, end-start+1)
			for i := start; i <= end; i++ {
				if i >= 0 && i < containersLen {
					idsToSelect = append(idsToSelect, m.containers[i].ID)
				}
			}
			m.containersMu.RUnlock()

			// Now update selection map (separate lock)
			m.selectedMu.Lock()
			m.selected = make(map[string]bool)
			for _, id := range idsToSelect {
				m.selected[id] = true
			}
			m.selectedMu.Unlock()
		} else {
			m.containersMu.RUnlock()
		}
		return m, nil

	case "ctrl+c":
		return m, tea.Quit

	case "esc", "q", "Q":
		// If filter is active, clear it instead of exiting
		if m.filterActive != "" {
			m.filterActive = ""
			m.filterInput = ""
			m.filterRegex = nil
			m.filterIsRegex = false
			return m, nil
		}

		// Show exit confirmation
		m.confirmMessage = "Are you sure you want to quit?\n\nPress Y to confirm, N to cancel"
		m.view = exitConfirmView
		return m, nil

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.shiftStart = -1
		}

	case "down", "j":
		m.containersMu.RLock()
		containersLen := len(m.containers)
		m.containersMu.RUnlock()
		if m.cursor < containersLen-1 {
			m.cursor++
			m.shiftStart = -1
		}

	case "pgup":
		m.cursor = max(0, m.cursor-10)
		m.shiftStart = -1

	case "pgdown":
		m.containersMu.RLock()
		containersLen := len(m.containers)
		m.containersMu.RUnlock()
		m.cursor = min(containersLen-1, m.cursor+10)
		m.shiftStart = -1

	case "home":
		m.cursor = 0
		m.shiftStart = -1

	case "end":
		m.containersMu.RLock()
		containersLen := len(m.containers)
		m.containersMu.RUnlock()
		if containersLen > 0 {
			m.cursor = containersLen - 1
		}
		m.shiftStart = -1

	case " ":
		m.containersMu.RLock()
		containersLen := len(m.containers)
		var containerID string
		if m.cursor >= 0 && m.cursor < containersLen {
			containerID = m.containers[m.cursor].ID
		}
		m.containersMu.RUnlock()

		if containerID != "" {
			// CRITICAL FIX: Protect concurrent map write
			m.selectedMu.Lock()
			m.selected[containerID] = !m.selected[containerID]
			m.selectedMu.Unlock()
		}

	case "x", "X":
		// CRITICAL FIX: Protect concurrent map write
		m.selectedMu.Lock()
		// Clear all selections
		m.selected = make(map[string]bool)
		m.selectedMu.Unlock()

	case "a", "A":
		// CRITICAL FIX: Protect concurrent read/write
		m.containersMu.RLock()
		allIDs := make([]string, 0, len(m.containers))
		for _, c := range m.containers {
			allIDs = append(allIDs, c.ID)
		}
		m.containersMu.RUnlock()

		m.selectedMu.Lock()
		m.selected = make(map[string]bool)
		for _, id := range allIDs {
			m.selected[id] = true
		}
		m.selectedMu.Unlock()

	case "ctrl+a":
		// CRITICAL FIX: Protect concurrent read/write
		m.containersMu.RLock()
		runningIDs := make([]string, 0)
		for _, c := range m.containers {
			if c.State == "running" {
				runningIDs = append(runningIDs, c.ID)
			}
		}
		m.containersMu.RUnlock()

		m.selectedMu.Lock()
		m.selected = make(map[string]bool)
		for _, id := range runningIDs {
			m.selected[id] = true
		}
		m.selectedMu.Unlock()

	case "i", "I":
		// CRITICAL FIX: Protect concurrent read/write
		m.containersMu.RLock()
		allIDs := make([]string, 0, len(m.containers))
		for _, c := range m.containers {
			allIDs = append(allIDs, c.ID)
		}
		m.containersMu.RUnlock()

		m.selectedMu.Lock()
		for _, id := range allIDs {
			m.selected[id] = !m.selected[id]
		}
		m.selectedMu.Unlock()

	case "/":
		// Enter filter mode
		m.filterMode = true
		m.filterInput = m.filterActive // Start with current filter
		return m, nil

	case "enter", "l", "L":
		selected := m.getSelectedIDs()
		if len(selected) > 0 {
			// CRITICAL FIX: Protect entire view transition with mutex
			m.viewTransitionMu.Lock()

			// Calculate max container name width for log alignment (use cleaned names)
			// CRITICAL FIX: Protect read of m.containers
			maxWidth := 0
			m.containersMu.RLock()
			for _, id := range selected {
				for _, c := range m.containers {
					if c.ID == id {
						name := getContainerName(c)
						name = m.cleanContainerName(name) // Apply demo mode cleaning
						if len(name) > maxWidth {
							maxWidth = len(name)
						}
						break
					}
				}
			}
			m.containersMu.RUnlock()
			m.logsViewMaxNameWidth = maxWidth
			m.view = logsView
			m.logsViewScroll = 0
			m.logsViewBuffer = []string{} // Reset buffer

			// CRITICAL FIX: Create NEW sync.Once instance when entering logs view
			m.logChanCloseOnce = sync.Once{}
			m.logChanClosing.Store(false)

			// Create channel for new log notifications
			m.newLogChan = make(chan struct{}, 100) // Buffer of 100 to avoid blocking

			// Create and register BufferConsumer with callback
			m.bufferConsumer = NewBufferConsumer(selected, m.logsBufferLength, func(entry LogEntry) {
				// Notify via channel (non-blocking)
				select {
				case m.newLogChan <- struct{}{}:
				default:
					// Channel full, ignore (avoids blocking)
				}
			}, &m.logChanClosing, &m.logChanWg)

			// Pre-load existing logs
			recentLogs := m.logBroker.FetchRecentLogs(selected, "100")
			containerNames := make(map[string]string)
			// CRITICAL FIX: Protect read of m.containers
			m.containersMu.RLock()
			for _, id := range selected {
				for _, c := range m.containers {
					if c.ID == id {
						containerNames[id] = getContainerName(c)
						break
					}
				}
			}
			m.containersMu.RUnlock()
			// Pass selected IDs to maintain stable ordering
			m.bufferConsumer.PreloadLogs(selected, recentLogs, containerNames)

			// Register consumer AFTER preloading to avoid duplicates
			m.logBroker.RegisterConsumer(m.bufferConsumer)

			// Auto-scroll to bottom
			m.logsViewScroll = max(0, m.getFilteredLogCount()-(m.height-5))

			// We start at bottom, so wasAtBottom = true
			m.wasAtBottom = true

			m.viewTransitionMu.Unlock()

			// Start listening for new logs
			return m, waitForNewLog(m.newLogChan)
		}

	case "s", "S":
		if m.countSelected() > 1 {
			selected := m.getSelectedIDs()
			m.showActionConfirmation("start", selected)
			return m, nil
		}
		return m, m.performAction("start")

	case "p", "P":
		if m.countSelected() > 1 {
			selected := m.getSelectedIDs()
			m.showActionConfirmation("stop", selected)
			return m, nil
		}
		return m, m.performAction("stop")

	case "r", "R":
		if m.countSelected() > 1 {
			selected := m.getSelectedIDs()
			m.showActionConfirmation("restart", selected)
			return m, nil
		}
		return m, m.performAction("restart")

	case "u", "U":
		if m.countSelected() > 1 {
			selected := m.getSelectedIDs()
			m.showActionConfirmation("pause", selected)
			return m, nil
		}
		return m, m.performAction("pause")

	case "d", "D":
		selected := m.getSelectedIDs()
		if len(selected) == 0 {
			m.containersMu.RLock()
			if m.cursor >= 0 && m.cursor < len(m.containers) {
				selected = []string{m.containers[m.cursor].ID}
			}
			m.containersMu.RUnlock()
		}
		if len(selected) > 0 {
			names := []string{}
			// CRITICAL FIX: Protect read of m.containers
			m.containersMu.RLock()
			for _, id := range selected {
				for _, c := range m.containers {
					if c.ID == id {
						names = append(names, getContainerName(c))
						break
					}
				}
			}
			m.containersMu.RUnlock()
			m.confirmMessage = fmt.Sprintf("Remove %d container(s)?\n\n%s\n\n⚠ This action cannot be undone!\n\nPress Y to confirm, N to cancel",
				len(selected), strings.Join(names, "\n"))
			m.pendingAction = "remove"
			m.view = confirmView
		}
	}

	return m, nil
}
