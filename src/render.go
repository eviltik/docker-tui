package main

import (
	"fmt"
	"hash/fnv"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/api/types"
)

// getContainerLogColor returns a dark background color for a container based on its name
func getContainerLogColor(containerName string) lipgloss.Color {
	// Very dark background colors with maximized distinction
	darkColors := []string{
		"#0d1f3d", // Very dark blue
		"#1a3d2e", // Very dark green
		"#3d1a3d", // Very dark purple
		"#4d2a1a", // Very dark orange/brown
		"#1a2a3d", // Very dark navy
		"#2e1a3d", // Very dark violet
		"#3d1a2e", // Very dark magenta/pink
		"#1a3d3d", // Very dark teal/cyan
		"#3d3d1a", // Very dark yellow/olive
		"#3d1a1a", // Very dark red/maroon
	}

	// Hash container name to get consistent color
	h := fnv.New32a()
	h.Write([]byte(containerName))
	index := int(h.Sum32()) % len(darkColors)

	return lipgloss.Color(darkColors[index])
}

func (m *model) renderFilterBar() string {
	// Build filter text with cursor
	filterText := "Filter: " + m.filterInput + "█"

	// Apply red background if regex is invalid
	if m.filterInput != "" && !m.filterIsRegex {
		invalidStyle := lipgloss.NewStyle().Background(lipgloss.Color("#3d1a1a")).Foreground(lipgloss.Color("#ffffff"))
		return invalidStyle.Render(filterText)
	}

	// Normal style (no background)
	return filterText
}

// renderDebugMetrics renders debug monitoring metrics (goroutines, FD, memory, streams)
func (m *model) renderDebugMetrics() string {
	// Get current metrics
	goroutines := runtime.NumGoroutine()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	memMB := memStats.Alloc / 1024 / 1024

	fds := countOpenFDs()

	streams := 0
	consumers := 0
	if m.logBroker != nil {
		streams = m.logBroker.GetActiveStreamCount()
		consumers = m.logBroker.GetConsumerCount()
	}

	// Color coding based on thresholds
	grStyle := lipgloss.NewStyle()
	if goroutines > 100 {
		grStyle = grStyle.Foreground(lipgloss.Color("#ff0000")) // Red
	} else if goroutines > 50 {
		grStyle = grStyle.Foreground(lipgloss.Color("#ffff00")) // Yellow
	} else {
		grStyle = grStyle.Foreground(lipgloss.Color("#00ff00")) // Green
	}

	fdStyle := lipgloss.NewStyle()
	if fds > 500 {
		fdStyle = fdStyle.Foreground(lipgloss.Color("#ff0000")) // Red
	} else if fds > 100 {
		fdStyle = fdStyle.Foreground(lipgloss.Color("#ffff00")) // Yellow
	} else {
		fdStyle = fdStyle.Foreground(lipgloss.Color("#00ff00")) // Green
	}

	memStyle := lipgloss.NewStyle()
	if memMB > 500 {
		memStyle = memStyle.Foreground(lipgloss.Color("#ff0000")) // Red
	} else if memMB > 100 {
		memStyle = memStyle.Foreground(lipgloss.Color("#ffff00")) // Yellow
	} else {
		memStyle = memStyle.Foreground(lipgloss.Color("#00ff00")) // Green
	}

	consStyle := lipgloss.NewStyle()
	if consumers > 2 {
		consStyle = consStyle.Foreground(lipgloss.Color("#ff0000")) // Red
	} else {
		consStyle = consStyle.Foreground(lipgloss.Color("#00ff00")) // Green
	}

	// Build metrics string parts
	parts := []string{
		grStyle.Render(fmt.Sprintf("GR:%d", goroutines)),
		fdStyle.Render(fmt.Sprintf("FD:%d", fds)),
		memStyle.Render(fmt.Sprintf("MEM:%dMB", memMB)),
		fmt.Sprintf("STREAM:%d", streams),
		consStyle.Render(fmt.Sprintf("CONS:%d", consumers)),
	}

	// Add buffer size if in logs view
	if m.view == logsView && m.bufferConsumer != nil {
		bufSize := len(m.bufferConsumer.GetBuffer())
		bufStyle := lipgloss.NewStyle()
		if bufSize > 8000 {
			bufStyle = bufStyle.Foreground(lipgloss.Color("#ffff00")) // Yellow
		} else {
			bufStyle = bufStyle.Foreground(lipgloss.Color("#00ff00")) // Green
		}
		parts = append(parts, bufStyle.Render(fmt.Sprintf("BUF:%d", bufSize)))
	}

	return strings.Join(parts, " ")
}

func (m *model) renderConfirm() string {
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		confirmStyle.Render(m.confirmMessage),
	)
}

func (m *model) renderLogs() string {
	var sb strings.Builder

	// Calculate reserved lines at bottom
	bottomLines := 4 // separator + help bar + toast + blank line before separator
	if m.filterMode {
		bottomLines++ // filter bar
	}

	// Title takes 2 lines
	titleLines := 2

	// Calculate available lines for logs content
	availableLines := m.height - titleLines - bottomLines
	if availableLines < 1 {
		availableLines = 1
	}

	sb.WriteString(titleStyle.Render("📋 Container Logs") + "\n\n")

	// Get logs from BufferConsumer and format them
	var rawLogs []string
	if m.bufferConsumer != nil {
		entries := m.bufferConsumer.GetBuffer()
		rawLogs = make([]string, len(entries))
		for i, entry := range entries {
			if entry.IsSeparator {
				// Use special marker for separators to identify them during rendering
				rawLogs[i] = "[SEPARATOR] " + entry.Line
			} else {
				displayName := m.cleanContainerName(entry.ContainerName)
				rawLogs[i] = fmt.Sprintf("[%s] %s", displayName, entry.Line)
			}
		}
	} else {
		rawLogs = m.logsViewBuffer // Fallback si pas de consumer
	}

	// Filter logs if filter is active
	filteredLogs := rawLogs
	if m.filterActive != "" {
		filteredLogs = []string{}
		for _, line := range rawLogs {
			if m.logLineMatchesFilter(line) {
				filteredLogs = append(filteredLogs, line)
			}
		}
	}

	// Calculate visible window
	visibleLines := availableLines
	if visibleLines < 1 {
		visibleLines = 1
	}

	// Calculate start position based on scroll
	start := m.logsViewScroll
	end := min(start+visibleLines, len(filteredLogs))

	// Display logs in the visible window with container-specific background colors
	linesRendered := 0
	for i := start; i < end; i++ {
		logLine := filteredLogs[i]

		// Check if this is a separator line (just render as blank line)
		if strings.HasPrefix(logLine, "[SEPARATOR] ") {
			sb.WriteString("\n")
			linesRendered++
			continue
		}

		// Parse container name from log line format: [containerName] logContent
		if strings.HasPrefix(logLine, "[") {
			endBracket := strings.Index(logLine, "]")
			if endBracket > 0 {
				containerName := logLine[1:endBracket]
				logContent := logLine[endBracket+1:]

				// Clean container name in demo mode
				displayName := m.cleanContainerName(containerName)

				// Apply container-specific background color only if enabled
				var containerPart, contentPart string
				if m.logsColorEnabled {
					// Apply container-specific background color (use original name for consistent colors)
					bgColor := getContainerLogColor(containerName)
					style := lipgloss.NewStyle().Background(bgColor).Foreground(lipgloss.Color("#ffffff"))

					// Pad display name to max width for alignment
					paddedName := displayName
					if m.logsViewMaxNameWidth > 0 && len(displayName) < m.logsViewMaxNameWidth {
						paddedName = displayName + strings.Repeat(" ", m.logsViewMaxNameWidth-len(displayName))
					}

					// Format: colored container name (padded) + pipe separator + log content with same background
					containerPart = style.Render(paddedName + " │")
					contentPart = style.Render(logContent)
				} else {
					// No colors: simple text with separator
					paddedName := displayName
					if m.logsViewMaxNameWidth > 0 && len(displayName) < m.logsViewMaxNameWidth {
						paddedName = displayName + strings.Repeat(" ", m.logsViewMaxNameWidth-len(displayName))
					}
					containerPart = paddedName + " │"
					contentPart = logContent
				}

				sb.WriteString(containerPart + contentPart + "\n")
				linesRendered++
				continue
			}
		}

		// Fallback for lines without container prefix (marks, errors, etc.)
		sb.WriteString(logLine + "\n")
		linesRendered++
	}

	// Fill remaining lines to push bottom bars down
	for linesRendered < visibleLines {
		sb.WriteString("\n")
		linesRendered++
	}

	// Build help bar (left-aligned)
	helpText := "[Q/ESC] Back  [ENTER] Insert Mark  [C] Toggle Colors  [↑/↓/PgUp/PgDn/Home/End/Wheel] Scroll  [/] Filter"

	// Build scroll indicator (right-aligned)
	scrollInfo := ""
	if len(filteredLogs) > visibleLines {
		percentage := 100
		if len(filteredLogs) > 0 {
			maxScroll := len(filteredLogs) - visibleLines
			percentage = (m.logsViewScroll * 100) / max(1, maxScroll)
			// Clamp to 100% maximum
			if percentage > 100 {
				percentage = 100
			}
		}
		scrollInfo = fmt.Sprintf("[%d%%]", percentage)
	}

	// Show filter indicator if active
	if m.filterActive != "" {
		totalLines := len(rawLogs)
		filteredCount := len(filteredLogs)
		scrollInfo = fmt.Sprintf("🔍 %d/%d %s", filteredCount, totalLines, scrollInfo)
	}

	// Add debug metrics if debug monitoring is enabled
	if m.debugMonitor {
		scrollInfo = m.renderDebugMetrics() + " " + scrollInfo
	}

	// Add separator line before help (dark gray horizontal line)
	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3c3c3c"))
	separatorLine := strings.Repeat("─", max(80, m.width))
	sb.WriteString("\n" + separatorStyle.Render(separatorLine))

	// Calculate spacing to align scroll to the right
	helpWidth := lipgloss.Width(helpText)
	scrollWidth := lipgloss.Width(scrollInfo)
	availableWidth := max(80, m.width)
	spacing := availableWidth - helpWidth - scrollWidth - 2
	if spacing < 2 {
		spacing = 2
	}

	helpLine := helpText + strings.Repeat(" ", spacing) + scrollInfo
	sb.WriteString("\n" + helpLine)

	// Toast messages (fixed space to prevent UI jumping)
	sb.WriteString("\n")
	if m.toastMessage != "" {
		if m.toastIsError {
			sb.WriteString(toastErrorStyle.Render("✗ " + m.toastMessage))
		} else {
			sb.WriteString(toastSuccessStyle.Render("✓ " + m.toastMessage))
		}
	} else {
		sb.WriteString(" ") // Reserve space even when no toast
	}

	// Show filter bar if in filter mode
	if m.filterMode {
		sb.WriteString("\n" + m.renderFilterBar())
	}

	return sb.String()
}

func (m *model) renderList() string {
	var sb strings.Builder

	// Early return if dimensions are invalid
	if m.width < 40 || m.height < 10 {
		return "Terminal too small. Please resize to at least 40x10."
	}

	// Calculate reserved lines at bottom
	// Help bar text (used later for rendering)
	selectionHelp := "[SPACE] Select  [A] All  [Ctrl+A] Running  [X] Clear  [I] Invert"
	actionsHelp := "[ENTER/L] Logs  [S] Start  [K] Kill (Stop)  [R] Restart  [P] Pause  [D] Remove  [/] Filter"
	if m.mcpServer != nil {
		actionsHelp += "  [M] MCP Logs"
	}
	actionsHelp += "  [Q/ESC] Quit"

	// Fixed bottom lines: blank line + toast + blank line + help bar = 4 lines
	bottomLines := 4
	if m.filterMode {
		bottomLines++ // filter bar
	}
	if m.err != nil {
		bottomLines++ // error line
	}

	// Header takes 2 lines (title + blank line)
	headerLines := 2

	// Box borders and header take 4 lines (top border + header + separator + bottom border)
	boxOverhead := 4

	// Calculate available lines for container rows
	availableForBox := m.height - headerLines - bottomLines
	availableForRows := availableForBox - boxOverhead
	if availableForRows < 1 {
		availableForRows = 1
	}

	// CRITICAL FIX: Protect all reads of m.containers with mutex to prevent race conditions
	m.containersMu.RLock()
	containersCopy := make([]types.Container, len(m.containers))
	copy(containersCopy, m.containers)
	m.containersMu.RUnlock()

	// Filter containers if filter is active
	visibleContainers := containersCopy
	hiddenCount := 0
	if m.filterActive != "" {
		visibleContainers = []types.Container{}
		for _, c := range containersCopy {
			if m.containerMatchesFilter(c) {
				visibleContainers = append(visibleContainers, c)
			}
		}
		hiddenCount = len(containersCopy) - len(visibleContainers)
	}

	// Calculate container stats (from all containers, not just visible)
	totalContainers := len(containersCopy)
	runningContainers := 0
	stoppedContainers := 0
	for _, c := range containersCopy {
		if c.State == "running" {
			runningContainers++
		} else {
			stoppedContainers++
		}
	}

	// Header line: title on the left, stats on the right
	title := titleStyle.Render(fmt.Sprintf("🐳 Docker TUI %s", Version))

	// Add filter indicator if filter is active
	filterIndicator := ""
	if m.filterActive != "" {
		filterIndicator = fmt.Sprintf(" [🔍 %s] ", m.filterActive)
	}

	// Build stats string
	stats := fmt.Sprintf("Total: %d │ Running: %d │ Stopped: %d",
		totalContainers, runningContainers, stoppedContainers)

	// Add hidden count if filter is active
	if m.filterActive != "" {
		stats += fmt.Sprintf(" │ Hidden: %d", hiddenCount)
	}

	// Add MCP status if MCP server is running
	if m.mcpServer != nil {
		mcpPort := m.mcpServer.GetPort()
		mcpClients := m.mcpServer.GetConnectedClients()
		stats += fmt.Sprintf(" │ MCP: %d clients (:%d)", mcpClients, mcpPort)
	}

	// Add debug metrics if debug monitoring is enabled
	if m.debugMonitor {
		stats += " │ " + m.renderDebugMetrics()
	}

	// Calculate spacing to align stats to the right
	// Use lipgloss.Width to get accurate visual width
	titleWidth := lipgloss.Width(title)
	filterWidth := lipgloss.Width(filterIndicator)
	statsWidth := lipgloss.Width(stats)
	availableWidth := max(80, m.width)
	spacing := availableWidth - titleWidth - filterWidth - statsWidth - 2
	if spacing < 2 {
		spacing = 2
	}

	headerLine := title + filterIndicator + strings.Repeat(" ", spacing) + statusBarStyle.Render(stats)
	sb.WriteString(headerLine + "\n\n")

	// Container list content (will be wrapped in a box)
	var containerList strings.Builder

	// Calculate available width for content (accounting for box borders and padding)
	contentWidth := max(80, m.width) - 6 // -6 for border + padding

	// Column headers - selection(2) = 2 chars prefix
	// Widths: NAME(35) STATE(13) CPU(7) L/S(6) UPTIME(7) PORTS(variable)
	// Format: column + space + sep + space (except last column)
	// Dark gray separator style
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorSeparator))
	sep := sepStyle.Render("│")
	containerList.WriteString(fmt.Sprintf("  %-35s %s %-13s %s %-7s %s %-6s %s %-7s %s %s\n",
		"NAME", sep, "STATE", sep, "CPU", sep, "L/S", sep, "UPTIME", sep, "PORTS"))
	containerList.WriteString(strings.Repeat("─", contentWidth) + "\n")

	// Calculate scroll window for containers
	startIdx := 0
	endIdx := len(visibleContainers)

	// CRITICAL FIX: Validate indices before slice access to prevent index out of bounds panic
	// Empty container list or invalid cursor position could cause crash
	var displayContainers []types.Container
	if len(visibleContainers) == 0 {
		displayContainers = []types.Container{}
	} else {
		// Adjust window if cursor is out of visible range
		if m.cursor >= availableForRows {
			startIdx = m.cursor - availableForRows + 1
			// CRITICAL FIX: Clamp startIdx to valid range
			if startIdx < 0 {
				startIdx = 0
			}
			if startIdx >= len(visibleContainers) {
				startIdx = max(0, len(visibleContainers)-1)
			}
		}
		if startIdx+availableForRows < len(visibleContainers) {
			endIdx = startIdx + availableForRows
		}

		// CRITICAL FIX: Final validation before slice access
		if startIdx >= len(visibleContainers) {
			startIdx = 0
			endIdx = min(availableForRows, len(visibleContainers))
		}
		if endIdx > len(visibleContainers) {
			endIdx = len(visibleContainers)
		}
		if startIdx > endIdx {
			startIdx = 0
			endIdx = 0
		}

		displayContainers = visibleContainers[startIdx:endIdx]
	}

	// Container rows (use displayContainers)
	rowsRendered := 0
	for i, c := range displayContainers {
		rowsRendered++
		actualIndex := startIdx + i // Index in the full container list
		var line strings.Builder

		// Selection mark with icons
		// CRITICAL FIX: Protect read of m.selected with mutex to prevent race condition
		m.selectedMu.RLock()
		isSelected := m.selected[c.ID]
		m.selectedMu.RUnlock()

		if isSelected {
			line.WriteString(selectedStyle.Render(iconSelected) + " ")
		} else {
			line.WriteString("  ")
		}

		// Name (first column) - 35 chars + space + sep + space
		name := getContainerName(c)
		name = m.cleanContainerName(name) // Apply demo mode cleaning
		if len(name) > 35 {
			name = name[:32] + "..."
		}
		line.WriteString(fmt.Sprintf("%-35s ", name))
		line.WriteString(sep + " ")

		// State (second column) - 13 chars + space + sep + space
		state := m.formatState(c)
		line.WriteString(state + " ")
		line.WriteString(sep + " ")

		// CPU (third column) - 7 chars + space + sep + space
		cpu := m.formatCPU(c.ID, c.State)
		line.WriteString(cpu + " ")
		line.WriteString(sep + " ")

		// L/S (fourth column) - 6 chars + space + sep + space
		logs := m.formatLogRate(c.ID, c.State)
		line.WriteString(logs + " ")
		line.WriteString(sep + " ")

		// Uptime (fifth column) - 7 chars + space + sep + space
		uptime := m.formatUptime(c.Status, c.State)
		line.WriteString(uptime + " ")
		line.WriteString(sep + " ")

		// Ports (sixth column) - no trailing separator
		ports := m.formatPorts(c.Ports)
		line.WriteString(ports)

		// Build final line with cursor background if applicable
		lineText := line.String()
		if actualIndex == m.cursor {
			// For cursor line, we need to inject background color into every ANSI sequence
			// Replace all ANSI reset codes and background codes with our background

			// First, pad to full width
			visualWidth := lipgloss.Width(lineText)
			if visualWidth < contentWidth {
				lineText = lineText + strings.Repeat(" ", contentWidth-visualWidth)
			}

			// Inject our background color after every SGR reset or at the start
			// Pattern: replace \x1b[0m with \x1b[0m\x1b[48;2;38;79;120m to maintain background
			// Also add background at the very start
			lineText = "\x1b[48;2;38;79;120m" +
				strings.ReplaceAll(lineText, "\x1b[0m", "\x1b[0m\x1b[48;2;38;79;120m") +
				"\x1b[49m"
		}

		containerList.WriteString(lineText + "\n")
	}

	// Fill remaining rows with empty lines to maintain consistent box height
	for rowsRendered < availableForRows {
		containerList.WriteString(strings.Repeat(" ", contentWidth) + "\n")
		rowsRendered++
	}

	// Wrap container list in a rounded border box
	boxContent := containerList.String()
	sb.WriteString(containerBoxStyle.Render(boxContent) + "\n")

	// Help bar at the bottom - selection on left, actions on right
	sb.WriteString("\n")
	// selectionHelp and actionsHelp already defined at top for bottomLines calculation

	// Calculate spacing to align actions to the right
	selectionWidth := lipgloss.Width(selectionHelp)
	actionsWidth := lipgloss.Width(actionsHelp)
	helpAvailableWidth := max(80, m.width)
	helpSpacing := helpAvailableWidth - selectionWidth - actionsWidth - 2
	if helpSpacing < 2 {
		helpSpacing = 2
	}

	helpLine := selectionHelp + strings.Repeat(" ", helpSpacing) + actionsHelp
	sb.WriteString(helpLine)

	// Toast messages (fixed space to prevent UI jumping)
	sb.WriteString("\n")
	if m.toastMessage != "" {
		if m.toastIsError {
			sb.WriteString(toastErrorStyle.Render("✗ " + m.toastMessage))
		} else {
			sb.WriteString(toastSuccessStyle.Render("✓ " + m.toastMessage))
		}
	} else {
		sb.WriteString(" ") // Reserve space even when no toast
	}

	// Show filter bar if in filter mode
	if m.filterMode {
		sb.WriteString("\n" + m.renderFilterBar())
	}

	// Error display
	if m.err != nil {
		sb.WriteString("\n" + errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	return sb.String()
}

// renderMCPLogs renders the MCP server logs popup
func (m *model) renderMCPLogs() string {
	var sb strings.Builder

	// Get logs from MCP server
	var logs []string
	if m.mcpServer != nil {
		logs = m.mcpServer.GetLogs()
	}

	// Build popup content
	title := "📡 MCP Server Logs"
	separator := strings.Repeat("─", 120)

	sb.WriteString(title + "\n")
	sb.WriteString(separator + "\n\n")

	if len(logs) == 0 {
		sb.WriteString("No logs available\n")
	} else {
		// Show last 50 logs (already limited by buffer)
		for _, log := range logs {
			sb.WriteString(log + "\n")
		}
	}

	sb.WriteString("\n" + separator + "\n")
	sb.WriteString("Press ESC or Q to close")

	// Center the popup
	content := sb.String()
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorProcess)).
		Padding(1, 2).
		Width(124)

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		boxStyle.Render(content),
	)
}
