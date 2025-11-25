package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// LogConsumer is an interface for receiving logs
type LogConsumer interface {
	OnLogLine(containerID, containerName, line string, timestamp time.Time)
	OnContainerStatusChange(containerID string, isRunning bool)
}

// LogBroker centralizes log streaming and distributes to consumers
type LogBroker struct {
	dockerClient *client.Client
	consumers    []LogConsumer
	consumersMu  sync.RWMutex

	activeStreams map[string]context.CancelFunc
	streamsMu     sync.RWMutex

	containers   []types.Container
	containersMu sync.RWMutex

	// CRITICAL GOROUTINE LEAK PREVENTION: Limit concurrent read goroutines
	// Semaphore to prevent unlimited goroutine creation if Read() blocks
	readSemaphore chan struct{}

	// Track containers that have already had initial logs fetched
	// This prevents duplicate logs when a container restarts
	initialFetchDone map[string]bool
	initialFetchMu   sync.RWMutex
}

// NewLogBroker creates a new LogBroker instance
func NewLogBroker(dockerClient *client.Client) *LogBroker {
	// CRITICAL: Limit concurrent read goroutines to prevent leak accumulation
	// Formula: 2x number of typical containers (allows burst during reconnects)
	// Max 200 concurrent reads = reasonable for systems with 100 containers
	const maxConcurrentReads = 200

	return &LogBroker{
		dockerClient:     dockerClient,
		consumers:        []LogConsumer{},
		activeStreams:    make(map[string]context.CancelFunc),
		containers:       []types.Container{},
		readSemaphore:    make(chan struct{}, maxConcurrentReads),
		initialFetchDone: make(map[string]bool),
	}
}

// RegisterConsumer adds a new log consumer
func (lb *LogBroker) RegisterConsumer(consumer LogConsumer) {
	lb.consumersMu.Lock()
	defer lb.consumersMu.Unlock()
	lb.consumers = append(lb.consumers, consumer)
}

// UnregisterConsumer removes a consumer
func (lb *LogBroker) UnregisterConsumer(consumer LogConsumer) {
	lb.consumersMu.Lock()
	defer lb.consumersMu.Unlock()

	filtered := []LogConsumer{}
	for _, c := range lb.consumers {
		if c != consumer {
			filtered = append(filtered, c)
		}
	}
	lb.consumers = filtered
}

// StartStreaming starts streaming for all running containers
func (lb *LogBroker) StartStreaming(containers []types.Container) {
	lb.containersMu.Lock()
	lb.containers = containers
	lb.containersMu.Unlock()

	for _, container := range containers {
		if container.State != "running" {
			continue
		}

		// CRITICAL FIX: Protect against empty Names slice
		if len(container.Names) == 0 {
			continue
		}

		// CRITICAL FIX: Check and insert atomically to prevent TOCTOU race
		// Lock once, check existence, create context, insert, then unlock
		lb.streamsMu.Lock()
		if _, exists := lb.activeStreams[container.ID]; exists {
			// Stream already active, skip
			lb.streamsMu.Unlock()
			continue
		}

		// Create context and insert into map while holding lock
		ctx, cancel := context.WithCancel(context.Background())
		lb.activeStreams[container.ID] = cancel
		lb.streamsMu.Unlock()

		// Launch goroutine AFTER releasing lock with crash protection
		containerID := container.ID
		containerName := container.Names[0]
		safeGo(fmt.Sprintf("streamContainer-%s", containerName), func() {
			lb.streamContainer(ctx, containerID, containerName)
		})
	}

	// Stop streams for containers that are no longer running
	lb.streamsMu.Lock()
	for containerID, cancel := range lb.activeStreams {
		found := false
		for _, c := range containers {
			if c.ID == containerID && c.State == "running" {
				found = true
				break
			}
		}
		if !found {
			// Container is no longer running, stop the stream
			cancel()
			delete(lb.activeStreams, containerID)
		}
	}
	lb.streamsMu.Unlock()
}

// streamContainer streaming goroutine for a container
func (lb *LogBroker) streamContainer(ctx context.Context, containerID, containerName string) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Panic in LogBroker.streamContainer for %s: %v\n", containerID, r)
		}

		// CRITICAL GOROUTINE LEAK FIX: Only delete from activeStreams in defer
		// This ensures the entry persists during reconnection loops
		// Without this, StartStreaming() can create duplicate goroutines during reconnection
		lb.streamsMu.Lock()
		delete(lb.activeStreams, containerID)
		lb.streamsMu.Unlock()

		// Notify consumers
		lb.notifyConsumers(func(c LogConsumer) {
			c.OnContainerStatusChange(containerID, false)
		})
	}()

	// Clean container name (remove leading slash)
	containerName = strings.TrimPrefix(containerName, "/")

	firstIteration := true
	checkTicker := time.NewTicker(5 * time.Second)
	defer checkTicker.Stop()

	for {
		// CRITICAL GOROUTINE LEAK FIX: Only use 'default:' on first iteration
		// The original 'default:' case allowed the select to NEVER block, creating
		// a tight loop that could spawn thousands of goroutines per second on errors
		//
		// Example of the bug: If ContainerLogs() fails fast (network error):
		// - Loop iterates immediately without waiting (default case)
		// - Each iteration creates a new goroutine with 30s timeout (line 232)
		// - 32 containers × 100 retries/sec = 3,200 goroutines/sec
		// - After 45 seconds: 144,000 goroutines → deadlock crash
		//
		// Fix: Only allow immediate pass-through on first iteration (for fast startup)
		// After that, block until checkTicker fires (every 5 seconds)
		if firstIteration {
			firstIteration = false
			// First iteration: don't block, proceed immediately to start streaming
		} else {
			// Subsequent iterations: block until ticker or context cancellation
			select {
			case <-ctx.Done():
				return
			case <-checkTicker.C:
				// Check if container exists and is running every 5 seconds
				inspectCtx, inspectCancel := context.WithTimeout(context.Background(), 2*time.Second)
				inspect, err := lb.dockerClient.ContainerInspect(inspectCtx, containerID)
				inspectCancel()
				if err != nil || !inspect.State.Running {
					return
				}
				// Container still running, continue to streaming code below
			}
		}

		// Stream logs with context timeout to prevent connection leaks
		// Check if initial logs have already been fetched for this container
		// This prevents duplicate logs when container restarts
		lb.initialFetchMu.RLock()
		alreadyFetched := lb.initialFetchDone[containerID]
		lb.initialFetchMu.RUnlock()

		tailLines := "50"
		if alreadyFetched {
			tailLines = "0"
		}

		// CRITICAL FIX: Add timeout to prevent hang if Docker daemon freezes
		// Use 10s timeout for initial connection, then use parent context for streaming
		logsCtx, logsCancel := context.WithTimeout(ctx, 10*time.Second)

		reader, err := lb.dockerClient.ContainerLogs(logsCtx, containerID, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Tail:       tailLines,
		})

		if err != nil {
			logsCancel() // CRITICAL FIX: Must cancel on error path
			time.Sleep(time.Second)
			continue
		}

		// Mark container as having had initial logs fetched
		// This persists across goroutine restarts to prevent duplicate logs
		if !alreadyFetched {
			lb.initialFetchMu.Lock()
			lb.initialFetchDone[containerID] = true
			lb.initialFetchMu.Unlock()
		}

		// Parse et distribuer
		var closeOnce sync.Once
		closeReader := func() {
			if reader != nil {
				reader.Close()
			}
			logsCancel() // CRITICAL FIX: Cancel context to prevent leak (success path)
		}

		// CRITICAL FIX: Use dynamic buffer size to handle large log lines
		// Start with 8KB, grow up to 1MB if needed
		const (
			minBufSize = 8192
			maxBufSize = 1024 * 1024 // 1MB max
		)
		buf := make([]byte, minBufSize)
		streamBroken := false
		consecutiveTimeouts := 0
		maxConsecutiveTimeouts := 3 // Force reconnect after 3 consecutive read timeouts
		incompleteData := []byte{}   // Hold incomplete frames across reads

		for {
			select {
			case <-ctx.Done():
				closeOnce.Do(closeReader)
				return
			default:
			}

			// CRITICAL FIX: Force reconnection if too many consecutive timeouts
			// This prevents accumulation of zombie goroutines
			if consecutiveTimeouts >= maxConsecutiveTimeouts {
				closeOnce.Do(closeReader)
				streamBroken = true
				break
			}

			// CRITICAL FIX: Use short timeout and aggressive cleanup to prevent goroutine leaks
			// The previous 30s timeout was too long and allowed goroutines to accumulate
			// New strategy: 5s timeout + immediate reader close on timeout
			type readResult struct {
				n   int
				err error
			}
			readChan := make(chan readResult, 1)
			readCtx, readCancel := context.WithTimeout(ctx, 5*time.Second)

			// CRITICAL FIX: Set read deadline BEFORE launching goroutine
			// This ensures the deadline is set even if the goroutine starts slowly
			if deadline, ok := readCtx.Deadline(); ok {
				type deadlineSetter interface {
					SetReadDeadline(time.Time) error
				}
				if ds, ok := reader.(deadlineSetter); ok {
					ds.SetReadDeadline(deadline)
				}
			}

			// CRITICAL GOROUTINE LEAK PREVENTION: Acquire semaphore before launching read goroutine
			// If semaphore is full (200 concurrent reads), abort this read attempt
			// This prevents unlimited goroutine accumulation when Read() blocks indefinitely
			select {
			case lb.readSemaphore <- struct{}{}:
				// Semaphore acquired, launch read goroutine
			case <-readCtx.Done():
				// Timeout waiting for semaphore - skip this read
				readCancel()
				consecutiveTimeouts++
				closeOnce.Do(closeReader)
				streamBroken = true
				continue // CRITICAL FIX: Skip goroutine launch without semaphore
			}

			// Launch read in goroutine with guaranteed cleanup
			go func() {
				defer func() {
					<-lb.readSemaphore // CRITICAL: Always release semaphore
					readCancel()
					if r := recover(); r != nil {
						select {
						case readChan <- readResult{0, fmt.Errorf("read panic: %v", r)}:
						default:
						}
					}
				}()

				n, err := reader.Read(buf)
				select {
				case readChan <- readResult{n, err}:
				case <-readCtx.Done():
					// Timeout: try one last send then exit
					select {
					case readChan <- readResult{n, err}:
					default:
					}
				}
			}()

			// Wait for read with timeout
			var n int
			var err error
			select {
			case <-ctx.Done():
				closeOnce.Do(closeReader)
				return
			case result := <-readChan:
				n = result.n
				err = result.err
			case <-readCtx.Done():
				// CRITICAL FIX: On timeout, immediately close reader to force Read() to abort
				// This prevents the goroutine from blocking forever
				consecutiveTimeouts++
				closeOnce.Do(closeReader)
				streamBroken = true
				break
			}

			if err != nil {
				closeOnce.Do(closeReader)
				streamBroken = true
				break
			}

			// Reset timeout counter on successful read
			consecutiveTimeouts = 0

			// CRITICAL FIX: Combine incomplete data from previous read with new data
			data := append(incompleteData, buf[:n]...)
			incompleteData = []byte{} // Clear incomplete buffer
			offset := 0

			// Parse multiplexed stream frames
			for offset < len(data) {
				// Need at least 8 bytes for header
				if offset+8 > len(data) {
					// Incomplete header - save for next read
					incompleteData = data[offset:]
					break
				}

				// Parse size (4 bytes in big-endian, unsigned)
				size := int(data[offset+4])<<24 | int(data[offset+5])<<16 | int(data[offset+6])<<8 | int(data[offset+7])

				// CRITICAL FIX: Validate size to prevent panic from negative or overflow values
				if size < 0 || size > maxBufSize {
					// Corrupted stream or size too large, abort this chunk
					break
				}

				// Check if we have complete frame
				frameEnd := offset + 8 + size
				if frameEnd > len(data) {
					// Incomplete frame - save for next read
					incompleteData = data[offset:]

					// CRITICAL FIX: If incomplete frame would exceed buffer size, grow buffer
					if len(incompleteData)+minBufSize > len(buf) && len(buf) < maxBufSize {
						newSize := min(len(buf)*2, maxBufSize)
						buf = make([]byte, newSize)
					}
					break
				}

				// Complete frame available
				payload := data[offset+8 : frameEnd]
				line := strings.TrimRight(string(payload), "\n")
				timestamp := time.Now()

				// Distribute to all consumers
				lb.notifyConsumers(func(c LogConsumer) {
					c.OnLogLine(containerID, containerName, line, timestamp)
				})

				offset = frameEnd
			}
		}

		// CRITICAL FIX: Always cleanup reader and context before loop continues
		closeOnce.Do(closeReader)

		// Stream ended or error occurred, wait before reconnecting
		if streamBroken {
			time.Sleep(time.Second)
		}
	}
}

// notifyConsumers applies a function to all consumers
func (lb *LogBroker) notifyConsumers(fn func(LogConsumer)) {
	lb.consumersMu.RLock()
	defer lb.consumersMu.RUnlock()

	for _, consumer := range lb.consumers {
		fn(consumer)
	}
}

// GetActiveStreamCount returns the number of active log streams
func (lb *LogBroker) GetActiveStreamCount() int {
	lb.streamsMu.RLock()
	defer lb.streamsMu.RUnlock()
	return len(lb.activeStreams)
}

// GetConsumerCount returns the number of registered consumers
func (lb *LogBroker) GetConsumerCount() int {
	lb.consumersMu.RLock()
	defer lb.consumersMu.RUnlock()
	return len(lb.consumers)
}

// StopAll stops all streams
func (lb *LogBroker) StopAll() {
	// Copy cancel functions to avoid holding lock during cancellation
	lb.streamsMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(lb.activeStreams))
	for _, cancel := range lb.activeStreams {
		cancels = append(cancels, cancel)
	}
	lb.streamsMu.Unlock()

	// CRITICAL FIX: Protect each cancel() with recover to prevent panic from stopping cleanup
	// Cancel all streams without holding lock (prevents deadlock)
	for _, cancel := range cancels {
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Log panic but continue cleanup
					fmt.Fprintf(os.Stderr, "Panic during context cancellation in StopAll: %v\n", r)
				}
			}()
			cancel()
		}()
	}

	// Clear the map after all cancellations
	lb.streamsMu.Lock()
	lb.activeStreams = make(map[string]context.CancelFunc)
	lb.streamsMu.Unlock()

	// Clear initial fetch tracking (allows re-fetching on restart)
	lb.initialFetchMu.Lock()
	lb.initialFetchDone = make(map[string]bool)
	lb.initialFetchMu.Unlock()
}

// FetchRecentLogs fetches recent log lines for specific containers (oneshot, no streaming)
func (lb *LogBroker) FetchRecentLogs(containerIDs []string, tailLines string) map[string][]string {
	result := make(map[string][]string)

	for _, containerID := range containerIDs {
		// Find container name
		lb.containersMu.RLock()
		var containerName string
		for _, c := range lb.containers {
			if c.ID == containerID {
				// CRITICAL FIX: Protect against empty Names slice
				if len(c.Names) > 0 {
					containerName = strings.TrimPrefix(c.Names[0], "/")
				} else {
					containerName = containerID[:12]
				}
				break
			}
		}
		lb.containersMu.RUnlock()

		if containerName == "" {
			continue
		}

		// Fetch logs (oneshot) - use closure to properly defer cancel
		lines := func() []string {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			reader, err := lb.dockerClient.ContainerLogs(ctx, containerID, container.LogsOptions{
				ShowStdout: true,
				ShowStderr: true,
				Follow:     false, // Oneshot, pas de streaming
				Tail:       tailLines,
			})

			if err != nil {
				return []string{}
			}
			defer reader.Close()

			// Parse logs with dynamic buffer
			const (
				minBufSize = 8192
				maxBufSize = 1024 * 1024
			)
			lines := []string{}
			buf := make([]byte, minBufSize)
			incompleteData := []byte{}

			for {
				n, err := reader.Read(buf)
				if err != nil {
					break
				}

				// Combine incomplete data from previous read
				data := append(incompleteData, buf[:n]...)
				incompleteData = []byte{}
				offset := 0

				// Parse multiplexed stream frames
				for offset < len(data) {
					// Need at least 8 bytes for header
					if offset+8 > len(data) {
						incompleteData = data[offset:]
						break
					}

					// Parse size (4 bytes in big-endian, unsigned)
					size := int(data[offset+4])<<24 | int(data[offset+5])<<16 | int(data[offset+6])<<8 | int(data[offset+7])

					// Validate size
					if size < 0 || size > maxBufSize {
						break
					}

					// Check if we have complete frame
					frameEnd := offset + 8 + size
					if frameEnd > len(data) {
						// Incomplete frame - save for next read
						incompleteData = data[offset:]
						// Grow buffer if needed
						if len(incompleteData)+minBufSize > len(buf) && len(buf) < maxBufSize {
							newSize := min(len(buf)*2, maxBufSize)
							buf = make([]byte, newSize)
						}
						break
					}

					// Complete frame available
					payload := data[offset+8 : frameEnd]
					line := strings.TrimRight(string(payload), "\n")
					lines = append(lines, line)

					offset = frameEnd
				}
			}

			return lines
		}()

		result[containerID] = lines
	}

	return result
}
