# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Docker TUI is a Terminal User Interface application for managing Docker containers, built with Go and Bubbletea (The Elm Architecture framework). It provides real-time container monitoring, multi-select operations, CPU usage tracking, live log streaming with colored backgrounds, regex-based filtering, and a demo mode for presentations.

## Build and Development Commands

```bash
# Build for current platform
make build              # Creates ./docker-tui binary

# Run without building
make run               # Executes: go run ./src

# Run tests
make test              # Executes: go test -v ./src/...

# Format code
make fmt               # Executes: go fmt ./src/...

# Clean artifacts
make clean             # Removes ./docker-tui and dist/

# Cross-platform builds
make dist-all          # Builds for Linux (amd64/arm64), macOS (amd64/arm64), Windows
```

## Architecture: Elm Architecture (Bubbletea)

The application follows Bubbletea's Elm Architecture pattern with three core components:

### Model (state)
Located in [src/model.go](src/model.go), the `model` struct contains all application state:
- Docker client and container list
- Selection state (`selected` map, `cursor` position)
- View mode (`listView`, `logsView`, `confirmView`, `exitConfirmView`)
- Processing state for async operations
- CPU statistics tracking (current + 10-value history)
- LogBroker instance (`logBroker`) for centralized log streaming management
- RateTracker instance (`rateTracker`) for L/S (lines/second) monitoring
- BufferConsumer instance (`bufferConsumer`) for logs view circular buffering
- Filter state (`filterMode`, `filterInput`, `filterActive`, `filterRegex`, `filterIsRegex`)
- Demo mode flag (`demoMode`) for hiding container name prefixes
- Closing flag (`closing atomic.Bool`) for graceful shutdown coordination

### Update (message handling)
Main update loop in [src/model.go](src/model.go) processes messages:
- `tea.KeyMsg` / `tea.MouseMsg` → [src/handlers.go](src/handlers.go) for input handling
- `containerListMsg` → Update container list from Docker API
- `cpuStatsMsg` → Update CPU stats (tracked via delta calculation between calls)
- `tickMsg` → Trigger container list refresh (every 5 seconds)
- `cpuTickMsg` → Trigger CPU stats refresh (every 5 seconds)
- `newLogLineMsg` → Notification from BufferConsumer when new log line arrives (triggers UI refresh)
- `toastMsg` → Display success/error notifications (auto-clear after 3 seconds)

### View (rendering)
Rendering functions in [src/render.go](src/render.go):
- `renderList()` → Main container list with visual column separators, status bar, filter indicator, and help text
- `renderLogs()` → Log viewer with colored container backgrounds, aligned names, scroll position indicator
- `renderConfirm()` → Confirmation dialogs (destructive actions)
- `renderFilterBar()` → Filter input bar with validation highlighting
- `getContainerLogColor()` → Hash-based consistent color assignment per container

Styles defined in [src/styles.go](src/styles.go) using Lipgloss. Column separator color: `#3c3c3c` (dark gray).

## Key Implementation Details

### Docker Operations
All Docker operations in [src/docker.go](src/docker.go):
- `loadContainers()` → Fetch and sort all containers by name
- `performAction()` → Execute start/stop/restart/remove on selected containers
- `streamLogs()` → Multi-container log streaming with automatic reconnection
- `fetchCPUStats()` → Parallel CPU stats fetching for running containers

### CPU Calculation
CPU percentage calculation (can exceed 100%):
- Uses manual delta tracking between consecutive stats calls
- **CRITICAL**: Never use `PreCPUStats` in oneshot mode (contains stale data from container start)
- Formula: `(cpuDelta / systemDelta) * numCPUs * 100.0`
- Stores previous stats in `model.cpuPrevStats` for next calculation
- See `calculateCPUPercentWithHistory()` in [src/docker.go:348-385](src/docker.go#L348-L385)

### Log Streaming Architecture (v1.3.0+)

**LogBroker Pattern** ([src/logbroker.go](src/logbroker.go)):
- Centralized log streaming broker that manages all container log streams
- Implements permanent streaming that persists across view changes
- Starts streaming automatically for all running containers on app startup
- Uses goroutines per container with automatic reconnection on stream failure
- Parses Docker's multiplexed stream format (8-byte header + payload)
- First connection fetches last 50 lines, reconnections fetch only new logs
- Distributes log lines to multiple consumers via `LogConsumer` interface
- Tracks active streams in `activeStreams` map with context cancellation

**Consumer Pattern**:
- `LogConsumer` interface: `OnLogLine(containerID, containerName, line, timestamp)` and `OnContainerStatusChange(containerID, isRunning)`
- `RateTrackerConsumer` ([src/ratetracker.go](src/ratetracker.go)): Tracks log rate (lines/second) for L/S column display, always active
- `BufferConsumer` ([src/bufferconsumer.go](src/bufferconsumer.go)): Circular buffer for logs view, registered when entering logs view, unregistered on exit

**Auto-scroll Behavior**:
- In logs view, automatically scrolls to bottom when new log line arrives
- Only scrolls if user is already at bottom (respects manual scroll position)
- Triggered by `newLogLineMsg` from BufferConsumer callback

### Selection Behavior
- SPACE toggles individual selection
- Shift+Up/Down for range selection (tracks `shiftStart` position)
- A selects all containers, Ctrl+A selects only running containers
- X clears selection, I inverts selection
- Single-click toggles selection, double-click shows logs
- Cursor position tracked separately from selection state

### Filtering System
**Container List Filtering** (regex with validation):
- Press `/` to enter filter mode (sets `filterMode = true`)
- Input is compiled as case-insensitive regex: `regexp.Compile("(?i)" + input)`
- Invalid regex patterns: `filterIsRegex = false` triggers red background highlighting
- Applied to container names via `containerMatchesFilter()` in [src/model.go](src/model.go)
- Filter active indicator shown in title bar with 🔍 icon
- Hidden container count displayed in stats bar

**Log Filtering** (substring search):
- Always uses case-insensitive substring search (not regex)
- Strips ANSI codes before matching via `stripAnsiCodes()` to avoid false matches
- Real-time filtering as user types (instant feedback)
- Applied via `logLineMatchesFilter()` in [src/model.go](src/model.go)
- Scroll resets to top when filter changes, to bottom when cleared

### Demo Mode
Activated via `--demo` command line flag:
- Hides container name prefixes up to and including first underscore
- Example: `project_nginx` displays as `nginx`
- Implemented in `cleanContainerName()` helper in [src/model.go](src/model.go)
- Applied in both list view and logs view for consistency
- Used for width calculations to ensure proper alignment
- Color hashing still uses original name for consistency

### Column Structure (v1.3.0+)
**Container List Columns** (left to right):
- NAME (35 chars, left-aligned) - container name with demo mode support
- STATE (13 chars, left-aligned) - icon + state name with color coding
- CPU (7 chars, right-aligned) - percentage with color: teal (<30%), yellow (30-70%), red (>70%)
- L/S (6 chars, right-aligned) - log rate (lines/second): 0-9999, formatted as "0", "0.5", "12", "1.2k"
- UPTIME (7 chars, right-aligned) - uptime for running containers, exit time for stopped
- PORTS (variable width) - exposed ports with range detection

**Column Separators**:
- Dark gray vertical bars (`│`) between all columns for visual clarity
- Color: `#3c3c3c` defined as `colorSeparator` in [src/styles.go](src/styles.go)
- Format: `column + space + sep + space` (except last column)

**Log View Alignment**:
- `logsViewMaxNameWidth` calculated when entering logs view
- Applies to all selected containers (finds longest cleaned name)
- Container names are padded with spaces to match max width
- Format: `containerName │ log content` with consistent colored background per container

### Message Flow for Async Operations
When performing container actions (start/stop/restart):
1. Mark containers as processing: `m.processing[id] = true`
2. Return `tea.Cmd` that executes Docker operation in background
3. Clear processing state after operation completes
4. Return `tea.Batch()` with both `loadContainers()` and `toastMsg` commands
5. Toast displays success/failure and auto-clears after 3 seconds

## Code Structure

All source code is in `src/` directory:
- [main.go](src/main.go) → Entry point, initializes Docker client, LogBroker, RateTracker, and Bubbletea program
- [model.go](src/model.go) → Model definition, Init(), Update(), View() with Elm Architecture message handling
- [handlers.go](src/handlers.go) → Keyboard and mouse input handlers for all views
- [docker.go](src/docker.go) → All Docker API interactions and commands
- [logbroker.go](src/logbroker.go) → LogBroker pattern implementation with consumer distribution (v1.3.0+)
- [ratetracker.go](src/ratetracker.go) → Log rate tracking consumer for L/S column (v1.3.0+)
- [bufferconsumer.go](src/bufferconsumer.go) → Circular buffer consumer for logs view (v1.3.0+)
- [render.go](src/render.go) → View rendering functions with column separators
- [styles.go](src/styles.go) → Lipgloss styles and color constants
- [formatters.go](src/formatters.go) → Data formatting helpers (state, CPU, uptime, ports, log rate)

## Version Management

Version and build time are injected at compile time via ldflags in Makefile:
```bash
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"
```

## Release Process

Uses GoReleaser (`.goreleaser.yml`) for automated multi-platform releases. Configuration uses v2 syntax:
- Builds for Linux (amd64/arm64), macOS (amd64/arm64), Windows (amd64)
- Creates tar.gz archives (zip for Windows)
- Generates checksums and GitHub release notes
