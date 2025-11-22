# Changelog

All notable changes to Docker TUI will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.2.0] - 2025-01-22

### Added
- **MCP Global Log Search**: `get_logs` tool can now search across ALL containers when 'containers' parameter is empty
- Enhanced MCP tool descriptions to better guide AI assistants on usage and capabilities

### Improved
- **MCP Tool Descriptions**: All 6 MCP tools now have detailed, explicit descriptions explaining:
  - What data they return (CPU%, log rate, ports, etc.)
  - Available features (filtering, partial matching, batch operations)
  - Typical use cases and best practices
- `get_logs` description emphasizes global search capability and filter usage for finding errors/warnings
- `list_containers` description details all returned metrics (status, CPU, log rate, uptime, ports)
- `get_stats` description specifies real-time metrics and optional CPU history
- Container action tools (start/stop/restart) descriptions mention timeout, partial matching, and batch support

### Changed
- `get_logs` 'containers' parameter is now optional (leave empty to search all containers)
- Parameter descriptions updated to guide AI on optional vs required usage

## [1.1.0] - 2025-01-22

### Changed
- **BREAKING**: Keyboard shortcut for Stop changed from `P` to `K` (Kill)
- **BREAKING**: Keyboard shortcut for Pause changed from `U` to `P`
- Removed vim-style navigation shortcuts (`j`/`k`) - use arrow keys instead
- Removed gray background from status bar for cleaner visual appearance

### Added
- MCP server status display in header bar (shows "MCP: ON (:port)" when active)
- Comprehensive keyboard shortcuts documentation in CLAUDE.md
- Status bar section in documentation
- CPU stats cache for instant MCP responses (shared between model and MCP server)
- MCP logs viewer accessible with `M` key when MCP server is running
- Real-time MCP client tracking with session management

### Improved
- Keyboard shortcuts now follow more intuitive Unix-style conventions:
  - `S` - Start
  - `K` - Kill (Stop)
  - `R` - Restart
  - `P` - Pause/Unpause
  - `D` - Remove (Delete)
- Help text updated to show `[K] Kill (Stop)` for clarity
- **MCP Performance**: Optimized list_containers from 25+ seconds to ~6ms using CPU cache
- **Resource Management**: Fixed critical file descriptor leaks in CPU stats fetching
- **Concurrency**: Fixed race conditions in LogBroker semaphore and container data access
- **Memory Safety**: Fixed BufferConsumer goroutine leaks on view transitions
- **Reliability**: Added defer cleanup for MCP log file on error paths

### Fixed
- Critical semaphore bypass in LogBroker allowing unlimited goroutine accumulation
- File descriptor leak in fetchCPUStats when Docker API calls timeout
- TOCTOU race condition on mcpServer nil check during rendering
- Shallow copy race in containerListMsg causing potential data corruption
- Log file descriptor leak when MCP server initialization fails
- BufferConsumer not properly cleaned up when entering logs view multiple times

## [1.0.0] - 2025-01-22

### Added
- Initial release
- Terminal User Interface for Docker container management
- Real-time container monitoring with CPU usage tracking
- Multi-select operations (start/stop/restart/pause/remove)
- Live log streaming with colored backgrounds per container
- Regex-based container filtering
- Log rate tracking (lines/second)
- Mouse support (click to select, double-click for logs)
- Demo mode for hiding container name prefixes
- Debug monitoring mode for resource tracking
- MCP server integration for Claude Desktop
- Comprehensive test coverage (49.9%, 215 tests)
