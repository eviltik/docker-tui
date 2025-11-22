package main

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleMCPLogsViewKeys handles keyboard input in MCP logs popup view
func (m *model) handleMCPLogsViewKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "Q":
		// Close popup and return to list view
		m.view = listView
		return m, nil
	}

	return m, nil
}
