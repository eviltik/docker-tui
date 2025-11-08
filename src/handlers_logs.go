package main

import tea "github.com/charmbracelet/bubbletea"

// handleLogsViewKeys handles keyboard input in logs view
func (m *model) handleLogsViewKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "Q":
		// If filter is active, clear it instead of exiting
		if m.filterActive != "" {
			m.filterActive = ""
			m.filterInput = ""
			m.filterRegex = nil
			m.filterIsRegex = false
			// Reset scroll to bottom (100%)
			m.logsViewScroll = max(0, m.getFilteredLogCount()-(m.height-5))
			return m, nil
		}

		// CRITICAL FIX: Protect entire view transition with mutex to prevent race with concurrent callbacks
		m.viewTransitionMu.Lock()

		// Unregister the buffer consumer and return to list view
		if m.bufferConsumer != nil {
			m.logBroker.UnregisterConsumer(m.bufferConsumer)
			m.bufferConsumer = nil
		}
		// CRITICAL FIX: Close the channel safely with sync.Once to prevent race
		// WARNING: DO NOT reset sync.Once after use - create new instance when entering logs view
		if m.newLogChan != nil {
			m.logChanCloseOnce.Do(func() {
				// Set closing flag BEFORE waiting
				m.logChanClosing.Store(true)
				// Wait for all active callbacks to complete
				m.logChanWg.Wait()
				// Now safe to close the channel
				close(m.newLogChan)
				// Reset closing flag INSIDE Do() for next use
				m.logChanClosing.Store(false)
			})
			m.newLogChan = nil
			// NOTE: sync.Once is NOT reset - new instance will be created when re-entering logs view
		}
		m.view = listView
		m.logsViewBuffer = []string{}
		m.logsViewScroll = 0

		m.viewTransitionMu.Unlock()
		return m, nil
	case "enter":
		// Insert a visual separator line in the log buffer
		if m.bufferConsumer != nil {
			m.bufferConsumer.InsertSeparator()
			// Auto-scroll to bottom after inserting separator
			m.logsViewScroll = max(0, m.getFilteredLogCount()-(m.height-5))
			m.updateWasAtBottom()
		}
		return m, nil
	case "up", "k":
		if m.logsViewScroll > 0 {
			m.logsViewScroll--
		}
		m.updateWasAtBottom()
		return m, nil
	case "down", "j":
		maxScroll := max(0, m.getFilteredLogCount()-(m.height-5))
		if m.logsViewScroll < maxScroll {
			m.logsViewScroll++
		}
		m.updateWasAtBottom()
		return m, nil
	case "pgup":
		m.logsViewScroll -= 10
		if m.logsViewScroll < 0 {
			m.logsViewScroll = 0
		}
		m.updateWasAtBottom()
		return m, nil
	case "pgdown":
		maxScroll := max(0, m.getFilteredLogCount()-(m.height-5))
		m.logsViewScroll += 10
		if m.logsViewScroll > maxScroll {
			m.logsViewScroll = maxScroll
		}
		m.updateWasAtBottom()
		return m, nil
	case "home":
		m.logsViewScroll = 0
		m.updateWasAtBottom()
		return m, nil
	case "end":
		m.logsViewScroll = max(0, m.getFilteredLogCount()-(m.height-5))
		m.updateWasAtBottom()
		return m, nil
	case "/":
		// Enter filter mode
		m.filterMode = true
		m.filterInput = m.filterActive // Start with current filter
		return m, nil
	case "c":
		// Toggle colored backgrounds in logs view
		m.logsColorEnabled = !m.logsColorEnabled
		return m, nil
	}

	return m, nil
}
