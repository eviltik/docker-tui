package main

import tea "github.com/charmbracelet/bubbletea"

// handleExitConfirmKeys handles keyboard input in exit confirmation dialog
func (m *model) handleExitConfirmKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		return m, tea.Quit
	case "n", "N", "esc", "q", "Q":
		m.view = listView
		return m, nil
	}
	return m, nil
}

// handleConfirmViewKeys handles keyboard input in action confirmation dialog
func (m *model) handleConfirmViewKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.view = listView
		action := m.pendingAction
		m.pendingAction = ""
		if action != "" {
			return m, m.performAction(action)
		}
		return m, nil
	case "n", "N", "esc", "q", "Q":
		m.view = listView
		m.pendingAction = ""
		return m, nil
	}
	return m, nil
}
