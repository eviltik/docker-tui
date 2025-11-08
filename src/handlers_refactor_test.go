package main

import (
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/docker/docker/api/types"
)

// Test handlers_filter.go

func TestHandleFilterMode_EnterAppliesFilter(t *testing.T) {
	m := &model{
		filterMode:   true,
		filterInput:  "test",
		filterActive: "",
	}

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, _ := m.handleFilterMode(msg)
	m = newModel.(*model)

	if m.filterMode {
		t.Error("Expected filterMode to be false after Enter")
	}
	if m.filterActive != "test" {
		t.Errorf("Expected filterActive='test', got '%s'", m.filterActive)
	}
}

func TestHandleFilterMode_EscCancels(t *testing.T) {
	m := &model{
		filterMode:   true,
		filterInput:  "new",
		filterActive: "old",
	}

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	newModel, _ := m.handleFilterMode(msg)
	m = newModel.(*model)

	if m.filterMode {
		t.Error("Expected filterMode to be false after Esc")
	}
	if m.filterInput != "old" {
		t.Errorf("Expected filterInput restored to 'old', got '%s'", m.filterInput)
	}
}

func TestHandleFilterMode_Backspace(t *testing.T) {
	m := &model{
		filterMode:   true,
		filterInput:  "test",
		filterActive: "test",
		height:       20,
	}

	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	newModel, _ := m.handleFilterMode(msg)
	m = newModel.(*model)

	if m.filterInput != "tes" {
		t.Errorf("Expected filterInput='tes', got '%s'", m.filterInput)
	}
	if m.filterActive != "tes" {
		t.Errorf("Expected filterActive='tes', got '%s'", m.filterActive)
	}
}

func TestHandleFilterMode_Space(t *testing.T) {
	m := &model{
		filterMode:   true,
		filterInput:  "foo",
		filterActive: "foo",
		height:       20,
	}

	msg := tea.KeyMsg{Type: tea.KeySpace}
	newModel, _ := m.handleFilterMode(msg)
	m = newModel.(*model)

	if m.filterInput != "foo " {
		t.Errorf("Expected filterInput='foo ', got '%s'", m.filterInput)
	}
}

func TestHandleFilterMode_Runes(t *testing.T) {
	m := &model{
		filterMode:   true,
		filterInput:  "ab",
		filterActive: "ab",
		height:       20,
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
	newModel, _ := m.handleFilterMode(msg)
	m = newModel.(*model)

	if m.filterInput != "abc" {
		t.Errorf("Expected filterInput='abc', got '%s'", m.filterInput)
	}
}

// Test handlers_confirm.go

func TestHandleExitConfirmKeys_YQuits(t *testing.T) {
	m := &model{}
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}

	_, cmd := m.handleExitConfirmKeys(msg)

	if cmd == nil || cmd() != tea.Quit() {
		t.Error("Expected tea.Quit command")
	}
}

func TestHandleExitConfirmKeys_NCancels(t *testing.T) {
	m := &model{view: exitConfirmView}
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}

	newModel, _ := m.handleExitConfirmKeys(msg)
	m = newModel.(*model)

	if m.view != listView {
		t.Errorf("Expected view=listView, got %v", m.view)
	}
}

func TestHandleConfirmViewKeys_YExecutesAction(t *testing.T) {
	m := &model{
		pendingAction: "start",
		selected:      map[string]bool{"container1": true},
		containers: []types.Container{
			{ID: "container1", State: "exited"},
		},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}}
	newModel, cmd := m.handleConfirmViewKeys(msg)
	m = newModel.(*model)

	if m.view != listView {
		t.Errorf("Expected view=listView, got %v", m.view)
	}
	if m.pendingAction != "" {
		t.Error("Expected pendingAction to be cleared")
	}
	if cmd == nil {
		t.Error("Expected command to be returned")
	}
}

func TestHandleConfirmViewKeys_NCancelsAction(t *testing.T) {
	m := &model{
		view:          confirmView,
		pendingAction: "remove",
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}}
	newModel, _ := m.handleConfirmViewKeys(msg)
	m = newModel.(*model)

	if m.view != listView {
		t.Errorf("Expected view=listView, got %v", m.view)
	}
	if m.pendingAction != "" {
		t.Error("Expected pendingAction to be cleared")
	}
}

// Test handlers_logs.go

func TestHandleLogsViewKeys_QClearsFilter(t *testing.T) {
	m := &model{
		view:         logsView,
		filterActive: "error",
		filterInput:  "error",
		height:       20,
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	newModel, _ := m.handleLogsViewKeys(msg)
	m = newModel.(*model)

	if m.filterActive != "" {
		t.Error("Expected filter to be cleared")
	}
	if m.view != logsView {
		t.Error("Expected to stay in logs view when clearing filter")
	}
}

func TestHandleLogsViewKeys_QExitsView(t *testing.T) {
	m := &model{
		view:              logsView,
		filterActive:      "",
		viewTransitionMu:  sync.Mutex{},
		logChanCloseOnce:  sync.Once{},
		logChanWg:         sync.WaitGroup{},
		bufferConsumer:    nil,
		newLogChan:        nil,
		logsViewBuffer:    []string{},
		logsViewScroll:    0,
		logBroker:         &LogBroker{consumers: []LogConsumer{}},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	newModel, _ := m.handleLogsViewKeys(msg)
	m = newModel.(*model)

	if m.view != listView {
		t.Errorf("Expected view=listView, got %v", m.view)
	}
}

func TestHandleLogsViewKeys_UpScroll(t *testing.T) {
	m := &model{
		logsViewScroll: 5,
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	newModel, _ := m.handleLogsViewKeys(msg)
	m = newModel.(*model)

	if m.logsViewScroll != 4 {
		t.Errorf("Expected scroll=4, got %d", m.logsViewScroll)
	}
}

func TestHandleLogsViewKeys_DownScroll(t *testing.T) {
	m := &model{
		logsViewScroll: 0,
		height:         20,
		bufferConsumer: &BufferConsumer{
			buffer:   make([]LogEntry, 100),
			size:     20,
			maxLines: 100,
		},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	newModel, _ := m.handleLogsViewKeys(msg)
	m = newModel.(*model)

	if m.logsViewScroll != 1 {
		t.Errorf("Expected scroll=1, got %d", m.logsViewScroll)
	}
}

func TestHandleLogsViewKeys_Home(t *testing.T) {
	m := &model{
		logsViewScroll: 10,
	}

	msg := tea.KeyMsg{Type: tea.KeyHome}
	newModel, _ := m.handleLogsViewKeys(msg)
	m = newModel.(*model)

	if m.logsViewScroll != 0 {
		t.Errorf("Expected scroll=0, got %d", m.logsViewScroll)
	}
}

func TestHandleLogsViewKeys_End(t *testing.T) {
	m := &model{
		logsViewScroll: 0,
		height:         10,
		bufferConsumer: &BufferConsumer{
			buffer:   make([]LogEntry, 50),
			size:     50,
			maxLines: 50,
		},
	}

	msg := tea.KeyMsg{Type: tea.KeyEnd}
	newModel, _ := m.handleLogsViewKeys(msg)
	m = newModel.(*model)

	expected := max(0, 50-(10-5))
	if m.logsViewScroll != expected {
		t.Errorf("Expected scroll=%d, got %d", expected, m.logsViewScroll)
	}
}

func TestHandleLogsViewKeys_PgUp(t *testing.T) {
	m := &model{
		logsViewScroll: 15,
	}

	msg := tea.KeyMsg{Type: tea.KeyPgUp}
	newModel, _ := m.handleLogsViewKeys(msg)
	m = newModel.(*model)

	if m.logsViewScroll != 5 {
		t.Errorf("Expected scroll=5, got %d", m.logsViewScroll)
	}
}

func TestHandleLogsViewKeys_PgDown(t *testing.T) {
	m := &model{
		logsViewScroll: 0,
		height:         20,
		bufferConsumer: &BufferConsumer{
			buffer:   make([]LogEntry, 100),
			size:     100,
			maxLines: 100,
		},
	}

	msg := tea.KeyMsg{Type: tea.KeyPgDown}
	newModel, _ := m.handleLogsViewKeys(msg)
	m = newModel.(*model)

	if m.logsViewScroll != 10 {
		t.Errorf("Expected scroll=10, got %d", m.logsViewScroll)
	}
}

func TestHandleLogsViewKeys_SlashEntersFilter(t *testing.T) {
	m := &model{
		filterMode:   false,
		filterActive: "old",
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	newModel, _ := m.handleLogsViewKeys(msg)
	m = newModel.(*model)

	if !m.filterMode {
		t.Error("Expected filterMode to be true")
	}
	if m.filterInput != "old" {
		t.Errorf("Expected filterInput='old', got '%s'", m.filterInput)
	}
}

func TestHandleLogsViewKeys_CTogglesColor(t *testing.T) {
	m := &model{
		logsColorEnabled: true,
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
	newModel, _ := m.handleLogsViewKeys(msg)
	m = newModel.(*model)

	if m.logsColorEnabled {
		t.Error("Expected logsColorEnabled to be false")
	}

	// Toggle again
	newModel, _ = m.handleLogsViewKeys(msg)
	m = newModel.(*model)

	if !m.logsColorEnabled {
		t.Error("Expected logsColorEnabled to be true")
	}
}

// Test handlers_list.go

func TestHandleListViewKeys_UpNavigation(t *testing.T) {
	m := &model{
		cursor:     5,
		shiftStart: -1,
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	if m.cursor != 4 {
		t.Errorf("Expected cursor=4, got %d", m.cursor)
	}
}

func TestHandleListViewKeys_DownNavigation(t *testing.T) {
	m := &model{
		cursor:       2,
		shiftStart:   -1,
		containers:   make([]types.Container, 10),
		containersMu: sync.RWMutex{},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	if m.cursor != 3 {
		t.Errorf("Expected cursor=3, got %d", m.cursor)
	}
}

func TestHandleListViewKeys_Home(t *testing.T) {
	m := &model{
		cursor:     10,
		shiftStart: 5,
	}

	msg := tea.KeyMsg{Type: tea.KeyHome}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	if m.cursor != 0 {
		t.Errorf("Expected cursor=0, got %d", m.cursor)
	}
	if m.shiftStart != -1 {
		t.Error("Expected shiftStart to be reset")
	}
}

func TestHandleListViewKeys_End(t *testing.T) {
	m := &model{
		cursor:       0,
		shiftStart:   -1,
		containers:   make([]types.Container, 10),
		containersMu: sync.RWMutex{},
	}

	msg := tea.KeyMsg{Type: tea.KeyEnd}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	if m.cursor != 9 {
		t.Errorf("Expected cursor=9, got %d", m.cursor)
	}
}

func TestHandleListViewKeys_Space_TogglesSelection(t *testing.T) {
	m := &model{
		cursor: 0,
		containers: []types.Container{
			{ID: "container1", State: "running"},
		},
		containersMu: sync.RWMutex{},
		selected:     make(map[string]bool),
		selectedMu:   sync.RWMutex{},
	}

	msg := tea.KeyMsg{Type: tea.KeySpace}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	if !m.selected["container1"] {
		t.Error("Expected container1 to be selected")
	}

	// Toggle again
	newModel, _ = m.handleListViewKeys(msg)
	m = newModel.(*model)

	if m.selected["container1"] {
		t.Error("Expected container1 to be deselected")
	}
}

func TestHandleListViewKeys_X_ClearsSelection(t *testing.T) {
	m := &model{
		selected: map[string]bool{
			"c1": true,
			"c2": true,
		},
		selectedMu: sync.RWMutex{},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	if len(m.selected) != 0 {
		t.Errorf("Expected selection to be cleared, got %d items", len(m.selected))
	}
}

func TestHandleListViewKeys_A_SelectsAll(t *testing.T) {
	m := &model{
		containers: []types.Container{
			{ID: "c1", State: "running"},
			{ID: "c2", State: "exited"},
			{ID: "c3", State: "running"},
		},
		containersMu: sync.RWMutex{},
		selected:     make(map[string]bool),
		selectedMu:   sync.RWMutex{},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	if len(m.selected) != 3 {
		t.Errorf("Expected 3 containers selected, got %d", len(m.selected))
	}
}

func TestHandleListViewKeys_CtrlA_SelectsRunning(t *testing.T) {
	m := &model{
		containers: []types.Container{
			{ID: "c1", State: "running"},
			{ID: "c2", State: "exited"},
			{ID: "c3", State: "running"},
		},
		containersMu: sync.RWMutex{},
		selected:     make(map[string]bool),
		selectedMu:   sync.RWMutex{},
	}

	msg := tea.KeyMsg{Type: tea.KeyCtrlA}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	if len(m.selected) != 2 {
		t.Errorf("Expected 2 running containers selected, got %d", len(m.selected))
	}
	if !m.selected["c1"] || !m.selected["c3"] {
		t.Error("Expected c1 and c3 to be selected")
	}
}

func TestHandleListViewKeys_I_InvertsSelection(t *testing.T) {
	m := &model{
		containers: []types.Container{
			{ID: "c1", State: "running"},
			{ID: "c2", State: "exited"},
			{ID: "c3", State: "running"},
		},
		containersMu: sync.RWMutex{},
		selected: map[string]bool{
			"c1": true,
		},
		selectedMu: sync.RWMutex{},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'I'}}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	if m.selected["c1"] {
		t.Error("Expected c1 to be deselected")
	}
	if !m.selected["c2"] || !m.selected["c3"] {
		t.Error("Expected c2 and c3 to be selected")
	}
}

func TestHandleListViewKeys_CtrlC_Quits(t *testing.T) {
	m := &model{}

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.handleListViewKeys(msg)

	if cmd == nil || cmd() != tea.Quit() {
		t.Error("Expected tea.Quit command")
	}
}

func TestHandleListViewKeys_Q_ShowsConfirmation(t *testing.T) {
	m := &model{
		view:         listView,
		filterActive: "",
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	if m.view != exitConfirmView {
		t.Errorf("Expected view=exitConfirmView, got %v", m.view)
	}
}

func TestHandleListViewKeys_Q_ClearsFilter(t *testing.T) {
	m := &model{
		view:         listView,
		filterActive: "test",
		filterInput:  "test",
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Q'}}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	if m.filterActive != "" {
		t.Error("Expected filter to be cleared")
	}
	if m.view != listView {
		t.Error("Expected to stay in list view")
	}
}

func TestHandleListViewKeys_Slash_EntersFilter(t *testing.T) {
	m := &model{
		filterMode:   false,
		filterActive: "old",
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	if !m.filterMode {
		t.Error("Expected filterMode to be true")
	}
	if m.filterInput != "old" {
		t.Errorf("Expected filterInput='old', got '%s'", m.filterInput)
	}
}
