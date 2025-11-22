package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
	"github.com/docker/docker/client"
)

// Version and BuildTime are set via ldflags during build
var Version = "dev"
var BuildTime = "unknown"

// MCPServer manages the MCP HTTP server with StreamableHTTPServerTransport
type MCPServer struct {
	dockerClient      *client.Client
	logBroker         *LogBroker
	rateTracker       *RateTrackerConsumer
	mcpServer         *server.Server
	httpServer        *http.Server
	port              int
	shutdownCtx       context.Context
	shutdownCancel    context.CancelFunc
	activeSessions    map[string]time.Time // Track active client sessions (sessionID -> lastSeen)
	sessionsmu        sync.RWMutex         // Protect activeSessions map
}

// NewMCPServer creates a new MCP server instance using go-mcp with StreamableHTTPServerTransport
func NewMCPServer(dockerClient *client.Client, logBroker *LogBroker, rateTracker *RateTrackerConsumer, port int) (*MCPServer, error) {
	s := &MCPServer{
		dockerClient:   dockerClient,
		logBroker:      logBroker,
		rateTracker:    rateTracker,
		port:           port,
		activeSessions: make(map[string]time.Time),
	}

	// Create StreamableHTTPServerTransport (stateful mode with SSE support)
	mcpTransport := transport.NewStreamableHTTPServerTransport(
		fmt.Sprintf(":%d", port),
		transport.WithStreamableHTTPServerTransportOptionEndpoint("/mcp"),
		transport.WithStreamableHTTPServerTransportOptionStateMode(transport.Stateful),
	)

	// Create MCP server with metadata
	var err error
	s.mcpServer, err = server.NewServer(
		mcpTransport,
		server.WithServerInfo(protocol.Implementation{
			Name:    "docker-tui-mcp",
			Version: Version,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP server: %w", err)
	}

	// Register tools
	if err := s.registerTools(); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}

	// Setup custom HTTP server with health check endpoint
	mux := http.NewServeMux()

	// MCP endpoint (handled by go-mcp transport)
	// Note: The StreamableHTTPServerTransport creates its own HTTP server internally
	// We add a health check on a separate path
	mux.HandleFunc("/health", s.handleHealth)

	return s, nil
}

// Start starts the MCP server (blocking call)
func (s *MCPServer) Start() error {
	log.Printf("MCP HTTP server listening on :%d/mcp (StreamableHTTPServerTransport, stateful mode with SSE support)\n", s.port)

	// CRITICAL FIX: Create cancellable context for graceful shutdown
	s.shutdownCtx, s.shutdownCancel = context.WithCancel(context.Background())

	// Start LogBroker streaming for all running containers
	go func() {
		containers, err := loadContainersSync(s.dockerClient)
		if err == nil {
			s.logBroker.StartStreaming(containers)
		}

		// Refresh container list every 5 seconds
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		// Cleanup stale MCP sessions every 10 seconds
		cleanupTicker := time.NewTicker(10 * time.Second)
		defer cleanupTicker.Stop()

		for {
			select {
			case <-ticker.C:
				// CRITICAL FIX: Check context to prevent access after shutdown
				if s.shutdownCtx.Err() != nil {
					return
				}
				containers, err := loadContainersSync(s.dockerClient)
				if err == nil {
					s.logBroker.StartStreaming(containers)
				}
			case <-cleanupTicker.C:
				// Clean up stale MCP sessions
				s.cleanupStaleSessions()
			case <-s.shutdownCtx.Done():
				// CRITICAL FIX: Exit goroutine cleanly on shutdown
				return
			}
		}
	}()

	// Start MCP server - this is blocking and creates its own HTTP server
	return s.mcpServer.Run()
}

// Shutdown gracefully shuts down the MCP server
func (s *MCPServer) Shutdown(ctx context.Context) error {
	log.Println("Shutting down MCP server...")
	// CRITICAL FIX: Cancel background goroutine before shutting down server
	if s.shutdownCancel != nil {
		s.shutdownCancel()
	}
	return s.mcpServer.Shutdown(ctx)
}

// GetPort returns the MCP server port
func (s *MCPServer) GetPort() int {
	return s.port
}

// GetConnectedClients returns the number of currently connected MCP clients
// Sessions are considered active if they had activity in the last 30 seconds
func (s *MCPServer) GetConnectedClients() int {
	s.sessionsmu.RLock()
	defer s.sessionsmu.RUnlock()

	// Clean up stale sessions (no activity for 30 seconds)
	now := time.Now()
	activeCount := 0
	for _, lastSeen := range s.activeSessions {
		if now.Sub(lastSeen) < 30*time.Second {
			activeCount++
		}
	}
	return activeCount
}

// recordActivity records activity for a client session
func (s *MCPServer) recordActivity(sessionID string) {
	s.sessionsmu.Lock()
	defer s.sessionsmu.Unlock()

	wasNew := false
	if _, exists := s.activeSessions[sessionID]; !exists {
		wasNew = true
	}
	s.activeSessions[sessionID] = time.Now()

	if wasNew {
		log.Printf("MCP new session: %s (total active: %d)", sessionID[:8], len(s.activeSessions))
	}
}

// cleanupStaleSessions removes sessions with no activity in the last 30 seconds
func (s *MCPServer) cleanupStaleSessions() {
	s.sessionsmu.Lock()
	defer s.sessionsmu.Unlock()

	now := time.Now()
	for sessionID, lastSeen := range s.activeSessions {
		if now.Sub(lastSeen) > 30*time.Second {
			delete(s.activeSessions, sessionID)
			log.Printf("MCP session expired: %s (total active: %d)", sessionID[:8], len(s.activeSessions))
		}
	}
}

// handleHealth responds to GET /health requests
func (s *MCPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method is supported", http.StatusMethodNotAllowed)
		return
	}

	// Load container count for health response
	containers, err := loadContainersSync(s.dockerClient)
	containerCount := len(containers)

	status := "healthy"
	if err != nil {
		status = "degraded"
	}

	response := map[string]interface{}{
		"status":          status,
		"version":         Version,
		"build_time":      BuildTime,
		"container_count": containerCount,
		"goroutines":      getGoroutineCount(),   // Monitor for goroutine leaks
		"file_descriptors": countOpenFDs(),       // Monitor for FD leaks
		"tools":           6,                     // get_logs, list_containers, get_stats, start_container, stop_container, restart_container
		"protocol":        "MCP",
		"transport":       "StreamableHTTPServerTransport (stateful, SSE)",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// registerTools registers all MCP tools
func (s *MCPServer) registerTools() error {
	// Register list_containers tool
	listContainersTool, err := protocol.NewTool(
		"list_containers",
		"List all Docker containers with status and resource usage",
		ListContainersArgs{},
	)
	if err != nil {
		return fmt.Errorf("failed to create list_containers tool: %w", err)
	}
	s.mcpServer.RegisterTool(listContainersTool, s.handleListContainers)

	// Register get_logs tool
	getLogsTool, err := protocol.NewTool(
		"get_logs",
		"Fetch Docker container logs with optional filtering",
		GetLogsArgs{},
	)
	if err != nil {
		return fmt.Errorf("failed to create get_logs tool: %w", err)
	}
	s.mcpServer.RegisterTool(getLogsTool, s.handleGetLogs)

	// Register get_stats tool
	getStatsTool, err := protocol.NewTool(
		"get_stats",
		"Get detailed resource statistics for specific containers",
		GetStatsArgs{},
	)
	if err != nil {
		return fmt.Errorf("failed to create get_stats tool: %w", err)
	}
	s.mcpServer.RegisterTool(getStatsTool, s.handleGetStats)

	// Register start_container tool
	startContainerTool, err := protocol.NewTool(
		"start_container",
		"Start one or more stopped Docker containers",
		ContainerActionArgs{},
	)
	if err != nil {
		return fmt.Errorf("failed to create start_container tool: %w", err)
	}
	s.mcpServer.RegisterTool(startContainerTool, s.handleStartContainer)

	// Register stop_container tool
	stopContainerTool, err := protocol.NewTool(
		"stop_container",
		"Stop one or more running Docker containers",
		ContainerActionArgs{},
	)
	if err != nil {
		return fmt.Errorf("failed to create stop_container tool: %w", err)
	}
	s.mcpServer.RegisterTool(stopContainerTool, s.handleStopContainer)

	// Register restart_container tool
	restartContainerTool, err := protocol.NewTool(
		"restart_container",
		"Restart one or more Docker containers",
		ContainerActionArgs{},
	)
	if err != nil {
		return fmt.Errorf("failed to create restart_container tool: %w", err)
	}
	s.mcpServer.RegisterTool(restartContainerTool, s.handleRestartContainer)

	return nil
}
