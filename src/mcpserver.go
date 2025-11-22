package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
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

// mcpCustomLogger implements pkg.Logger interface and writes to both file and buffer
type mcpCustomLogger struct {
	logBuffer *MCPLogBuffer
	logFile   *os.File
}

func (l *mcpCustomLogger) Debugf(format string, a ...any) {
	msg := fmt.Sprintf("[Debug] "+format, a...)
	l.writeLog(msg)
}

func (l *mcpCustomLogger) Infof(format string, a ...any) {
	msg := fmt.Sprintf("[Info] "+format, a...)
	l.writeLog(msg)
}

func (l *mcpCustomLogger) Warnf(format string, a ...any) {
	msg := fmt.Sprintf("[Warn] "+format, a...)
	l.writeLog(msg)
}

func (l *mcpCustomLogger) Errorf(format string, a ...any) {
	msg := fmt.Sprintf("[Error] "+format, a...)
	l.writeLog(msg)
}

func (l *mcpCustomLogger) writeLog(msg string) {
	// Write to buffer (for UI display)
	l.logBuffer.Add(msg)

	// Write to file with timestamp
	if l.logFile != nil {
		timestamp := time.Now().Format("2006/01/02 15:04:05")
		fmt.Fprintf(l.logFile, "%s %s\n", timestamp, msg)
	}
}

// MCPLogBuffer stores MCP server logs in a circular buffer
type MCPLogBuffer struct {
	logs   []string
	mu     sync.RWMutex
	maxLen int
}

// NewMCPLogBuffer creates a new log buffer
func NewMCPLogBuffer(maxLen int) *MCPLogBuffer {
	return &MCPLogBuffer{
		logs:   make([]string, 0, maxLen),
		maxLen: maxLen,
	}
}

// Add adds a log entry to the buffer (circular, oldest removed if full)
func (b *MCPLogBuffer) Add(entry string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Remove timestamp prefix if present and clean up
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return
	}

	// Add timestamp
	timestamped := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), entry)

	// Add to buffer
	b.logs = append(b.logs, timestamped)

	// Remove oldest if exceeds max
	if len(b.logs) > b.maxLen {
		b.logs = b.logs[1:]
	}
}

// GetLogs returns a copy of all logs
func (b *MCPLogBuffer) GetLogs() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Return copy
	logsCopy := make([]string, len(b.logs))
	copy(logsCopy, b.logs)
	return logsCopy
}

// mcpLogWriter implements io.Writer to capture log output
type mcpLogWriter struct {
	buffer       *MCPLogBuffer
	originalOut  io.Writer
	suppressOut  bool // If true, don't write to original output
	logFile      *os.File // Also write to file
}

func (w *mcpLogWriter) Write(p []byte) (n int, err error) {
	// Add to buffer
	w.buffer.Add(string(p))

	// Also write to file if available
	if w.logFile != nil {
		w.logFile.Write(p)
	}

	// Also write to original output if not suppressed
	if !w.suppressOut && w.originalOut != nil {
		return w.originalOut.Write(p)
	}

	return len(p), nil
}

// MCPServer manages the MCP HTTP server with StreamableHTTPServerTransport
type MCPServer struct {
	dockerClient      *client.Client
	logBroker         *LogBroker
	rateTracker       *RateTrackerConsumer
	cpuCache          *CPUStatsCache       // CPU stats cache for instant responses
	mcpServer         *server.Server
	httpServer        *http.Server
	port              int
	shutdownCtx       context.Context
	shutdownCancel    context.CancelFunc
	activeSessions    map[string]time.Time // Track active client sessions (sessionID -> lastSeen)
	sessionsmu        sync.RWMutex         // Protect activeSessions map
	logBuffer         *MCPLogBuffer        // Buffer for MCP server logs
	originalLogger    *log.Logger          // Original logger to restore on shutdown
}

// NewMCPServer creates a new MCP server instance using go-mcp with StreamableHTTPServerTransport
func NewMCPServer(dockerClient *client.Client, logBroker *LogBroker, rateTracker *RateTrackerConsumer, cpuCache *CPUStatsCache, port int) (*MCPServer, error) {
	// Create log buffer (keep last 50 entries)
	logBuffer := NewMCPLogBuffer(50)

	// Save original stderr
	originalLogger := log.New(os.Stderr, "", log.LstdFlags)

	// Open log file
	logFile, err := os.OpenFile("/tmp/mcp-debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Create custom writer that captures logs in buffer AND file, but suppresses stdout
	logWriter := &mcpLogWriter{
		buffer:      logBuffer,
		originalOut: nil, // Set to nil to suppress stdout for MCP logs
		suppressOut: true,
		logFile:     logFile, // Write to file
	}

	// Redirect standard log output to our custom writer
	log.SetOutput(logWriter)
	log.SetFlags(0) // We add our own timestamps in MCPLogBuffer.Add()

	s := &MCPServer{
		dockerClient:   dockerClient,
		logBroker:      logBroker,
		rateTracker:    rateTracker,
		cpuCache:       cpuCache,
		port:           port,
		activeSessions: make(map[string]time.Time),
		logBuffer:      logBuffer,
		originalLogger: originalLogger,
	}

	customLogger := &mcpCustomLogger{
		logBuffer: logBuffer,
		logFile:   logFile,
	}

	// Create StreamableHTTPServerTransport (stateful mode with SSE support)
	mcpTransport := transport.NewStreamableHTTPServerTransport(
		fmt.Sprintf(":%d", port),
		transport.WithStreamableHTTPServerTransportOptionEndpoint("/mcp"),
		transport.WithStreamableHTTPServerTransportOptionStateMode(transport.Stateful),
		transport.WithStreamableHTTPServerTransportOptionLogger(customLogger),
	)

	// CRITICAL: Re-apply log redirection after transport creation
	// The transport may have reset the logger during initialization
	log.SetOutput(logWriter)
	log.SetFlags(0)

	// Create MCP server with metadata
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

// GetLogs returns the MCP server logs
func (s *MCPServer) GetLogs() []string {
	if s.logBuffer == nil {
		return []string{}
	}
	return s.logBuffer.GetLogs()
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
