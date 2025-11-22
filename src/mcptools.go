package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// handleListContainers implements the list_containers tool
func (s *MCPServer) handleListContainers(ctx context.Context, request *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	startTime := time.Now()
	log.Printf("[TRACE] handleListContainers START at %s", startTime.Format("15:04:05.000"))

	// Record MCP activity (use context value or create session ID)
	sessionID := getSessionID(ctx)
	s.recordActivity(sessionID)
	log.Printf("Tool: %s", request.Name)

	// Parse arguments
	log.Printf("[TRACE] Parsing arguments...")
	args := new(ListContainersArgs)
	if err := protocol.VerifyAndUnmarshal(request.RawArguments, args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	log.Printf("[TRACE] Arguments parsed in %dms", time.Since(startTime).Milliseconds())

	// Load containers
	log.Printf("[TRACE] Loading containers...")
	t1 := time.Now()
	containers, err := loadContainersSync(s.dockerClient)
	if err != nil {
		return nil, fmt.Errorf("failed to load containers: %w", err)
	}
	log.Printf("[TRACE] Loaded %d containers in %dms", len(containers), time.Since(t1).Milliseconds())

	// Get CPU stats from cache (instant, no Docker API call)
	log.Printf("[TRACE] Getting CPU stats from cache...")
	t2 := time.Now()
	cpuStats := s.cpuCache.Get()
	log.Printf("[TRACE] Got CPU stats from cache in %dms", time.Since(t2).Milliseconds())

	// Filter containers
	var filtered []types.Container
	for _, c := range containers {
		// Filter by state (all or running only)
		if !args.All && c.State != "running" {
			continue
		}

		// Filter by name
		if args.NameFilter != "" {
			name := getContainerName(c)
			if name == "" || !strings.Contains(strings.ToLower(name), strings.ToLower(args.NameFilter)) {
				continue
			}
		}

		// Filter by state
		if args.StateFilter != "" && !strings.EqualFold(c.State, args.StateFilter) {
			continue
		}

		filtered = append(filtered, c)
	}

	// Build result list
	type ContainerInfo struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		State      string `json:"state"`
		Status     string `json:"status"`
		CPUPercent string `json:"cpu_percent"`
		LogRate    string `json:"log_rate"`
		Ports      string `json:"ports"`
	}

	var result []ContainerInfo
	for _, c := range filtered {
		name := getContainerName(c)
		if name == "" {
			name = c.ID[:12] // Fallback to short ID
		}

		// Get CPU percentage (cpuStats now returns current values directly, not history)
		cpuPct := "0.0"
		if cpu, ok := cpuStats[c.ID]; ok {
			cpuPct = fmt.Sprintf("%.1f", cpu)
		}

		// Get log rate
		logRate := "0.0"
		if s.rateTracker != nil {
			rate := s.rateTracker.GetRate(c.ID)
			if rate >= 1000 {
				logRate = fmt.Sprintf("%.1fk", rate/1000)
			} else if rate >= 1 {
				logRate = fmt.Sprintf("%.0f", rate)
			} else if rate > 0 {
				logRate = fmt.Sprintf("%.1f", rate)
			} else {
				logRate = "0"
			}
		}

		// Format ports
		ports := formatPortsForMCP(c.Ports)

		result = append(result, ContainerInfo{
			ID:         c.ID[:12],
			Name:       name,
			State:      c.State,
			Status:     c.Status,
			CPUPercent: cpuPct,
			LogRate:    logRate,
			Ports:      ports,
		})
	}

	// Convert to JSON
	log.Printf("[TRACE] Marshalling JSON...")
	t3 := time.Now()
	jsonOutput, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	log.Printf("[TRACE] Marshalled JSON in %dms", time.Since(t3).Milliseconds())

	log.Printf("[TRACE] handleListContainers COMPLETE in %dms", time.Since(startTime).Milliseconds())

	return &protocol.CallToolResult{
		Content: []protocol.Content{
			&protocol.TextContent{
				Type: "text",
				Text: string(jsonOutput),
			},
		},
	}, nil
}

// handleGetLogs implements the get_logs tool
func (s *MCPServer) handleGetLogs(ctx context.Context, request *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	// Record MCP activity
	sessionID := getSessionID(ctx)
	s.recordActivity(sessionID)
	log.Printf("Tool: %s", request.Name)

	args := new(GetLogsArgs)
	if err := protocol.VerifyAndUnmarshal(request.RawArguments, args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Set defaults
	if args.Lines == 0 {
		args.Lines = 100
	}
	if args.Lines > 10000 {
		args.Lines = 10000
	}

	// Match containers by name, or get ALL containers if none specified
	var containers []types.Container
	var err error

	if len(args.Containers) == 0 {
		// No containers specified - search across ALL containers
		containers, err = loadContainersSync(s.dockerClient)
		if err != nil {
			return nil, fmt.Errorf("failed to load containers: %w", err)
		}
	} else {
		// Specific containers requested
		containers, err = matchContainersByName(s.dockerClient, args.Containers)
		if err != nil {
			return nil, fmt.Errorf("failed to match containers: %w", err)
		}
	}

	if len(containers) == 0 {
		return &protocol.CallToolResult{
			Content: []protocol.Content{
				&protocol.TextContent{
					Type: "text",
					Text: "No containers found",
				},
			},
		}, nil
	}

	// Compile regex if needed
	var filterRegex *regexp.Regexp
	if args.Filter != "" && args.IsRegex {
		filterRegex, err = regexp.Compile("(?i)" + args.Filter)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %w", err)
		}
	}

	// Fetch logs from LogBroker
	containerIDs := make([]string, len(containers))
	for i, c := range containers {
		containerIDs[i] = c.ID
	}

	tailLines := fmt.Sprintf("%d", args.Lines)
	logsMap := s.logBroker.FetchRecentLogs(containerIDs, tailLines)

	// Build output
	var output strings.Builder
	for _, c := range containers {
		name := getContainerName(c)
		output.WriteString(fmt.Sprintf("=== Container: %s ===\n", name))

		logLines, ok := logsMap[c.ID]
		if !ok || len(logLines) == 0 {
			output.WriteString("(no logs available)\n\n")
			continue
		}

		// Filter logs if requested
		filtered := logLines
		if args.Filter != "" {
			filtered = []string{}
			for _, line := range logLines {
				content := stripAnsiCodes(line)
				if filterRegex != nil {
					if filterRegex.MatchString(content) {
						filtered = append(filtered, line)
					}
				} else {
					if strings.Contains(strings.ToLower(content), strings.ToLower(args.Filter)) {
						filtered = append(filtered, line)
					}
				}
			}
		}

		if len(filtered) == 0 {
			output.WriteString("(no matching logs)\n\n")
		} else {
			for _, line := range filtered {
				output.WriteString(fmt.Sprintf("[%s] %s\n", name, line))
			}
			output.WriteString("\n")
		}
	}

	return &protocol.CallToolResult{
		Content: []protocol.Content{
			&protocol.TextContent{
				Type: "text",
				Text: output.String(),
			},
		},
	}, nil
}

// handleGetStats implements the get_stats tool
func (s *MCPServer) handleGetStats(ctx context.Context, request *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	// Record MCP activity
	sessionID := getSessionID(ctx)
	s.recordActivity(sessionID)
	log.Printf("Tool: %s", request.Name)

	args := new(GetStatsArgs)
	if err := protocol.VerifyAndUnmarshal(request.RawArguments, args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Match containers
	containers, err := matchContainersByName(s.dockerClient, args.Containers)
	if err != nil {
		return nil, fmt.Errorf("failed to match containers: %w", err)
	}

	if len(containers) == 0 {
		return &protocol.CallToolResult{
			Content: []protocol.Content{
				&protocol.TextContent{
					Type: "text",
					Text: "No containers found matching the specified names",
				},
			},
		}, nil
	}

	// Fetch CPU stats
	cpuStats, _ := fetchCPUStatsSync(s.dockerClient, containers)

	// Build result
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

	var result []StatsInfo
	for _, c := range containers {
		name := getContainerName(c)

		// Get CPU stats
		cpuPct := "0.0"
		var cpuHistory []float64
		if cpu, ok := cpuStats[c.ID]; ok && len(cpu) > 0 {
			cpuPct = fmt.Sprintf("%.1f", cpu[len(cpu)-1])
			if args.History {
				cpuHistory = cpu
			}
		}

		// Get log rate
		logRate := "0.0"
		if s.rateTracker != nil {
			rate := s.rateTracker.GetRate(c.ID)
			logRate = fmt.Sprintf("%.1f", rate)
		}

		result = append(result, StatsInfo{
			ID:         c.ID[:12],
			Name:       name,
			State:      c.State,
			CPUPercent: cpuPct,
			CPUHistory: cpuHistory,
			LogRate:    logRate,
			Status:     c.Status,
			Ports:      formatPortsForMCP(c.Ports),
		})
	}

	jsonOutput, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return &protocol.CallToolResult{
		Content: []protocol.Content{
			&protocol.TextContent{
				Type: "text",
				Text: string(jsonOutput),
			},
		},
	}, nil
}

// handleStartContainer implements the start_container tool
func (s *MCPServer) handleStartContainer(ctx context.Context, request *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	// Record MCP activity
	sessionID := getSessionID(ctx)
	s.recordActivity(sessionID)
	log.Printf("Tool: %s", request.Name)

	args := new(ContainerActionArgs)
	if err := protocol.VerifyAndUnmarshal(request.RawArguments, args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	containers, err := matchContainersByName(s.dockerClient, args.Containers)
	if err != nil {
		return nil, fmt.Errorf("failed to match containers: %w", err)
	}

	if len(containers) == 0 {
		return &protocol.CallToolResult{
			Content: []protocol.Content{
				&protocol.TextContent{
					Type: "text",
					Text: "No containers found matching the specified names",
				},
			},
		}, nil
	}

	var results []string
	for _, c := range containers {
		name := getContainerName(c)
		if c.State == "running" {
			results = append(results, fmt.Sprintf("✓ %s: already running", name))
			continue
		}

		if err := s.dockerClient.ContainerStart(ctx, c.ID, container.StartOptions{}); err != nil {
			results = append(results, fmt.Sprintf("✗ %s: %v", name, err))
		} else {
			results = append(results, fmt.Sprintf("✓ %s: started successfully", name))
		}
	}

	return &protocol.CallToolResult{
		Content: []protocol.Content{
			&protocol.TextContent{
				Type: "text",
				Text: strings.Join(results, "\n"),
			},
		},
	}, nil
}

// handleStopContainer implements the stop_container tool
func (s *MCPServer) handleStopContainer(ctx context.Context, request *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	// Record MCP activity
	sessionID := getSessionID(ctx)
	s.recordActivity(sessionID)
	log.Printf("Tool: %s", request.Name)

	args := new(ContainerActionArgs)
	if err := protocol.VerifyAndUnmarshal(request.RawArguments, args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	containers, err := matchContainersByName(s.dockerClient, args.Containers)
	if err != nil {
		return nil, fmt.Errorf("failed to match containers: %w", err)
	}

	if len(containers) == 0 {
		return &protocol.CallToolResult{
			Content: []protocol.Content{
				&protocol.TextContent{
					Type: "text",
					Text: "No containers found matching the specified names",
				},
			},
		}, nil
	}

	timeout := 10
	var results []string
	for _, c := range containers {
		name := getContainerName(c)
		if c.State != "running" {
			results = append(results, fmt.Sprintf("✓ %s: already stopped", name))
			continue
		}

		if err := s.dockerClient.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout}); err != nil {
			results = append(results, fmt.Sprintf("✗ %s: %v", name, err))
		} else {
			results = append(results, fmt.Sprintf("✓ %s: stopped successfully", name))
		}
	}

	return &protocol.CallToolResult{
		Content: []protocol.Content{
			&protocol.TextContent{
				Type: "text",
				Text: strings.Join(results, "\n"),
			},
		},
	}, nil
}

// handleRestartContainer implements the restart_container tool
func (s *MCPServer) handleRestartContainer(ctx context.Context, request *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	// Record MCP activity
	sessionID := getSessionID(ctx)
	s.recordActivity(sessionID)
	log.Printf("Tool: %s", request.Name)

	args := new(ContainerActionArgs)
	if err := protocol.VerifyAndUnmarshal(request.RawArguments, args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	containers, err := matchContainersByName(s.dockerClient, args.Containers)
	if err != nil {
		return nil, fmt.Errorf("failed to match containers: %w", err)
	}

	if len(containers) == 0 {
		return &protocol.CallToolResult{
			Content: []protocol.Content{
				&protocol.TextContent{
					Type: "text",
					Text: "No containers found matching the specified names",
				},
			},
		}, nil
	}

	timeout := 10
	var results []string
	for _, c := range containers {
		name := getContainerName(c)
		if err := s.dockerClient.ContainerRestart(ctx, c.ID, container.StopOptions{Timeout: &timeout}); err != nil {
			results = append(results, fmt.Sprintf("✗ %s: %v", name, err))
		} else {
			results = append(results, fmt.Sprintf("✓ %s: restarted successfully", name))
		}
	}

	return &protocol.CallToolResult{
		Content: []protocol.Content{
			&protocol.TextContent{
				Type: "text",
				Text: strings.Join(results, "\n"),
			},
		},
	}, nil
}

// Helper functions

// loadContainersSync loads all containers synchronously
func loadContainersSync(cli *client.Client) ([]types.Container, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	// Sort by name
	sort.Slice(containers, func(i, j int) bool {
		// CRITICAL FIX: Protect against empty Names slice (Docker edge case)
		nameI := ""
		if len(containers[i].Names) > 0 {
			nameI = strings.TrimPrefix(containers[i].Names[0], "/")
		}
		nameJ := ""
		if len(containers[j].Names) > 0 {
			nameJ = strings.TrimPrefix(containers[j].Names[0], "/")
		}
		return nameI < nameJ
	})

	return containers, nil
}

// fetchCPUStatsSync fetches CPU stats synchronously
func fetchCPUStatsSync(cli *client.Client, containers []types.Container) (map[string][]float64, error) {
	cpuStats := make(map[string][]float64)

	for _, c := range containers {
		if c.State != "running" {
			continue
		}

		// CRITICAL FIX: Use closure to ensure Body.Close() happens after EACH iteration, not at function end
		// defer inside loop defers until function returns, causing massive FD leak!
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			stats, err := cli.ContainerStats(ctx, c.ID, false)
			if err != nil {
				return
			}
			// CRITICAL FIX: Close body after draining to prevent FD leak on error/timeout
			defer func() {
				io.Copy(io.Discard, stats.Body) // Drain any remaining data
				stats.Body.Close()
			}()

			var v container.StatsResponse
			if err := json.NewDecoder(stats.Body).Decode(&v); err != nil {
				return
			}

			// Calculate CPU percentage
			cpuPercent := calculateCPUPercent(&v)
			cpuStats[c.ID] = []float64{cpuPercent}
		}()
	}

	return cpuStats, nil
}

// calculateCPUPercent calculates CPU usage percentage
func calculateCPUPercent(stats *container.StatsResponse) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)

	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent := (cpuDelta / systemDelta) * float64(len(stats.CPUStats.CPUUsage.PercpuUsage)) * 100.0
		return cpuPercent
	}
	return 0.0
}

// matchContainersByName finds containers by partial name match
func matchContainersByName(cli *client.Client, names []string) ([]types.Container, error) {
	all, err := loadContainersSync(cli)
	if err != nil {
		return nil, err
	}

	var matched []types.Container
	for _, name := range names {
		nameLower := strings.ToLower(name)
		for _, c := range all {
			containerName := strings.ToLower(getContainerName(c))
			containerID := strings.ToLower(c.ID)

			if strings.Contains(containerName, nameLower) || strings.Contains(containerID, nameLower) {
				matched = append(matched, c)
				break
			}
		}
	}

	return matched, nil
}

// formatPortsForMCP formats port bindings for MCP output
func formatPortsForMCP(ports []types.Port) string {
	if len(ports) == 0 {
		return ""
	}

	var parts []string
	for _, p := range ports {
		if p.PublicPort > 0 {
			parts = append(parts, fmt.Sprintf("%d:%d/%s", p.PublicPort, p.PrivatePort, p.Type))
		} else {
			parts = append(parts, fmt.Sprintf("%d/%s", p.PrivatePort, p.Type))
		}
	}

	return strings.Join(parts, ", ")
}

// getSessionID extracts or generates a session ID from context
// Uses a hash of the context to create a pseudo-session identifier
func getSessionID(ctx context.Context) string {
	// Try to get a session identifier from context
	// If not available, create a simple hash based on timestamp and context pointer
	// This will group requests from the same general timeframe/client
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%p-%d", ctx, time.Now().Unix()/10))) // Group by 10-second windows
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// stripAnsiCodes is already defined in model.go
