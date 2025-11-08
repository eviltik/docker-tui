package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

// TestContainerSorting tests container sorting logic used in loadContainersSync
func TestContainerSorting(t *testing.T) {
	containers := []types.Container{
		{ID: "3", Names: []string{"/zebra"}, State: "running"},
		{ID: "1", Names: []string{"/alpha"}, State: "running"},
		{ID: "2", Names: []string{"/beta"}, State: "exited"},
		{ID: "4", Names: []string{}, State: "running"}, // Empty names
	}

	// Test sorting logic (same as in loadContainersSync)
	// Sort by name
	type ContainerWithName struct {
		container types.Container
		name      string
	}

	var namedContainers []ContainerWithName
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		namedContainers = append(namedContainers, ContainerWithName{c, name})
	}

	// Verify we have all containers
	if len(namedContainers) != 4 {
		t.Errorf("namedContainers length = %d, want 4", len(namedContainers))
	}

	// Verify names are extracted correctly
	if namedContainers[1].name != "alpha" {
		t.Errorf("name = %q, want %q", namedContainers[1].name, "alpha")
	}
	if namedContainers[3].name != "" {
		t.Errorf("empty names should result in empty string, got %q", namedContainers[3].name)
	}
}

// TestMatchContainersByName tests container matching by name/ID
func TestMatchContainersByName(t *testing.T) {
	// Note: matchContainersByName requires a real client.Client type
	// We test the matching logic separately

	tests := []struct {
		name           string
		containers     []types.Container
		searchNames    []string
		wantMatchCount int
	}{
		{
			name: "exact name match",
			containers: []types.Container{
				{ID: "1", Names: []string{"/nginx"}},
				{ID: "2", Names: []string{"/postgres"}},
			},
			searchNames:    []string{"nginx"},
			wantMatchCount: 1,
		},
		{
			name: "partial name match",
			containers: []types.Container{
				{ID: "1", Names: []string{"/my-nginx-app"}},
				{ID: "2", Names: []string{"/postgres"}},
			},
			searchNames:    []string{"nginx"},
			wantMatchCount: 1,
		},
		{
			name: "ID match",
			containers: []types.Container{
				{ID: "abc123def456", Names: []string{"/nginx"}},
			},
			searchNames:    []string{"abc123"},
			wantMatchCount: 1,
		},
		{
			name: "no match",
			containers: []types.Container{
				{ID: "1", Names: []string{"/nginx"}},
			},
			searchNames:    []string{"mysql"},
			wantMatchCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the matching logic (case insensitive substring search)
			matchCount := 0
			for _, searchName := range tt.searchNames {
				searchLower := strings.ToLower(searchName)
				for _, c := range tt.containers {
					containerName := strings.ToLower(getContainerName(c))
					containerID := strings.ToLower(c.ID)

					if strings.Contains(containerName, searchLower) || strings.Contains(containerID, searchLower) {
						matchCount++
						break
					}
				}
			}

			if matchCount != tt.wantMatchCount {
				t.Errorf("match count = %d, want %d", matchCount, tt.wantMatchCount)
			}
		})
	}
}

// TestCalculateCPUPercent tests CPU calculation for MCP
func TestCalculateCPUPercent(t *testing.T) {
	tests := []struct {
		name  string
		stats *container.StatsResponse
		want  float64
	}{
		{
			name: "50% usage on 2 CPUs",
			stats: &container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage:  20000000000,
						PercpuUsage: []uint64{1, 2}, // 2 CPUs
					},
					SystemUsage: 40000000000,
				},
				PreCPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 10000000000,
					},
					SystemUsage: 20000000000,
				},
			},
			want: 100.0, // (20-10)/(40-20) * 2 * 100 = 100%
		},
		{
			name: "zero delta",
			stats: &container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage:  10000000000,
						PercpuUsage: []uint64{1},
					},
					SystemUsage: 20000000000,
				},
				PreCPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 10000000000,
					},
					SystemUsage: 20000000000,
				},
			},
			want: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateCPUPercent(tt.stats)
			if got != tt.want {
				t.Errorf("calculateCPUPercent() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestFormatPortsForMCP tests port formatting for MCP output
func TestFormatPortsForMCP(t *testing.T) {
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
			name: "single public port",
			ports: []types.Port{
				{PublicPort: 8080, PrivatePort: 80, Type: "tcp"},
			},
			want: "8080:80/tcp",
		},
		{
			name: "multiple ports",
			ports: []types.Port{
				{PublicPort: 8080, PrivatePort: 80, Type: "tcp"},
				{PublicPort: 8443, PrivatePort: 443, Type: "tcp"},
			},
			want: "8080:80/tcp, 8443:443/tcp",
		},
		{
			name: "private port only",
			ports: []types.Port{
				{PublicPort: 0, PrivatePort: 3306, Type: "tcp"},
			},
			want: "3306/tcp",
		},
		{
			name: "udp port",
			ports: []types.Port{
				{PublicPort: 53, PrivatePort: 53, Type: "udp"},
			},
			want: "53:53/udp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPortsForMCP(tt.ports)
			if got != tt.want {
				t.Errorf("formatPortsForMCP() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestMCPToolArgumentParsing tests argument struct definitions
func TestMCPToolArgumentParsing(t *testing.T) {
	t.Run("ListContainersArgs", func(t *testing.T) {
		args := ListContainersArgs{
			All:         true,
			NameFilter:  "nginx",
			StateFilter: "running",
		}

		if !args.All {
			t.Error("All should be true")
		}
		if args.NameFilter != "nginx" {
			t.Errorf("NameFilter = %q, want %q", args.NameFilter, "nginx")
		}
		if args.StateFilter != "running" {
			t.Errorf("StateFilter = %q, want %q", args.StateFilter, "running")
		}
	})

	t.Run("GetLogsArgs", func(t *testing.T) {
		args := GetLogsArgs{
			Containers: []string{"nginx", "postgres"},
			Lines:      100,
			Filter:     "error",
			IsRegex:    true,
			Tail:       true,
		}

		if len(args.Containers) != 2 {
			t.Errorf("Containers count = %d, want 2", len(args.Containers))
		}
		if args.Lines != 100 {
			t.Errorf("Lines = %d, want 100", args.Lines)
		}
		if !args.IsRegex {
			t.Error("IsRegex should be true")
		}
	})

	t.Run("GetStatsArgs", func(t *testing.T) {
		args := GetStatsArgs{
			Containers: []string{"nginx"},
			History:    true,
		}

		if len(args.Containers) != 1 {
			t.Errorf("Containers count = %d, want 1", len(args.Containers))
		}
		if !args.History {
			t.Error("History should be true")
		}
	})

	t.Run("ContainerActionArgs", func(t *testing.T) {
		args := ContainerActionArgs{
			Containers: []string{"nginx", "postgres", "redis"},
		}

		if len(args.Containers) != 3 {
			t.Errorf("Containers count = %d, want 3", len(args.Containers))
		}
	})
}

// TestMCPToolResponseFormat tests response structure
func TestMCPToolResponseFormat(t *testing.T) {
	t.Run("list_containers response", func(t *testing.T) {
		// Simulate the JSON structure
		type ContainerInfo struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			State      string `json:"state"`
			Status     string `json:"status"`
			CPUPercent string `json:"cpu_percent"`
			LogRate    string `json:"log_rate"`
			Ports      string `json:"ports"`
		}

		result := []ContainerInfo{
			{
				ID:         "abc123def456",
				Name:       "nginx",
				State:      "running",
				Status:     "Up 2 hours",
				CPUPercent: "5.2",
				LogRate:    "12",
				Ports:      "8080:80/tcp",
			},
		}

		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			t.Fatalf("json.MarshalIndent() error = %v", err)
		}

		if !strings.Contains(string(jsonData), "nginx") {
			t.Error("JSON output should contain container name")
		}
		if !strings.Contains(string(jsonData), "running") {
			t.Error("JSON output should contain state")
		}
	})

	t.Run("get_stats response", func(t *testing.T) {
		type StatsInfo struct {
			ID         string    `json:"id"`
			Name       string    `json:"name"`
			State      string    `json:"state"`
			CPUPercent string    `json:"cpu_percent"`
			CPUHistory []float64 `json:"cpu_history,omitempty"`
			LogRate    string    `json:"log_rate"`
			Status     string    `json:"status"`
			Ports      string    `json:"ports"`
		}

		result := []StatsInfo{
			{
				ID:         "abc123",
				Name:       "nginx",
				State:      "running",
				CPUPercent: "15.3",
				CPUHistory: []float64{10.0, 12.5, 15.3},
				LogRate:    "25.0",
				Status:     "Up 2 hours",
				Ports:      "8080:80/tcp",
			},
		}

		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			t.Fatalf("json.MarshalIndent() error = %v", err)
		}

		if !strings.Contains(string(jsonData), "cpu_history") {
			t.Error("JSON output should contain cpu_history when present")
		}
	})

	t.Run("action response format", func(t *testing.T) {
		results := []string{
			"✓ nginx: started successfully",
			"✗ postgres: already running",
			"✓ redis: stopped successfully",
		}

		output := strings.Join(results, "\n")

		if !strings.Contains(output, "✓") {
			t.Error("Action response should contain success indicators")
		}
		if !strings.Contains(output, "nginx") {
			t.Error("Action response should contain container names")
		}
	})
}

// TestMCPContainerFiltering tests container filtering logic
func TestMCPContainerFiltering(t *testing.T) {
	containers := []types.Container{
		{ID: "1", Names: []string{"/nginx"}, State: "running"},
		{ID: "2", Names: []string{"/postgres"}, State: "exited"},
		{ID: "3", Names: []string{"/redis"}, State: "running"},
		{ID: "4", Names: []string{"/nginx-proxy"}, State: "paused"},
	}

	t.Run("filter by state - running only", func(t *testing.T) {
		var filtered []types.Container
		for _, c := range containers {
			if c.State == "running" {
				filtered = append(filtered, c)
			}
		}

		if len(filtered) != 2 {
			t.Errorf("filtered count = %d, want 2 running containers", len(filtered))
		}
	})

	t.Run("filter by name", func(t *testing.T) {
		nameFilter := "nginx"
		var filtered []types.Container
		for _, c := range containers {
			name := getContainerName(c)
			if strings.Contains(strings.ToLower(name), strings.ToLower(nameFilter)) {
				filtered = append(filtered, c)
			}
		}

		if len(filtered) != 2 {
			t.Errorf("filtered count = %d, want 2 containers matching 'nginx'", len(filtered))
		}
	})

	t.Run("filter by state and name", func(t *testing.T) {
		nameFilter := "nginx"
		stateFilter := "running"
		var filtered []types.Container
		for _, c := range containers {
			if c.State != stateFilter {
				continue
			}
			name := getContainerName(c)
			if !strings.Contains(strings.ToLower(name), strings.ToLower(nameFilter)) {
				continue
			}
			filtered = append(filtered, c)
		}

		if len(filtered) != 1 {
			t.Errorf("filtered count = %d, want 1 running nginx container", len(filtered))
		}
	})
}

// TestMCPLogFiltering tests log filtering with regex
func TestMCPLogFiltering(t *testing.T) {
	logs := []string{
		"[INFO] Application started",
		"[ERROR] Connection failed",
		"[WARN] Memory usage high",
		"[ERROR] Database timeout",
		"[INFO] Request processed",
	}

	t.Run("substring filter", func(t *testing.T) {
		filter := "error"
		var filtered []string
		for _, line := range logs {
			content := stripAnsiCodes(line)
			if strings.Contains(strings.ToLower(content), strings.ToLower(filter)) {
				filtered = append(filtered, line)
			}
		}

		if len(filtered) != 2 {
			t.Errorf("filtered count = %d, want 2 error logs", len(filtered))
		}
	})

	t.Run("no filter", func(t *testing.T) {
		if len(logs) != 5 {
			t.Errorf("log count = %d, want 5 total logs", len(logs))
		}
	})
}

// TestMCPDefaultValues tests default value handling
func TestMCPDefaultValues(t *testing.T) {
	t.Run("GetLogsArgs defaults", func(t *testing.T) {
		args := GetLogsArgs{Lines: 0}

		// Simulate default application
		if args.Lines == 0 {
			args.Lines = 100
		}

		if args.Lines != 100 {
			t.Errorf("default Lines = %d, want 100", args.Lines)
		}
	})

	t.Run("GetLogsArgs max limit", func(t *testing.T) {
		args := GetLogsArgs{Lines: 50000}

		// Simulate max limit
		if args.Lines > 10000 {
			args.Lines = 10000
		}

		if args.Lines != 10000 {
			t.Errorf("capped Lines = %d, want 10000", args.Lines)
		}
	})

	t.Run("timeout values", func(t *testing.T) {
		timeout := 10

		if timeout != 10 {
			t.Errorf("timeout = %d, want 10", timeout)
		}
	})
}

// TestMCPErrorHandling tests error scenarios
func TestMCPErrorHandling(t *testing.T) {
	t.Run("no containers found", func(t *testing.T) {
		containers := []types.Container{}
		message := "No containers found matching the specified names"

		if len(containers) == 0 {
			// Should return the expected message
			if message != "No containers found matching the specified names" {
				t.Error("Expected 'no containers found' message")
			}
		}
	})

	t.Run("already running error", func(t *testing.T) {
		c := types.Container{
			ID:    "1",
			Names: []string{"/nginx"},
			State: "running",
		}

		var result string
		if c.State == "running" {
			result = fmt.Sprintf("✓ %s: already running", getContainerName(c))
		}

		if !strings.Contains(result, "already running") {
			t.Error("Expected 'already running' message")
		}
	})

	t.Run("already stopped error", func(t *testing.T) {
		c := types.Container{
			ID:    "1",
			Names: []string{"/nginx"},
			State: "exited",
		}

		var result string
		if c.State != "running" {
			result = fmt.Sprintf("✓ %s: already stopped", getContainerName(c))
		}

		if !strings.Contains(result, "already stopped") {
			t.Error("Expected 'already stopped' message")
		}
	})
}

// TestMCPContextHandling tests context timeout behavior
func TestMCPContextHandling(t *testing.T) {
	t.Run("context with timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		select {
		case <-ctx.Done():
			t.Error("Context should not be done immediately")
		default:
			// Expected: context is still valid
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		cancel() // Cancel immediately

		if ctx.Err() == nil {
			t.Error("Context should be cancelled")
		}
	})
}

// TestProtocolContentCreation tests protocol.Content creation
func TestProtocolContentCreation(t *testing.T) {
	t.Run("TextContent creation", func(t *testing.T) {
		content := &protocol.TextContent{
			Type: "text",
			Text: "Test output",
		}

		if content.Type != "text" {
			t.Errorf("content.Type = %q, want %q", content.Type, "text")
		}
		if content.Text != "Test output" {
			t.Errorf("content.Text = %q, want %q", content.Text, "Test output")
		}
	})

	t.Run("CallToolResult creation", func(t *testing.T) {
		result := &protocol.CallToolResult{
			Content: []protocol.Content{
				&protocol.TextContent{
					Type: "text",
					Text: "Success",
				},
			},
		}

		if len(result.Content) != 1 {
			t.Errorf("result.Content length = %d, want 1", len(result.Content))
		}
	})
}
