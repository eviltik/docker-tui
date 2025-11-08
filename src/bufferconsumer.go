package main

import (
	"sync"
	"sync/atomic"
	"time"
)

// LogEntry represents a log entry
type LogEntry struct {
	ContainerID   string
	ContainerName string
	Line          string
	Timestamp     time.Time
	IsSeparator   bool // True if this is a user-inserted separator line
}

// BufferConsumer implements LogConsumer to buffer logs (logsView)
type BufferConsumer struct {
	containerIDs map[string]bool // Containers to track
	buffer       []LogEntry      // Pre-allocated circular buffer
	head         int             // Head index (write position)
	size         int             // Current number of entries
	bufferMu     sync.RWMutex
	maxLines     int             // Maximum number of lines
	callback     func(LogEntry)  // Callback called for each new line
	closing      *atomic.Bool    // Pointer to model's closing flag
	wg           *sync.WaitGroup // WaitGroup to track active callbacks
	callbackMu   sync.Mutex      // CRITICAL FIX: Protects callback execution and WaitGroup operations
}

// NewBufferConsumer creates a new instance
func NewBufferConsumer(containerIDs []string, maxLines int, callback func(LogEntry), closing *atomic.Bool, wg *sync.WaitGroup) *BufferConsumer {
	idMap := make(map[string]bool)
	for _, id := range containerIDs {
		idMap[id] = true
	}

	return &BufferConsumer{
		containerIDs: idMap,
		buffer:       make([]LogEntry, maxLines), // Pre-allocate circular buffer
		head:         0,
		size:         0,
		maxLines:     maxLines,
		callback:     callback,
		closing:      closing,
		wg:           wg,
	}
}

// OnLogLine is called when a new log line arrives
func (bc *BufferConsumer) OnLogLine(containerID, containerName, line string, timestamp time.Time) {
	// Check if we track this container
	if !bc.containerIDs[containerID] {
		return
	}

	bc.bufferMu.Lock()
	defer bc.bufferMu.Unlock()

	entry := LogEntry{
		ContainerID:   containerID,
		ContainerName: containerName,
		Line:          line,
		Timestamp:     timestamp,
	}

	// Write to circular buffer (no reallocation)
	bc.buffer[bc.head] = entry
	bc.head = (bc.head + 1) % bc.maxLines

	// Increment size up to max
	if bc.size < bc.maxLines {
		bc.size++
	}

	// CRITICAL FIX: Atomically check closing flag and increment WaitGroup
	// This prevents race where closing happens between check and Add()
	bc.callbackMu.Lock()
	if bc.callback != nil && bc.closing != nil && !bc.closing.Load() {
		// Increment WaitGroup BEFORE releasing lock
		if bc.wg != nil {
			bc.wg.Add(1)
		}
		bc.callbackMu.Unlock()

		// Decrement WaitGroup after callback completes
		if bc.wg != nil {
			defer bc.wg.Done()
		}

		// Protect against panic from closed channel
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Channel was closed, silently ignore (normal during shutdown)
				}
			}()
			bc.callback(entry)
		}()
	} else {
		bc.callbackMu.Unlock()
	}
}

// OnContainerStatusChange is called when a container changes state
func (bc *BufferConsumer) OnContainerStatusChange(containerID string, isRunning bool) {
	// Optional: add an entry in the buffer to indicate state change
}

// GetBuffer returns a copy of the buffer in chronological order
func (bc *BufferConsumer) GetBuffer() []LogEntry {
	bc.bufferMu.RLock()
	defer bc.bufferMu.RUnlock()

	if bc.size == 0 {
		return []LogEntry{}
	}

	// Allocate exactly the necessary size
	result := make([]LogEntry, bc.size)

	if bc.size < bc.maxLines {
		// Buffer not full yet, copy from 0 to size
		copy(result, bc.buffer[:bc.size])
	} else {
		// Buffer full, rebuild in chronological order
		// Oldest entries start at head
		copy(result, bc.buffer[bc.head:])
		copy(result[bc.maxLines-bc.head:], bc.buffer[:bc.head])
	}

	return result
}

// Clear empties the buffer (circular buffer reset)
func (bc *BufferConsumer) Clear() {
	bc.bufferMu.Lock()
	defer bc.bufferMu.Unlock()
	bc.head = 0
	bc.size = 0
	// No need to reallocate, just reset pointers
}

// InsertSeparator inserts a blank separator line in the log buffer
func (bc *BufferConsumer) InsertSeparator() {
	bc.bufferMu.Lock()
	defer bc.bufferMu.Unlock()

	entry := LogEntry{
		ContainerID:   "",
		ContainerName: "",
		Line:          "", // Empty line (just spaces when rendered)
		Timestamp:     time.Now(),
		IsSeparator:   true,
	}

	// Write to circular buffer
	bc.buffer[bc.head] = entry
	bc.head = (bc.head + 1) % bc.maxLines

	// Increment size up to max
	if bc.size < bc.maxLines {
		bc.size++
	}

	// Trigger callback to refresh UI
	bc.callbackMu.Lock()
	if bc.callback != nil && bc.closing != nil && !bc.closing.Load() {
		bc.wg.Add(1)
		bc.callbackMu.Unlock()

		go func() {
			defer func() {
				bc.wg.Done()
				if r := recover(); r != nil {
					// Channel was closed, silently ignore
				}
			}()
			bc.callback(entry)
		}()
	} else {
		bc.callbackMu.Unlock()
	}
}

// PreloadLogs pre-fills the buffer with existing logs
// IMPORTANT: containerIDs parameter ensures logs are loaded in stable order
func (bc *BufferConsumer) PreloadLogs(containerIDs []string, logsByContainer map[string][]string, containerNames map[string]string) {
	bc.bufferMu.Lock()
	defer bc.bufferMu.Unlock()

	// Add logs from each container to the buffer using circular buffer
	// CRITICAL FIX: Iterate over containerIDs slice (stable order) instead of map (random order)
	// This ensures logs appear in the same order each time logs view is entered
	for _, containerID := range containerIDs {
		lines, exists := logsByContainer[containerID]
		if !exists {
			continue
		}
		containerName := containerNames[containerID]
		for _, line := range lines {
			entry := LogEntry{
				ContainerID:   containerID,
				ContainerName: containerName,
				Line:          line,
				Timestamp:     time.Now(),
			}

			// Use same pattern as OnLogLine for circular buffer
			bc.buffer[bc.head] = entry
			bc.head = (bc.head + 1) % bc.maxLines
			if bc.size < bc.maxLines {
				bc.size++
			}
		}
	}
}
