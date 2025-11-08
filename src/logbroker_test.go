package main

import (
	"context"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

// mockDockerClient is a mock implementation for testing
type mockDockerClient struct {
	containers []types.Container
	logs       map[string]string // containerID -> log content
	mu         sync.Mutex
}

func (m *mockDockerClient) ContainerList(ctx context.Context, opts container.ListOptions) ([]types.Container, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.containers, nil
}

func (m *mockDockerClient) ContainerLogs(ctx context.Context, containerID string, opts container.LogsOptions) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	logContent := m.logs[containerID]
	if logContent == "" {
		logContent = "test log line\n"
	}

	// Return a multiplexed stream (Docker format)
	// Header: 1 byte stream type + 3 bytes padding + 4 bytes size
	size := len(logContent)
	header := []byte{1, 0, 0, 0, byte(size >> 24), byte(size >> 16), byte(size >> 8), byte(size)}
	data := append(header, []byte(logContent)...)

	return io.NopCloser(strings.NewReader(string(data))), nil
}

func (m *mockDockerClient) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			State: &types.ContainerState{
				Running: true,
			},
		},
	}, nil
}

// mockLogConsumer for testing log distribution
type mockLogConsumer struct {
	logs     []LogEntry
	mu       sync.Mutex
	callback func(LogEntry) // Optional callback for testing
}

func (m *mockLogConsumer) OnLogLine(containerID, containerName, line string, timestamp time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := LogEntry{
		ContainerID:   containerID,
		ContainerName: containerName,
		Line:          line,
		Timestamp:     timestamp,
	}
	m.logs = append(m.logs, entry)

	if m.callback != nil {
		m.callback(entry)
	}
}

func (m *mockLogConsumer) OnContainerStatusChange(containerID string, isRunning bool) {
	// No-op for basic tests
}

func (m *mockLogConsumer) GetLogs() []LogEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]LogEntry{}, m.logs...)
}

// TestLogBrokerRegistration tests consumer registration and unregistration
func TestLogBrokerRegistration(t *testing.T) {
	// Create a minimal LogBroker for testing
	broker := &LogBroker{
		dockerClient:  nil,
		consumers:     []LogConsumer{},
		activeStreams: make(map[string]context.CancelFunc),
		containers:    []types.Container{},
	}

	// Test consumer registration
	consumer1 := &mockLogConsumer{}
	consumer2 := &mockLogConsumer{}

	broker.RegisterConsumer(consumer1)
	if count := broker.GetConsumerCount(); count != 1 {
		t.Errorf("GetConsumerCount() = %d, want 1", count)
	}

	broker.RegisterConsumer(consumer2)
	if count := broker.GetConsumerCount(); count != 2 {
		t.Errorf("GetConsumerCount() = %d, want 2", count)
	}

	// Test consumer unregistration
	broker.UnregisterConsumer(consumer1)
	if count := broker.GetConsumerCount(); count != 1 {
		t.Errorf("GetConsumerCount() after unregister = %d, want 1", count)
	}

	broker.UnregisterConsumer(consumer2)
	if count := broker.GetConsumerCount(); count != 0 {
		t.Errorf("GetConsumerCount() after all unregister = %d, want 0", count)
	}
}

// TestLogBrokerConcurrentRegistration tests thread-safe consumer registration
func TestLogBrokerConcurrentRegistration(t *testing.T) {
	broker := &LogBroker{
		consumers:     []LogConsumer{},
		activeStreams: make(map[string]context.CancelFunc),
	}

	const numConsumers = 100
	var wg sync.WaitGroup

	// Concurrently register consumers
	for i := 0; i < numConsumers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			consumer := &mockLogConsumer{}
			broker.RegisterConsumer(consumer)
		}()
	}

	wg.Wait()

	if count := broker.GetConsumerCount(); count != numConsumers {
		t.Errorf("GetConsumerCount() after concurrent registration = %d, want %d", count, numConsumers)
	}
}

// TestLogBrokerNotifyConsumers tests log distribution to multiple consumers
func TestLogBrokerNotifyConsumers(t *testing.T) {
	broker := &LogBroker{
		consumers:     []LogConsumer{},
		activeStreams: make(map[string]context.CancelFunc),
	}

	consumer1 := &mockLogConsumer{}
	consumer2 := &mockLogConsumer{}
	consumer3 := &mockLogConsumer{}

	broker.RegisterConsumer(consumer1)
	broker.RegisterConsumer(consumer2)
	broker.RegisterConsumer(consumer3)

	// Simulate log notification
	testLog := LogEntry{
		ContainerID:   "test-container",
		ContainerName: "test-name",
		Line:          "test log line",
		Timestamp:     time.Now(),
	}

	broker.notifyConsumers(func(c LogConsumer) {
		c.OnLogLine(testLog.ContainerID, testLog.ContainerName, testLog.Line, testLog.Timestamp)
	})

	// Wait a bit for goroutines to process
	time.Sleep(50 * time.Millisecond)

	// Verify all consumers received the log
	for i, consumer := range []*mockLogConsumer{consumer1, consumer2, consumer3} {
		logs := consumer.GetLogs()
		if len(logs) != 1 {
			t.Errorf("Consumer %d: got %d logs, want 1", i, len(logs))
			continue
		}
		if logs[0].Line != testLog.Line {
			t.Errorf("Consumer %d: got line %q, want %q", i, logs[0].Line, testLog.Line)
		}
	}
}

// TestLogBrokerStopAll tests stopping all active streams
func TestLogBrokerStopAll(t *testing.T) {
	broker := &LogBroker{
		consumers:     []LogConsumer{},
		activeStreams: make(map[string]context.CancelFunc),
	}

	// Create some dummy cancel functions
	called := make([]bool, 3)
	for i := 0; i < 3; i++ {
		idx := i
		_, cancel := context.WithCancel(context.Background())

		// Wrap cancel to track if it was called
		wrappedCancel := func() {
			called[idx] = true
			cancel()
		}

		broker.activeStreams[string(rune('A'+i))] = wrappedCancel
	}

	if count := broker.GetActiveStreamCount(); count != 3 {
		t.Errorf("GetActiveStreamCount() before StopAll = %d, want 3", count)
	}

	broker.StopAll()

	// Verify all cancel functions were called
	for i, wasCalled := range called {
		if !wasCalled {
			t.Errorf("Cancel function %d was not called", i)
		}
	}

	// Verify streams were cleared
	if count := broker.GetActiveStreamCount(); count != 0 {
		t.Errorf("GetActiveStreamCount() after StopAll = %d, want 0", count)
	}
}

// TestBufferConsumerCircularBuffer tests circular buffer overflow behavior
func TestBufferConsumerCircularBuffer(t *testing.T) {
	const bufferSize = 10
	closing := &atomic.Bool{}
	var wg sync.WaitGroup

	consumer := NewBufferConsumer(
		[]string{"container-1"},
		bufferSize,
		func(entry LogEntry) {}, // No-op callback
		closing,
		&wg,
	)

	// Add more logs than buffer size
	for i := 0; i < bufferSize*2; i++ {
		consumer.OnLogLine(
			"container-1",
			"test-container",
			"log line "+string(rune('0'+i)),
			time.Now(),
		)
	}

	buffer := consumer.GetBuffer()

	// Buffer should not exceed max size
	if len(buffer) > bufferSize {
		t.Errorf("Buffer size = %d, want <= %d", len(buffer), bufferSize)
	}

	// Should contain only the last N entries
	if len(buffer) == bufferSize {
		// First entry should be from the second batch (log line 10)
		expectedStart := bufferSize
		if !strings.Contains(buffer[0].Line, string(rune('0'+expectedStart))) {
			t.Errorf("Oldest entry = %q, want to contain char %c", buffer[0].Line, rune('0'+expectedStart))
		}
	}
}

// TestBufferConsumerConcurrentWrites tests thread-safe log writing
func TestBufferConsumerConcurrentWrites(t *testing.T) {
	const bufferSize = 1000
	const numGoroutines = 10
	const logsPerGoroutine = 100

	closing := &atomic.Bool{}
	var wg sync.WaitGroup

	consumer := NewBufferConsumer(
		[]string{"container-1"},
		bufferSize,
		func(entry LogEntry) {},
		closing,
		&wg,
	)

	var writeWg sync.WaitGroup

	// Concurrently write logs
	for i := 0; i < numGoroutines; i++ {
		writeWg.Add(1)
		go func(goroutineID int) {
			defer writeWg.Done()
			for j := 0; j < logsPerGoroutine; j++ {
				consumer.OnLogLine(
					"container-1",
					"test-container",
					"log from goroutine "+string(rune('0'+goroutineID)),
					time.Now(),
				)
			}
		}(i)
	}

	writeWg.Wait()

	buffer := consumer.GetBuffer()

	// Should have received all logs (up to buffer limit)
	expectedCount := numGoroutines * logsPerGoroutine
	if expectedCount > bufferSize {
		expectedCount = bufferSize
	}

	if len(buffer) != expectedCount {
		t.Errorf("Buffer size after concurrent writes = %d, want %d", len(buffer), expectedCount)
	}
}

// TestRateTrackerBasic tests basic rate tracking functionality
func TestRateTrackerBasic(t *testing.T) {
	tracker := NewRateTrackerConsumer()

	containerID := "test-container"

	// Initially should be 0
	rate := tracker.GetRate(containerID)
	if rate != 0 {
		t.Errorf("Initial rate = %f, want 0", rate)
	}

	// Add some logs
	for i := 0; i < 10; i++ {
		tracker.OnLogLine(containerID, "test", "line", time.Now())
	}

	// Rate should be calculated (exact value depends on timing)
	rate = tracker.GetRate(containerID)
	if rate < 0 {
		t.Errorf("Rate after logs = %f, want >= 0", rate)
	}
}

