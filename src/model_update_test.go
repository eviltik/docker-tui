package main

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

// Test Update() message handlers

func TestUpdate_ContainerListMsg_CursorAdjustment(t *testing.T) {
	m := &model{
		cursor:       5,
		containers:   []types.Container{{ID: "c1"}, {ID: "c2"}},
		containersMu: sync.RWMutex{},
		cpuStats:     make(map[string][]float64),
		cpuCurrent:   make(map[string]float64),
		cpuPrevStats: make(map[string]*container.StatsResponse),
		cpuStatsMu:   sync.RWMutex{},
		processing:   make(map[string]bool),
		processingMu: sync.RWMutex{},
	}

	// Simulate container list update with fewer containers
	msg := containerListMsg([]types.Container{{ID: "c1"}})
	newModel, _ := m.Update(msg)
	m = newModel.(*model)

	// Cursor should be adjusted to last valid index
	if m.cursor != 0 {
		t.Errorf("Expected cursor=0 after list shrink, got %d", m.cursor)
	}
}

func TestUpdate_ContainerListMsg_EmptyList(t *testing.T) {
	m := &model{
		cursor:       0,
		containers:   []types.Container{{ID: "c1"}},
		containersMu: sync.RWMutex{},
		cpuStats:     make(map[string][]float64),
		cpuCurrent:   make(map[string]float64),
		cpuPrevStats: make(map[string]*container.StatsResponse),
		cpuStatsMu:   sync.RWMutex{},
		processing:   make(map[string]bool),
		processingMu: sync.RWMutex{},
	}

	// Empty container list
	msg := containerListMsg([]types.Container{})
	newModel, _ := m.Update(msg)
	m = newModel.(*model)

	if m.cursor != 0 {
		t.Errorf("Expected cursor=0 for empty list, got %d", m.cursor)
	}
	if len(m.containers) != 0 {
		t.Errorf("Expected 0 containers, got %d", len(m.containers))
	}
}

func TestUpdate_ContainerListMsg_CleanupMaps(t *testing.T) {
	m := &model{
		containers:   []types.Container{{ID: "c1"}},
		containersMu: sync.RWMutex{},
		cpuStats: map[string][]float64{
			"c1":      {10.0},
			"removed": {20.0}, // This should be cleaned up
		},
		cpuCurrent: map[string]float64{
			"c1":      10.0,
			"removed": 20.0,
		},
		cpuPrevStats: map[string]*container.StatsResponse{
			"c1":      {},
			"removed": {},
		},
		cpuStatsMu: sync.RWMutex{},
		processing: map[string]bool{
			"c1":      true,
			"removed": true,
		},
		processingMu: sync.RWMutex{},
	}

	// Update with only c1
	msg := containerListMsg([]types.Container{{ID: "c1"}})
	m.Update(msg)

	// Check cleanup
	if _, exists := m.cpuStats["removed"]; exists {
		t.Error("Expected 'removed' to be cleaned from cpuStats")
	}
	if _, exists := m.cpuCurrent["removed"]; exists {
		t.Error("Expected 'removed' to be cleaned from cpuCurrent")
	}
	if _, exists := m.cpuPrevStats["removed"]; exists {
		t.Error("Expected 'removed' to be cleaned from cpuPrevStats")
	}
	if _, exists := m.processing["removed"]; exists {
		t.Error("Expected 'removed' to be cleaned from processing")
	}
}

func TestUpdate_CPUStatsMsg(t *testing.T) {
	m := &model{
		cpuStats:     make(map[string][]float64),
		cpuCurrent:   make(map[string]float64),
		cpuPrevStats: make(map[string]*container.StatsResponse),
		cpuStatsMu:   sync.RWMutex{},
	}

	msg := cpuStatsMsg{
		stats: map[string]float64{
			"c1": 25.5,
			"c2": 50.0,
		},
		rawStats: map[string]*container.StatsResponse{
			"c1": {},
			"c2": {},
		},
	}

	newModel, _ := m.Update(msg)
	m = newModel.(*model)

	if m.cpuCurrent["c1"] != 25.5 {
		t.Errorf("Expected cpuCurrent[c1]=25.5, got %f", m.cpuCurrent["c1"])
	}
	if len(m.cpuStats["c1"]) != 1 {
		t.Errorf("Expected 1 history entry, got %d", len(m.cpuStats["c1"]))
	}
}

func TestUpdate_CPUStatsMsg_HistoryLimit(t *testing.T) {
	m := &model{
		cpuStats: map[string][]float64{
			"c1": {1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, // Already 10 values
		},
		cpuCurrent:   make(map[string]float64),
		cpuPrevStats: make(map[string]*container.StatsResponse),
		cpuStatsMu:   sync.RWMutex{},
	}

	msg := cpuStatsMsg{
		stats: map[string]float64{
			"c1": 11.0,
		},
		rawStats: map[string]*container.StatsResponse{
			"c1": {},
		},
	}

	newModel, _ := m.Update(msg)
	m = newModel.(*model)

	// Should keep only last 10 values
	if len(m.cpuStats["c1"]) != 10 {
		t.Errorf("Expected 10 history entries (limit), got %d", len(m.cpuStats["c1"]))
	}
	// First value should be 2 (1 was dropped)
	if m.cpuStats["c1"][0] != 2 {
		t.Errorf("Expected first value=2, got %f", m.cpuStats["c1"][0])
	}
	// Last value should be 11
	if m.cpuStats["c1"][9] != 11.0 {
		t.Errorf("Expected last value=11, got %f", m.cpuStats["c1"][9])
	}
}

func TestUpdate_TickMsg(t *testing.T) {
	m := &model{
		spinnerFrame: 0,
	}

	msg := tickMsg(time.Now())
	newModel, cmd := m.Update(msg)
	m = newModel.(*model)

	if m.spinnerFrame != 1 {
		t.Errorf("Expected spinnerFrame=1, got %d", m.spinnerFrame)
	}
	if cmd == nil {
		t.Error("Expected batch command from tick")
	}
}

func TestUpdate_LogRateTickMsg(t *testing.T) {
	m := &model{}

	msg := logRateTickMsg(time.Now())
	newModel, cmd := m.Update(msg)
	m = newModel.(*model)

	if cmd == nil {
		t.Error("Expected command from logRateTickMsg")
	}
}

func TestUpdate_CleanupTickMsg(t *testing.T) {
	m := &model{
		rateTracker: NewRateTrackerConsumer(),
	}

	msg := cleanupTickMsg(time.Now())
	newModel, cmd := m.Update(msg)
	m = newModel.(*model)

	if cmd == nil {
		t.Error("Expected command from cleanupTickMsg")
	}
}

func TestUpdate_CPUCleanupTickMsg(t *testing.T) {
	m := &model{
		containers:   []types.Container{{ID: "c1"}},
		containersMu: sync.RWMutex{},
		cpuStats: map[string][]float64{
			"c1":    {10.0},
			"stale": {20.0},
		},
		cpuCurrent: map[string]float64{
			"c1":    10.0,
			"stale": 20.0,
		},
		cpuPrevStats: map[string]*container.StatsResponse{
			"c1":    {},
			"stale": {},
		},
		cpuStatsMu: sync.RWMutex{},
	}

	msg := cpuCleanupTickMsg(time.Now())
	newModel, _ := m.Update(msg)
	m = newModel.(*model)

	// Stale entries should be removed
	if _, exists := m.cpuStats["stale"]; exists {
		t.Error("Expected stale entry removed from cpuStats")
	}
	if _, exists := m.cpuCurrent["stale"]; exists {
		t.Error("Expected stale entry removed from cpuCurrent")
	}
	if _, exists := m.cpuPrevStats["stale"]; exists {
		t.Error("Expected stale entry removed from cpuPrevStats")
	}

	// Active container should remain
	if _, exists := m.cpuStats["c1"]; !exists {
		t.Error("Expected c1 to remain in cpuStats")
	}
}

func TestUpdate_ActionStartMsg(t *testing.T) {
	m := &model{
		processing:   make(map[string]bool),
		processingMu: sync.RWMutex{},
		containers:   []types.Container{{ID: "c1"}},
		containersMu: sync.RWMutex{},
	}

	msg := actionStartMsg{
		action: "start",
		ids:    []string{"c1", "c2"},
	}

	newModel, cmd := m.Update(msg)
	m = newModel.(*model)

	// Should mark containers as processing
	if !m.processing["c1"] {
		t.Error("Expected c1 to be marked as processing")
	}
	if !m.processing["c2"] {
		t.Error("Expected c2 to be marked as processing")
	}
	if cmd == nil {
		t.Error("Expected command from actionStartMsg")
	}
}

func TestUpdate_ErrorMsg(t *testing.T) {
	m := &model{}

	testErr := errorMsg{err: errors.New("test error")}
	newModel, _ := m.Update(testErr)
	m = newModel.(*model)

	if m.err.(error).Error() != "test error" {
		t.Errorf("Expected err='test error', got '%v'", m.err)
	}
}

func TestUpdate_NewLogLineMsg_AtBottom(t *testing.T) {
	closing := atomic.Bool{}
	wg := sync.WaitGroup{}

	m := &model{
		view:           logsView,
		wasAtBottom:    true,
		logsViewScroll: 0,
		height:         20,
		bufferConsumer: NewBufferConsumer(
			[]string{"c1"},
			100,
			func(entry LogEntry) {},
			&closing,
			&wg,
		),
		newLogChan: make(chan struct{}, 100),
	}

	// Add some logs
	for i := 0; i < 50; i++ {
		m.bufferConsumer.OnLogLine("c1", "container1", "log line", time.Now())
	}

	msg := newLogLineMsg{}
	newModel, _ := m.Update(msg)
	m = newModel.(*model)

	// Should auto-scroll to bottom
	expected := max(0, 50-(20-5))
	if m.logsViewScroll != expected {
		t.Errorf("Expected scroll=%d, got %d", expected, m.logsViewScroll)
	}
}

func TestUpdate_NewLogLineMsg_NotAtBottom(t *testing.T) {
	closing := atomic.Bool{}
	wg := sync.WaitGroup{}

	m := &model{
		view:           logsView,
		wasAtBottom:    false,
		logsViewScroll: 5,
		height:         20,
		bufferConsumer: NewBufferConsumer(
			[]string{"c1"},
			100,
			func(entry LogEntry) {},
			&closing,
			&wg,
		),
		newLogChan: make(chan struct{}, 100),
	}

	msg := newLogLineMsg{}
	newModel, _ := m.Update(msg)
	m = newModel.(*model)

	// Should NOT auto-scroll (stay at 5)
	if m.logsViewScroll != 5 {
		t.Errorf("Expected scroll=5 (no change), got %d", m.logsViewScroll)
	}
}

func TestUpdate_NewLogLineMsg_NotInLogsView(t *testing.T) {
	m := &model{
		view: listView, // Not in logs view
	}

	msg := newLogLineMsg{}
	newModel, cmd := m.Update(msg)
	m = newModel.(*model)

	// Should not wait for next log
	if cmd != nil {
		t.Error("Expected no command when not in logs view")
	}
}
