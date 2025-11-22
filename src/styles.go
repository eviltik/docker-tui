package main

import "github.com/charmbracelet/lipgloss"

// Spinner frames for processing indicator
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// State icons
const (
	iconRunning  = "▶"
	iconStopped  = "■"
	iconPaused   = "⏸"
	iconRestart  = "⟳"
	iconSelected = "✓"
	iconEmpty    = "○"
)

// VSCode color palette - sober and professional
const (
	// Background colors
	bgDefault  = "#1e1e1e"
	bgSelected = "#264f78" // Dark blue for selected line
	bgBorder   = "#3c3c3c"

	// Foreground colors
	fgDefault = "#cccccc"
	fgBright  = "#ffffff"
	fgDim     = "#808080"

	// State colors
	colorRunning = "#4ec9b0" // Teal
	colorStopped = "#f48771" // Light red
	colorProcess = "#4fc1ff" // Sky blue
	colorError   = "#f48771" // Red
	colorSuccess = "#89d185" // Green
	colorWarning = "#dcdcaa" // Pale yellow

	// Column separator color
	colorSeparator = "#3c3c3c" // Dark gray
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorProcess))

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(fgBright))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorWarning)).
			Bold(true)

	selectedLineStyle = lipgloss.NewStyle().
				Background(lipgloss.Color(bgSelected))

	runningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorRunning))

	stoppedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorStopped))

	processingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorProcess))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorError)).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorSuccess))

	confirmStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorError)).
			Padding(1, 2).
			Bold(true)

	containerBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(bgBorder)).
				Padding(0, 1)

	toastSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorSuccess)).
				Background(lipgloss.Color(bgDefault)).
				Bold(true).
				Padding(0, 1)

	toastErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorError)).
			Background(lipgloss.Color(bgDefault)).
			Bold(true).
			Padding(0, 1)
)
