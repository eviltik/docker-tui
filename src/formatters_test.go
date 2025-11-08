package main

import (
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
)

// TestFormatCPU tests CPU percentage formatting with color coding
func TestFormatCPU(t *testing.T) {
	tests := []struct {
		name     string
		cpu      float64
		state    string
		wantText string // Expected text without ANSI codes
	}{
		{
			name:     "stopped container",
			cpu:      0,
			state:    "exited",
			wantText: "       ", // Empty spaces for non-running
		},
		{
			name:     "paused container",
			cpu:      0,
			state:    "paused",
			wantText: "       ", // Empty spaces for non-running
		},
		{
			name:     "running low CPU",
			cpu:      5.2,
			state:    "running",
			wantText: "   5.2%",
		},
		{
			name:     "running medium CPU",
			cpu:      45.8,
			state:    "running",
			wantText: "  45.8%",
		},
		{
			name:     "running high CPU",
			cpu:      95.3,
			state:    "running",
			wantText: "  95.3%",
		},
		{
			name:     "running very high CPU",
			cpu:      250.0,
			state:    "running",
			wantText: " 250.0%",
		},
		{
			name:     "running zero CPU",
			cpu:      0.0,
			state:    "running",
			wantText: "   0.0%",
		},
	}

	m := &model{
		cpuCurrent: make(map[string]float64),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			containerID := "test-id-" + tt.name
			m.cpuCurrent[containerID] = tt.cpu

			result := m.formatCPU(containerID, tt.state)

			// Strip ANSI codes for text comparison
			cleaned := stripAnsiCodes(result)

			if cleaned != tt.wantText {
				t.Errorf("formatCPU() = %q (cleaned: %q), want %q", result, cleaned, tt.wantText)
			}

			// Verify alignment (should be 7 chars when cleaned)
			if len(cleaned) != 7 {
				t.Errorf("formatCPU() length = %d, want 7 (for alignment)", len(cleaned))
			}
		})
	}
}

// TestFormatUptime tests uptime formatting
func TestFormatUptime(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		state    string
		wantText string
	}{
		{
			name:     "running less than 1 minute",
			status:   "Up 30 seconds",
			state:    "running",
			wantText: "    30s",
		},
		{
			name:     "running minutes",
			status:   "Up 5 minutes",
			state:    "running",
			wantText: "     5m",
		},
		{
			name:     "running hours",
			status:   "Up 2 hours",
			state:    "running",
			wantText: "     2h",
		},
		{
			name:     "running days",
			status:   "Up 3 days",
			state:    "running",
			wantText: "     3d",
		},
		{
			name:     "stopped recently",
			status:   "Exited (0) 5 minutes ago",
			state:    "exited",
			wantText: "     5m",
		},
		{
			name:     "stopped hours ago",
			status:   "Exited (1) 3 hours ago",
			state:    "exited",
			wantText: "     3h",
		},
		{
			name:     "stopped days ago",
			status:   "Exited (0) 7 days ago",
			state:    "exited",
			wantText: "     7d",
		},
		{
			name:     "created but not started",
			status:   "Created",
			state:    "created",
			wantText: "       ", // Empty string (7 spaces, no uptime data)
		},
	}

	m := &model{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.formatUptime(tt.status, tt.state)
			cleaned := stripAnsiCodes(result)

			if cleaned != tt.wantText {
				t.Errorf("formatUptime(%q, %q) = %q, want %q", tt.status, tt.state, cleaned, tt.wantText)
			}

			// Verify alignment (should be 7 chars)
			if len(cleaned) != 7 {
				t.Errorf("formatUptime() length = %d, want 7 (for alignment)", len(cleaned))
			}
		})
	}
}

// TestFormatLogRate tests log rate formatting
func TestFormatLogRate(t *testing.T) {
	tests := []struct {
		name     string
		rate     float64
		state    string
		wantText string
	}{
		{
			name:     "stopped container",
			rate:     0,
			state:    "exited",
			wantText: "      ", // Empty spaces for non-running
		},
		{
			name:     "running zero logs",
			rate:     0.0,
			state:    "running",
			wantText: "     0",
		},
		// Note: The rate tracking is based on actual log line additions within a 1-second window
		// Since we can't easily inject rates directly, we skip detailed rate formatting tests
		// and only test the basic formatting logic
	}

	m := &model{
		rateTracker: NewRateTrackerConsumer(),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			containerID := "test-id-" + tt.name

			// Simulate log rate by adding lines
			if tt.rate > 0 {
				m.rateTracker.ratesMu.Lock()
				tracker := m.rateTracker.rates[containerID]
				if tracker == nil {
					tracker = &LogRateTracker{
						lines: []time.Time{},
						lastUpdate: time.Now(),
					}
					m.rateTracker.rates[containerID] = tracker
				}
				m.rateTracker.ratesMu.Unlock()

				// Add lines to simulate rate (approximate)
				for i := 0; i < int(tt.rate); i++ {
					tracker.AddLine()
				}
			}

			result := m.formatLogRate(containerID, tt.state)
			cleaned := stripAnsiCodes(result)

			if cleaned != tt.wantText {
				t.Errorf("formatLogRate(%f, %q) = %q, want %q", tt.rate, tt.state, cleaned, tt.wantText)
			}

			// Verify alignment (should be 6 chars for L/S column, but we allow 5-6)
			if len(cleaned) < 5 || len(cleaned) > 6 {
				t.Errorf("formatLogRate() length = %d, want 5-6 (for alignment)", len(cleaned))
			}
		})
	}
}

// TestFormatPorts tests port formatting
func TestFormatPorts(t *testing.T) {
	tests := []struct {
		name  string
		ports []types.Port
		want  string
	}{
		{
			name:  "no ports",
			ports: []types.Port{},
			want:  "",
		},
		{
			name: "single port",
			ports: []types.Port{
				{PublicPort: 8080, PrivatePort: 80, Type: "tcp"},
			},
			want: "8080", // Only shows PublicPort
		},
		{
			name: "multiple ports",
			ports: []types.Port{
				{PublicPort: 8080, PrivatePort: 80, Type: "tcp"},
				{PublicPort: 8443, PrivatePort: 443, Type: "tcp"},
			},
			want: "8080,8443", // Shows PublicPorts separated by comma
		},
		{
			name: "port range detection",
			ports: []types.Port{
				{PublicPort: 8080, PrivatePort: 80, Type: "tcp"},
				{PublicPort: 8081, PrivatePort: 81, Type: "tcp"},
				{PublicPort: 8082, PrivatePort: 82, Type: "tcp"},
			},
			want: "8080-8082", // Shows PublicPort range
		},
		{
			name: "udp port",
			ports: []types.Port{
				{PublicPort: 53, PrivatePort: 53, Type: "udp"},
			},
			want: "53", // Only shows PublicPort
		},
		{
			name: "no public port",
			ports: []types.Port{
				{PublicPort: 0, PrivatePort: 80, Type: "tcp"},
			},
			want: "", // No public port = empty string
		},
	}

	m := &model{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.formatPorts(tt.ports)
			if result != tt.want {
				t.Errorf("formatPorts() = %q, want %q", result, tt.want)
			}
		})
	}
}

// TestFormatState tests container state formatting
func TestFormatState(t *testing.T) {
	tests := []struct {
		name      string
		container types.Container
		wantIcon  string // Expected icon (without ANSI)
		wantState string // Expected state text
	}{
		{
			name: "running container",
			container: types.Container{
				State: "running",
			},
			wantIcon:  "▶",
			wantState: "running",
		},
		{
			name: "exited container",
			container: types.Container{
				State: "exited",
			},
			wantIcon:  "■",
			wantState: "stopped", // "exited" is displayed as "stopped"
		},
		{
			name: "paused container",
			container: types.Container{
				State: "paused",
			},
			wantIcon:  "⏸",
			wantState: "paused",
		},
		{
			name: "restarting container",
			container: types.Container{
				State: "restarting",
			},
			wantIcon:  "⟳", // Note: iconRestart is "⟳", not "↻"
			wantState: "restart", // "restarting" is displayed as "restart"
		},
		{
			name: "created container",
			container: types.Container{
				State: "created",
			},
			wantIcon:  "■", // Created uses iconStopped ("■"), not iconEmpty
			wantState: "created",
		},
	}

	m := &model{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.formatState(tt.container)
			cleaned := stripAnsiCodes(result)

			// Verify icon is present
			if !strings.Contains(cleaned, tt.wantIcon) {
				t.Errorf("formatState() = %q, want to contain icon %q", cleaned, tt.wantIcon)
			}

			// Verify state name is present
			if !strings.Contains(cleaned, tt.wantState) {
				t.Errorf("formatState() = %q, want to contain state %q", cleaned, tt.wantState)
			}
		})
	}
}

// TestCleanContainerName tests demo mode name cleaning
func TestCleanContainerName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		demoMode  bool
		want      string
	}{
		{
			name:     "demo mode with prefix",
			input:    "project_nginx",
			demoMode: true,
			want:     "nginx",
		},
		{
			name:     "demo mode without prefix",
			input:    "simple",
			demoMode: true,
			want:     "simple",
		},
		{
			name:     "demo mode with multiple underscores",
			input:    "my_project_nginx_production",
			demoMode: true,
			want:     "project_nginx_production",
		},
		{
			name:     "normal mode with prefix",
			input:    "project_nginx",
			demoMode: false,
			want:     "project_nginx",
		},
		{
			name:     "demo mode with leading slash",
			input:    "/project_nginx",
			demoMode: true,
			want:     "nginx",
		},
		{
			name:     "demo mode empty after underscore",
			input:    "project_",
			demoMode: true,
			want:     "", // Returns empty string (everything after the underscore)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{demoMode: tt.demoMode}
			result := m.cleanContainerName(tt.input)
			if result != tt.want {
				t.Errorf("cleanContainerName(%q, demoMode=%v) = %q, want %q", tt.input, tt.demoMode, result, tt.want)
			}
		})
	}
}

// TestStripAnsiCodes tests ANSI code stripping (used by log filtering)
func TestStripAnsiCodes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no ANSI codes",
			input: "simple text",
			want:  "simple text",
		},
		{
			name:  "basic color code",
			input: "\x1b[31mred text\x1b[0m",
			want:  "red text",
		},
		{
			name:  "multiple ANSI codes",
			input: "\x1b[1m\x1b[32mbold green\x1b[0m normal",
			want:  "bold green normal",
		},
		{
			name:  "SGR codes with parameters",
			input: "\x1b[48;2;255;0;0mred background\x1b[49m",
			want:  "red background",
		},
		{
			name:  "mixed content",
			input: "normal \x1b[33myellow\x1b[0m more normal",
			want:  "normal yellow more normal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripAnsiCodes(tt.input)
			if result != tt.want {
				t.Errorf("stripAnsiCodes(%q) = %q, want %q", tt.input, result, tt.want)
			}
		})
	}
}
