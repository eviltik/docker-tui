package main

import (
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/docker/docker/api/types"
)

// Test Shift+Up/Down range selection

func TestHandleListViewKeys_ShiftUp_RangeSelection(t *testing.T) {
	m := &model{
		cursor:       5,
		shiftStart:   -1,
		containers: []types.Container{
			{ID: "c0"}, {ID: "c1"}, {ID: "c2"},
			{ID: "c3"}, {ID: "c4"}, {ID: "c5"},
			{ID: "c6"}, {ID: "c7"},
		},
		containersMu: sync.RWMutex{},
		selected:     make(map[string]bool),
		selectedMu:   sync.RWMutex{},
	}

	msg := tea.KeyMsg{Type: tea.KeyShiftUp}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	if m.cursor != 4 {
		t.Errorf("Expected cursor=4, got %d", m.cursor)
	}
	if m.shiftStart != 5 {
		t.Errorf("Expected shiftStart=5, got %d", m.shiftStart)
	}

	// Should have selected containers 4 and 5
	if !m.selected["c4"] || !m.selected["c5"] {
		t.Error("Expected c4 and c5 to be selected")
	}
}

func TestHandleListViewKeys_ShiftDown_RangeSelection(t *testing.T) {
	m := &model{
		cursor:       3,
		shiftStart:   -1,
		containers: []types.Container{
			{ID: "c0"}, {ID: "c1"}, {ID: "c2"},
			{ID: "c3"}, {ID: "c4"}, {ID: "c5"},
		},
		containersMu: sync.RWMutex{},
		selected:     make(map[string]bool),
		selectedMu:   sync.RWMutex{},
	}

	msg := tea.KeyMsg{Type: tea.KeyShiftDown}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	if m.cursor != 4 {
		t.Errorf("Expected cursor=4, got %d", m.cursor)
	}
	if m.shiftStart != 3 {
		t.Errorf("Expected shiftStart=3, got %d", m.shiftStart)
	}

	// Should have selected containers 3 and 4
	if !m.selected["c3"] || !m.selected["c4"] {
		t.Error("Expected c3 and c4 to be selected")
	}
}

func TestHandleListViewKeys_ShiftUp_ContinueRange(t *testing.T) {
	m := &model{
		cursor:       5,
		shiftStart:   5, // Already started shift selection
		containers: []types.Container{
			{ID: "c0"}, {ID: "c1"}, {ID: "c2"},
			{ID: "c3"}, {ID: "c4"}, {ID: "c5"},
			{ID: "c6"}, {ID: "c7"},
		},
		containersMu: sync.RWMutex{},
		selected:     make(map[string]bool),
		selectedMu:   sync.RWMutex{},
	}

	msg := tea.KeyMsg{Type: tea.KeyShiftUp}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	if m.cursor != 4 {
		t.Errorf("Expected cursor=4, got %d", m.cursor)
	}

	// Should have selected 4 and 5
	if len(m.selected) != 2 {
		t.Errorf("Expected 2 selected containers, got %d", len(m.selected))
	}

	// Move up again
	newModel, _ = m.handleListViewKeys(msg)
	m = newModel.(*model)

	if m.cursor != 3 {
		t.Errorf("Expected cursor=3, got %d", m.cursor)
	}

	// Should now have 3, 4, 5 selected
	if len(m.selected) != 3 {
		t.Errorf("Expected 3 selected containers, got %d", len(m.selected))
	}
	if !m.selected["c3"] || !m.selected["c4"] || !m.selected["c5"] {
		t.Error("Expected c3, c4, c5 to be selected")
	}
}

func TestHandleListViewKeys_PgUp(t *testing.T) {
	m := &model{
		cursor:     15,
		shiftStart: -1,
	}

	msg := tea.KeyMsg{Type: tea.KeyPgUp}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	if m.cursor != 5 {
		t.Errorf("Expected cursor=5, got %d", m.cursor)
	}
	if m.shiftStart != -1 {
		t.Error("Expected shiftStart to be reset")
	}
}

func TestHandleListViewKeys_PgDown(t *testing.T) {
	m := &model{
		cursor:       5,
		shiftStart:   -1,
		containers:   make([]types.Container, 20),
		containersMu: sync.RWMutex{},
	}

	msg := tea.KeyMsg{Type: tea.KeyPgDown}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	if m.cursor != 15 {
		t.Errorf("Expected cursor=15, got %d", m.cursor)
	}
	if m.shiftStart != -1 {
		t.Error("Expected shiftStart to be reset")
	}
}

func TestHandleListViewKeys_ShiftUp_BoundsCheck(t *testing.T) {
	m := &model{
		cursor:       0, // Already at top
		shiftStart:   -1,
		containers:   []types.Container{{ID: "c0"}, {ID: "c1"}},
		containersMu: sync.RWMutex{},
		selected:     make(map[string]bool),
		selectedMu:   sync.RWMutex{},
	}

	msg := tea.KeyMsg{Type: tea.KeyShiftUp}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	// Cursor should not move
	if m.cursor != 0 {
		t.Errorf("Expected cursor=0, got %d", m.cursor)
	}
}

func TestHandleListViewKeys_ShiftDown_BoundsCheck(t *testing.T) {
	m := &model{
		cursor:       1, // At bottom
		shiftStart:   -1,
		containers:   []types.Container{{ID: "c0"}, {ID: "c1"}},
		containersMu: sync.RWMutex{},
		selected:     make(map[string]bool),
		selectedMu:   sync.RWMutex{},
	}

	msg := tea.KeyMsg{Type: tea.KeyShiftDown}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	// Cursor should not move
	if m.cursor != 1 {
		t.Errorf("Expected cursor=1, got %d", m.cursor)
	}
}

// Test actions (s, p, r, u, d keys)

func TestHandleListViewKeys_S_SingleContainer(t *testing.T) {
	m := &model{
		cursor: 0,
		containers: []types.Container{
			{ID: "c1", State: "exited"},
		},
		containersMu: sync.RWMutex{},
		selected:     map[string]bool{"c1": true},
		selectedMu:   sync.RWMutex{},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
	_, cmd := m.handleListViewKeys(msg)

	if cmd == nil {
		t.Error("Expected command to be returned for single container start")
	}
}

func TestHandleListViewKeys_S_MultipleContainers(t *testing.T) {
	m := &model{
		containers: []types.Container{
			{ID: "c1", Names: []string{"/cont1"}, State: "exited"},
			{ID: "c2", Names: []string{"/cont2"}, State: "exited"},
		},
		containersMu: sync.RWMutex{},
		selected: map[string]bool{
			"c1": true,
			"c2": true,
		},
		selectedMu: sync.RWMutex{},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	if m.view != confirmView {
		t.Errorf("Expected view=confirmView for multi-container start, got %v", m.view)
	}
	if m.pendingAction != "start" {
		t.Errorf("Expected pendingAction=start, got %s", m.pendingAction)
	}
}

func TestHandleListViewKeys_K_StopContainers(t *testing.T) {
	m := &model{
		containers: []types.Container{
			{ID: "c1", Names: []string{"/cont1"}, State: "running"},
			{ID: "c2", Names: []string{"/cont2"}, State: "running"},
		},
		containersMu: sync.RWMutex{},
		selected: map[string]bool{
			"c1": true,
			"c2": true,
		},
		selectedMu: sync.RWMutex{},
	}

	// K is now the shortcut for Kill (stop) - changed in v1.1.0
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	if m.view != confirmView {
		t.Errorf("Expected view=confirmView for multi-container stop, got %v", m.view)
	}
	if m.pendingAction != "stop" {
		t.Errorf("Expected pendingAction=stop, got %s", m.pendingAction)
	}
}

func TestHandleListViewKeys_R_RestartContainers(t *testing.T) {
	m := &model{
		containers: []types.Container{
			{ID: "c1", Names: []string{"/cont1"}, State: "running"},
			{ID: "c2", Names: []string{"/cont2"}, State: "running"},
		},
		containersMu: sync.RWMutex{},
		selected: map[string]bool{
			"c1": true,
			"c2": true,
		},
		selectedMu: sync.RWMutex{},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	if m.view != confirmView {
		t.Errorf("Expected view=confirmView for multi-container restart, got %v", m.view)
	}
	if m.pendingAction != "restart" {
		t.Errorf("Expected pendingAction=restart, got %s", m.pendingAction)
	}
}

func TestHandleListViewKeys_P_PauseContainers(t *testing.T) {
	m := &model{
		containers: []types.Container{
			{ID: "c1", Names: []string{"/cont1"}, State: "running"},
			{ID: "c2", Names: []string{"/cont2"}, State: "running"},
		},
		containersMu: sync.RWMutex{},
		selected: map[string]bool{
			"c1": true,
			"c2": true,
		},
		selectedMu: sync.RWMutex{},
	}

	// P is now the shortcut for Pause - changed in v1.1.0
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	if m.view != confirmView {
		t.Errorf("Expected view=confirmView for multi-container pause, got %v", m.view)
	}
	if m.pendingAction != "pause" {
		t.Errorf("Expected pendingAction=pause, got %s", m.pendingAction)
	}
}

func TestHandleListViewKeys_D_RemoveContainers(t *testing.T) {
	m := &model{
		cursor: 0,
		containers: []types.Container{
			{ID: "c1", Names: []string{"/cont1"}, State: "exited"},
			{ID: "c2", Names: []string{"/cont2"}, State: "exited"},
		},
		containersMu: sync.RWMutex{},
		selected: map[string]bool{
			"c1": true,
			"c2": true,
		},
		selectedMu: sync.RWMutex{},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	if m.view != confirmView {
		t.Errorf("Expected view=confirmView for remove, got %v", m.view)
	}
	if m.pendingAction != "remove" {
		t.Errorf("Expected pendingAction=remove, got %s", m.pendingAction)
	}
}

func TestHandleListViewKeys_D_NoSelection_UseCursor(t *testing.T) {
	m := &model{
		cursor: 1,
		containers: []types.Container{
			{ID: "c1", Names: []string{"/cont1"}},
			{ID: "c2", Names: []string{"/cont2"}},
		},
		containersMu: sync.RWMutex{},
		selected:     make(map[string]bool), // No selection
		selectedMu:   sync.RWMutex{},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}
	newModel, _ := m.handleListViewKeys(msg)
	m = newModel.(*model)

	if m.view != confirmView {
		t.Errorf("Expected view=confirmView, got %v", m.view)
	}
	if m.pendingAction != "remove" {
		t.Errorf("Expected pendingAction=remove, got %s", m.pendingAction)
	}
}
