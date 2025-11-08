package main

import tea "github.com/charmbracelet/bubbletea"

// handleFilterMode handles keyboard input when filter mode is active
// Returns (model, command) - command may be nil
func (m *model) handleFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		// Apply filter and exit filter mode
		m.filterActive = m.filterInput
		m.compileFilter(m.filterActive)
		m.filterMode = false
		return m, nil

	case tea.KeyEsc:
		// Cancel filter input - restore previous filter
		m.filterMode = false
		m.filterInput = m.filterActive
		return m, nil

	case tea.KeyBackspace:
		// Remove last character
		if len(m.filterInput) > 0 {
			m.filterInput = m.filterInput[:len(m.filterInput)-1]
		}
		// Update filter in real-time
		m.filterActive = m.filterInput
		m.compileFilter(m.filterActive)
		// Reset scroll: to bottom (100%) if filter is empty, to top (0%) if filter has content
		if m.filterActive == "" {
			m.logsViewScroll = max(0, m.getFilteredLogCount()-(m.height-5))
		} else {
			m.logsViewScroll = 0
		}
		return m, nil

	case tea.KeySpace:
		// Add space to filter
		m.filterInput += " "
		// Update filter in real-time
		m.filterActive = m.filterInput
		m.compileFilter(m.filterActive)
		// Scroll to bottom to see latest filtered results
		m.logsViewScroll = max(0, m.getFilteredLogCount()-(m.height-5))
		m.updateWasAtBottom()
		return m, nil

	case tea.KeyRunes:
		// Add character to filter
		m.filterInput += string(msg.Runes)
		// Update filter in real-time
		m.filterActive = m.filterInput
		m.compileFilter(m.filterActive)
		// Scroll to bottom to see latest filtered results
		m.logsViewScroll = max(0, m.getFilteredLogCount()-(m.height-5))
		m.updateWasAtBottom()
		return m, nil
	}

	return m, nil
}
