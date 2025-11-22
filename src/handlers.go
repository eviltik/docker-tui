package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// MAINTAINABILITY NOTE: This file acts as a router to view-specific handlers.
// Individual view logic is in:
//   - handlers_filter.go  (filter mode)
//   - handlers_logs.go    (logs view)
//   - handlers_list.go    (list view)
//   - handlers_confirm.go (confirmation dialogs)
//   - handlers_mouse.go   (mouse events)

// showActionConfirmation displays a confirmation dialog for multi-container operations
func (m *model) showActionConfirmation(action string, selected []string) {
	names := []string{}
	// CRITICAL FIX: Protect read of m.containers with mutex
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
	// Capitalize first letter for display
	actionDisplay := strings.ToUpper(action[:1]) + action[1:]
	m.confirmMessage = fmt.Sprintf("%s %d container(s)?\n\n%s\n\nPress Y to confirm, N to cancel",
		actionDisplay, len(selected), strings.Join(names, "\n"))
	m.pendingAction = action
	m.view = confirmView
}

func (m *model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle filter mode first (intercept all keys)
	if m.filterMode {
		return m.handleFilterMode(msg)
	}

	switch m.view {
	case exitConfirmView:
		return m.handleExitConfirmKeys(msg)

	case confirmView:
		return m.handleConfirmViewKeys(msg)

	case logsView:
		return m.handleLogsViewKeys(msg)

	case mcpLogsView:
		return m.handleMCPLogsViewKeys(msg)

	case listView:
		return m.handleListViewKeys(msg)
	}

	return m, nil
}

// handleMouseEvent is now in handlers_mouse.go
