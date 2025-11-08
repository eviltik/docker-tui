package main

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/docker/docker/api/types"
)

// Test advanced mouse event handling

func TestHandleMouseEvent_LogsView_WheelUp(t *testing.T) {
	closing := atomic.Bool{}
	wg := sync.WaitGroup{}

	m := &model{
		view:           logsView,
		logsViewScroll: 10,
		height:         20,
		bufferConsumer: NewBufferConsumer(
			[]string{"c1"},
			100,
			func(entry LogEntry) {},
			&closing,
			&wg,
		),
	}

	msg := tea.MouseMsg{Type: tea.MouseWheelUp}
	newModel, _ := m.handleMouseEvent(msg)
	m = newModel.(*model)

	// Should scroll up by 3
	if m.logsViewScroll != 7 {
		t.Errorf("Expected scroll=7, got %d", m.logsViewScroll)
	}
}

func TestHandleMouseEvent_LogsView_WheelUp_BoundsCheck(t *testing.T) {
	closing := atomic.Bool{}
	wg := sync.WaitGroup{}

	m := &model{
		view:           logsView,
		logsViewScroll: 2, // Less than 3
		height:         20,
		bufferConsumer: NewBufferConsumer(
			[]string{"c1"},
			100,
			func(entry LogEntry) {},
			&closing,
			&wg,
		),
	}

	msg := tea.MouseMsg{Type: tea.MouseWheelUp}
	newModel, _ := m.handleMouseEvent(msg)
	m = newModel.(*model)

	// Should clamp to 0
	if m.logsViewScroll != 0 {
		t.Errorf("Expected scroll=0, got %d", m.logsViewScroll)
	}
}

func TestHandleMouseEvent_LogsView_WheelDown(t *testing.T) {
	closing := atomic.Bool{}
	wg := sync.WaitGroup{}

	bc := NewBufferConsumer(
		[]string{"c1"},
		100,
		func(entry LogEntry) {},
		&closing,
		&wg,
	)

	// Add 50 logs
	for i := 0; i < 50; i++ {
		bc.OnLogLine("c1", "container1", "log line", time.Now())
	}

	m := &model{
		view:           logsView,
		logsViewScroll: 0,
		height:         20,
		bufferConsumer: bc,
	}

	msg := tea.MouseMsg{Type: tea.MouseWheelDown}
	newModel, _ := m.handleMouseEvent(msg)
	m = newModel.(*model)

	// Should scroll down by 3
	if m.logsViewScroll != 3 {
		t.Errorf("Expected scroll=3, got %d", m.logsViewScroll)
	}
}

func TestHandleMouseEvent_LogsView_WheelDown_MaxScroll(t *testing.T) {
	closing := atomic.Bool{}
	wg := sync.WaitGroup{}

	bc := NewBufferConsumer(
		[]string{"c1"},
		100,
		func(entry LogEntry) {},
		&closing,
		&wg,
	)

	// Add 20 logs
	for i := 0; i < 20; i++ {
		bc.OnLogLine("c1", "container1", "log line", time.Now())
	}

	m := &model{
		view:           logsView,
		logsViewScroll: 100, // Already at max
		height:         20,
		bufferConsumer: bc,
	}

	maxScroll := max(0, 20-(20-5))
	m.logsViewScroll = maxScroll

	msg := tea.MouseMsg{Type: tea.MouseWheelDown}
	newModel, _ := m.handleMouseEvent(msg)
	m = newModel.(*model)

	// Should clamp to maxScroll
	if m.logsViewScroll > maxScroll {
		t.Errorf("Expected scroll<=%d, got %d", maxScroll, m.logsViewScroll)
	}
}

func TestHandleMouseEvent_ListView_SingleClick(t *testing.T) {
	m := &model{
		view:   listView,
		cursor: 0,
		containers: []types.Container{
			{ID: "c1", Names: []string{"/container1"}},
			{ID: "c2", Names: []string{"/container2"}},
			{ID: "c3", Names: []string{"/container3"}},
		},
		containersMu:    sync.RWMutex{},
		selected:        make(map[string]bool),
		selectedMu:      sync.RWMutex{},
		lastClickTime:   time.Time{},
		lastClickIndex:  -1,
		shiftStart:      -1,
		viewTransitionMu: sync.Mutex{},
	}

	// Click on line 7 (header offset 5 + index 2)
	msg := tea.MouseMsg{
		Type: tea.MouseLeft,
		Y:    7,
	}

	newModel, _ := m.handleMouseEvent(msg)
	m = newModel.(*model)

	// Should move cursor to index 2
	if m.cursor != 2 {
		t.Errorf("Expected cursor=2, got %d", m.cursor)
	}

	// Should toggle selection of c3
	if !m.selected["c3"] {
		t.Error("Expected c3 to be selected")
	}

	// Should reset shiftStart
	if m.shiftStart != -1 {
		t.Errorf("Expected shiftStart=-1, got %d", m.shiftStart)
	}
}

func TestHandleMouseEvent_ListView_SingleClick_Toggle(t *testing.T) {
	m := &model{
		view:   listView,
		cursor: 0,
		containers: []types.Container{
			{ID: "c1", Names: []string{"/container1"}},
		},
		containersMu: sync.RWMutex{},
		selected: map[string]bool{
			"c1": true, // Already selected
		},
		selectedMu:       sync.RWMutex{},
		lastClickTime:    time.Time{},
		lastClickIndex:   -1,
		viewTransitionMu: sync.Mutex{},
	}

	msg := tea.MouseMsg{
		Type: tea.MouseLeft,
		Y:    5, // First container (offset 5)
	}

	newModel, _ := m.handleMouseEvent(msg)
	m = newModel.(*model)

	// Should deselect c1
	if m.selected["c1"] {
		t.Error("Expected c1 to be deselected")
	}
}

func TestHandleMouseEvent_ListView_OutOfBounds(t *testing.T) {
	m := &model{
		view:         listView,
		cursor:       0,
		containers:   []types.Container{{ID: "c1"}},
		containersMu: sync.RWMutex{},
		selected:     make(map[string]bool),
		selectedMu:   sync.RWMutex{},
	}

	// Click beyond container list
	msg := tea.MouseMsg{
		Type: tea.MouseLeft,
		Y:    100,
	}

	newModel, _ := m.handleMouseEvent(msg)
	m = newModel.(*model)

	// Should not crash, cursor stays at 0
	if m.cursor != 0 {
		t.Errorf("Expected cursor=0, got %d", m.cursor)
	}
}

func TestHandleMouseEvent_ListView_BeforeHeaderOffset(t *testing.T) {
	m := &model{
		view:         listView,
		cursor:       0,
		containers:   []types.Container{{ID: "c1"}},
		containersMu: sync.RWMutex{},
		selected:     make(map[string]bool),
		selectedMu:   sync.RWMutex{},
	}

	// Click on title area
	msg := tea.MouseMsg{
		Type: tea.MouseLeft,
		Y:    2,
	}

	newModel, _ := m.handleMouseEvent(msg)
	m = newModel.(*model)

	// Should do nothing
	if m.cursor != 0 {
		t.Errorf("Expected cursor=0, got %d", m.cursor)
	}
}

func TestHandleMouseEvent_ListView_WheelUp(t *testing.T) {
	m := &model{
		view:         listView,
		cursor:       5,
		shiftStart:   2,
		containers:   []types.Container{{ID: "c1"}, {ID: "c2"}},
		containersMu: sync.RWMutex{},
	}

	msg := tea.MouseMsg{Type: tea.MouseWheelUp}
	newModel, _ := m.handleMouseEvent(msg)
	m = newModel.(*model)

	if m.cursor != 4 {
		t.Errorf("Expected cursor=4, got %d", m.cursor)
	}
	if m.shiftStart != -1 {
		t.Errorf("Expected shiftStart=-1, got %d", m.shiftStart)
	}
}

func TestHandleMouseEvent_ListView_WheelUp_AtTop(t *testing.T) {
	m := &model{
		view:         listView,
		cursor:       0,
		containers:   []types.Container{{ID: "c1"}},
		containersMu: sync.RWMutex{},
	}

	msg := tea.MouseMsg{Type: tea.MouseWheelUp}
	newModel, _ := m.handleMouseEvent(msg)
	m = newModel.(*model)

	// Should stay at 0
	if m.cursor != 0 {
		t.Errorf("Expected cursor=0, got %d", m.cursor)
	}
}

func TestHandleMouseEvent_ListView_WheelDown(t *testing.T) {
	m := &model{
		view:         listView,
		cursor:       0,
		shiftStart:   5,
		containers:   []types.Container{{ID: "c1"}, {ID: "c2"}, {ID: "c3"}},
		containersMu: sync.RWMutex{},
	}

	msg := tea.MouseMsg{Type: tea.MouseWheelDown}
	newModel, _ := m.handleMouseEvent(msg)
	m = newModel.(*model)

	if m.cursor != 1 {
		t.Errorf("Expected cursor=1, got %d", m.cursor)
	}
	if m.shiftStart != -1 {
		t.Errorf("Expected shiftStart=-1, got %d", m.shiftStart)
	}
}

func TestHandleMouseEvent_ListView_WheelDown_AtBottom(t *testing.T) {
	m := &model{
		view:         listView,
		cursor:       2,
		containers:   []types.Container{{ID: "c1"}, {ID: "c2"}, {ID: "c3"}},
		containersMu: sync.RWMutex{},
	}

	msg := tea.MouseMsg{Type: tea.MouseWheelDown}
	newModel, _ := m.handleMouseEvent(msg)
	m = newModel.(*model)

	// Should stay at 2 (last index)
	if m.cursor != 2 {
		t.Errorf("Expected cursor=2, got %d", m.cursor)
	}
}

func TestHandleMouseEvent_NotListOrLogsView(t *testing.T) {
	m := &model{
		view: confirmView,
	}

	msg := tea.MouseMsg{Type: tea.MouseLeft}
	newModel, cmd := m.handleMouseEvent(msg)
	m = newModel.(*model)

	if cmd != nil {
		t.Error("Expected no command for non-list/logs view")
	}
}

func TestHandleMouseEvent_LogsView_LegacyBuffer(t *testing.T) {
	m := &model{
		view:            logsView,
		logsViewScroll:  0,
		height:          20,
		bufferConsumer:  nil, // No BufferConsumer
		logsViewBuffer:  []string{"line1", "line2", "line3"},
	}

	msg := tea.MouseMsg{Type: tea.MouseWheelDown}
	newModel, _ := m.handleMouseEvent(msg)
	m = newModel.(*model)

	// Should use logsViewBuffer length
	if m.logsViewScroll < 0 {
		t.Error("Expected scroll to be valid")
	}
}
