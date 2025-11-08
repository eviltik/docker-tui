package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

func main() {
	// Recover from panics and write crash log
	defer func() {
		if r := recover(); r != nil {
			writeCrashLog(r, "main")
			os.Exit(1)
		}
	}()

	// Parse command line arguments
	demoMode := false
	debugMonitor := false
	logsBufferLength := 10000
	mcpServerMode := false
	mcpPort := 9876
	for i, arg := range os.Args[1:] {
		switch arg {
		case "--help", "-h":
			fmt.Println("Docker TUI - Terminal User Interface for Docker")
			fmt.Println()
			fmt.Println("Usage: docker-tui [OPTIONS]")
			fmt.Println()
			fmt.Println("Options:")
			fmt.Println("  --demo                      Hide container name prefixes (removes text up to first underscore)")
			fmt.Println("  --debug-monitor             Show debug metrics (goroutines, FD, memory, streams)")
			fmt.Println("  --logs-buffer-length SIZE   Maximum log lines in buffer (default: 10000)")
			fmt.Println("  --mcp-server                Enable MCP HTTP server alongside TUI (default port: 9876)")
			fmt.Println("  --mcp-port PORT             Set MCP server port (default: 9876)")
			fmt.Println("  --help, -h                  Show this help message")
			fmt.Println()
			fmt.Println("Examples:")
			fmt.Println("  docker-tui                                    Run in normal mode")
			fmt.Println("  docker-tui --demo                             Run in demo mode (clean container names)")
			fmt.Println("  docker-tui --debug-monitor                    Run with debug metrics displayed")
			fmt.Println("  docker-tui --logs-buffer-length 50000         Use 50k lines buffer")
			fmt.Println("  docker-tui --mcp-server                       Run with MCP HTTP server on port 9876 (v1.4.0+)")
			fmt.Println("  docker-tui --mcp-server --mcp-port 9000       Run with MCP server on custom port")
			fmt.Println()
			fmt.Println("Keyboard Shortcuts:")
			fmt.Println("  List View:")
			fmt.Println("    ↑/↓, k/j           Navigate containers")
			fmt.Println("    SPACE              Toggle selection")
			fmt.Println("    A                  Select all containers")
			fmt.Println("    Ctrl+A             Select running containers")
			fmt.Println("    X                  Clear selection")
			fmt.Println("    I                  Invert selection")
			fmt.Println("    ENTER, L           View logs")
			fmt.Println("    S                  Start container(s)")
			fmt.Println("    P                  Stop container(s)")
			fmt.Println("    R                  Restart container(s)")
			fmt.Println("    U                  Pause/Unpause container(s)")
			fmt.Println("    D                  Remove container(s)")
			fmt.Println("    /                  Filter containers")
			fmt.Println("    Q, ESC             Quit")
			fmt.Println()
			fmt.Println("  Logs View:")
			fmt.Println("    ↑/↓, k/j           Scroll logs")
			fmt.Println("    PgUp/PgDn          Page up/down")
			fmt.Println("    Home/End           Jump to top/bottom")
			fmt.Println("    ENTER              Insert timestamp mark")
			fmt.Println("    /                  Filter logs")
			fmt.Println("    Q, ESC             Back to list")
			fmt.Println()
			os.Exit(0)
		case "--demo":
			demoMode = true
		case "--debug-monitor":
			debugMonitor = true
		case "--logs-buffer-length":
			if i+1 < len(os.Args[1:]) {
				fmt.Sscanf(os.Args[i+2], "%d", &logsBufferLength)
				if logsBufferLength < 100 {
					logsBufferLength = 100 // Minimum 100 lines
				}
			}
		case "--mcp-server":
			mcpServerMode = true
		case "--mcp-port":
			if i+1 < len(os.Args[1:]) {
				fmt.Sscanf(os.Args[i+2], "%d", &mcpPort)
			}
		}
	}

	// Initialize Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Printf("Error creating Docker client: %v\n", err)
		os.Exit(1)
	}
	defer cli.Close()

	// Initialize LogBroker and RateTracker (shared between TUI and MCP server)
	logBroker := NewLogBroker(cli)
	rateTracker := NewRateTrackerConsumer()
	logBroker.RegisterConsumer(rateTracker)

	// CRITICAL GOROUTINE LEAK PREVENTION: Monitor goroutine count
	// Panic if count exceeds threshold to prevent accumulation crash
	safeGo("goroutine-monitor", func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			count := getGoroutineCount()

			// Log warning if count is high
			if count > 1000 {
				fmt.Fprintf(os.Stderr, "\n⚠️  WARNING: High goroutine count: %d\n", count)
			}

			// Panic if count is critically high to trigger crash log and restart
			// This prevents silent accumulation leading to deadlock (like the 145k goroutine crash)
			if count > 10000 {
				panic(fmt.Sprintf("FATAL: Goroutine leak detected - %d goroutines active (threshold: 10000)", count))
			}
		}
	})

	// Start MCP server if requested
	var mcpServer *MCPServer
	var mcpErrChan chan error
	if mcpServerMode {
		mcpServer, err = NewMCPServer(cli, logBroker, rateTracker, mcpPort)
		if err != nil {
			fmt.Printf("Error creating MCP server: %v\n", err)
			os.Exit(1)
		}

		mcpErrChan = make(chan error, 1)

		// Check if we have a TTY
		_, err := os.Open("/dev/tty")
		hasTTY := err == nil

		if !hasTTY {
			// HTTP-only mode: run MCP server without TUI
			fmt.Printf("Running in HTTP-only mode (no TTY detected)\n")
			fmt.Printf("MCP server starting on port %d...\n", mcpPort)

			// Setup signal handling
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			// Run server in goroutine with crash protection
			safeGo("mcp-server-http-only", func() {
				if err := mcpServer.Start(); err != nil {
					mcpErrChan <- err
				}
			})

			// Wait for signal or error
			select {
			case <-sigChan:
				fmt.Println("\nShutting down...")
			case err := <-mcpErrChan:
				fmt.Printf("\n\033[31mFailed to start MCP server: %v\033[0m\n", err)
				fmt.Printf("\nPlease check:\n")
				fmt.Printf("  - Port %d is not already in use (try: lsof -i:%d)\n", mcpPort, mcpPort)
				fmt.Printf("  - You have permission to bind to the port\n")
				fmt.Printf("\nTry using a different port with --mcp-port <port>\n\n")
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				mcpServer.Shutdown(ctx)
				os.Exit(1)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			mcpServer.Shutdown(ctx)
			return
		}

		// Start MCP server in background (TUI mode) with crash protection
		safeGo("mcp-server-tui-mode", func() {
			fmt.Printf("Starting MCP HTTP server on port %d...\n", mcpPort)
			if err := mcpServer.Start(); err != nil {
				mcpErrChan <- err
			}
		})

		// Give server time to start and check for errors
		select {
		case err := <-mcpErrChan:
			fmt.Printf("\n\033[31mFailed to start MCP server: %v\033[0m\n", err)
			fmt.Printf("\nPlease check:\n")
			fmt.Printf("  - Port %d is not already in use\n", mcpPort)
			fmt.Printf("  - You have permission to bind to the port\n")
			fmt.Printf("\nTry using a different port with --mcp-port <port>\n\n")
			os.Exit(1)
		case <-time.After(100 * time.Millisecond):
			// Server started successfully, continue
		}
	}

	// Create model - use pointer to avoid copying mutexes
	m := &model{
		dockerClient:     cli,
		containers:       []types.Container{},
		selected:         make(map[string]bool),
		processing:       make(map[string]bool),
		view:             listView,
		shiftStart:       -1,
		lastClickIndex:   -1,
		cpuStats:         make(map[string][]float64),
		cpuCurrent:       make(map[string]float64),
		cpuPrevStats:     make(map[string]*container.StatsResponse),
		demoMode:         demoMode,
		debugMonitor:     debugMonitor,
		logsBufferLength: logsBufferLength,
		logsColorEnabled: true, // Enable colored backgrounds in logs by default
		logBroker:        logBroker,
		rateTracker:      rateTracker,
	}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run the TUI program - m is already a pointer
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseAllMotion())

	// Handle shutdown in separate goroutine with crash protection
	safeGo("shutdown-handler", func() {
		<-sigChan
		// CRITICAL FIX: Stop all log streams first
		if logBroker != nil {
			logBroker.StopAll()
		}
		if mcpServer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			mcpServer.Shutdown(ctx)
		}
		p.Quit()
	})

	// Monitor MCP server errors in background (if running) with crash protection
	if mcpServerMode {
		safeGo("mcp-error-monitor", func() {
			err := <-mcpErrChan
			if err != nil {
				// MCP server crashed, quit TUI
				p.Quit()
			}
		})
	}

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		// CRITICAL FIX: Stop all log streams before shutdown
		if logBroker != nil {
			logBroker.StopAll()
		}
		if mcpServer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			mcpServer.Shutdown(ctx)
		}
		os.Exit(1)
	}

	// Clean shutdown of MCP server and log broker
	if logBroker != nil {
		logBroker.StopAll()
	}
	if mcpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mcpServer.Shutdown(ctx)
	}
}
