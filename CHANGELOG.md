# Changelog

All notable changes to Docker TUI will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- **BREAKING**: Keyboard shortcut for Stop changed from `P` to `K` (Kill)
- **BREAKING**: Keyboard shortcut for Pause changed from `U` to `P`
- Removed vim-style navigation shortcuts (`j`/`k`) - use arrow keys instead
- Removed gray background from status bar for cleaner visual appearance

### Added
- MCP server status display in header bar (shows "MCP: ON (:port)" when active)
- Comprehensive keyboard shortcuts documentation in CLAUDE.md
- Status bar section in documentation

### Improved
- Keyboard shortcuts now follow more intuitive Unix-style conventions:
  - `S` - Start
  - `K` - Kill (Stop)
  - `R` - Restart
  - `P` - Pause/Unpause
  - `D` - Remove (Delete)
- Help text updated to show `[K] Kill (Stop)` for clarity

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
