package main

import (
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/docker/docker/api/types"
)

// Test remaining uncovered functions to boost coverage to 70%+

// Test getSelectedIDs
func TestGetSelectedIDs_NoSelection(t *testing.T) {
	m := &model{
		cursor: 0,
		containers: []types.Container{
			{ID: "c1"},
		},
		containersMu: sync.RWMutex{},
		selected:     make(map[string]bool),
		selectedMu:   sync.RWMutex{},
	}

	ids := m.getSelectedIDs()

	if len(ids) != 1 {
		t.Errorf("Expected 1 ID (cursor container), got %d", len(ids))
	}
	if ids[0] != "c1" {
		t.Errorf("Expected c1, got %s", ids[0])
	}
}

func TestGetSelectedIDs_WithSelection(t *testing.T) {
	m := &model{
		cursor: 0,
		containers: []types.Container{
			{ID: "c1"},
			{ID: "c2"},
			{ID: "c3"},
		},
		containersMu: sync.RWMutex{},
		selected: map[string]bool{
			"c2": true,
			"c3": true,
		},
		selectedMu: sync.RWMutex{},
	}

	ids := m.getSelectedIDs()

	if len(ids) != 2 {
		t.Errorf("Expected 2 IDs, got %d", len(ids))
	}
	// Should maintain order from containers list
	if ids[0] != "c2" || ids[1] != "c3" {
		t.Errorf("Expected [c2, c3], got %v", ids)
	}
}

func TestGetSelectedIDs_OutOfBoundsCursor(t *testing.T) {
	m := &model{
		cursor:       10, // Out of bounds
		containers:   []types.Container{{ID: "c1"}},
		containersMu: sync.RWMutex{},
		selected:     make(map[string]bool),
		selectedMu:   sync.RWMutex{},
	}

	ids := m.getSelectedIDs()

	// Should return empty when cursor out of bounds
	if len(ids) != 0 {
		t.Errorf("Expected 0 IDs for out of bounds cursor, got %d", len(ids))
	}
}

// Test performAction variations
func TestPerformAction_Start(t *testing.T) {
	m := &model{
		cursor: 0,
		containers: []types.Container{
			{ID: "c1", State: "exited"},
		},
		containersMu: sync.RWMutex{},
		selected: map[string]bool{
			"c1": true,
		},
		selectedMu: sync.RWMutex{},
	}

	cmd := m.performAction("start")

	if cmd == nil {
		t.Error("Expected command to be returned")
	}
}

func TestPerformAction_Stop(t *testing.T) {
	m := &model{
		cursor: 0,
		containers: []types.Container{
			{ID: "c1", State: "running"},
		},
		containersMu: sync.RWMutex{},
		selected: map[string]bool{
			"c1": true,
		},
		selectedMu: sync.RWMutex{},
	}

	cmd := m.performAction("stop")

	if cmd == nil {
		t.Error("Expected command to be returned")
	}
}

func TestPerformAction_Restart(t *testing.T) {
	m := &model{
		cursor: 0,
		containers: []types.Container{
			{ID: "c1", State: "running"},
		},
		containersMu: sync.RWMutex{},
		selected: map[string]bool{
			"c1": true,
		},
		selectedMu: sync.RWMutex{},
	}

	cmd := m.performAction("restart")

	if cmd == nil {
		t.Error("Expected command to be returned")
	}
}

func TestPerformAction_Remove(t *testing.T) {
	m := &model{
		cursor: 0,
		containers: []types.Container{
			{ID: "c1", State: "exited"},
		},
		containersMu: sync.RWMutex{},
		selected: map[string]bool{
			"c1": true,
		},
		selectedMu: sync.RWMutex{},
	}

	cmd := m.performAction("remove")

	if cmd == nil {
		t.Error("Expected command to be returned")
	}
}

func TestPerformAction_Pause(t *testing.T) {
	m := &model{
		cursor: 0,
		containers: []types.Container{
			{ID: "c1", State: "running"},
		},
		containersMu: sync.RWMutex{},
		selected: map[string]bool{
			"c1": true,
		},
		selectedMu: sync.RWMutex{},
	}

	cmd := m.performAction("pause")

	if cmd == nil {
		t.Error("Expected command to be returned")
	}
}

// Test router (handleKeyPress)
func TestHandleKeyPress_RouterFilter(t *testing.T) {
	m := &model{
		filterMode:  true,
		filterInput: "test",
	}

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, _ := m.handleKeyPress(msg)
	m = newModel.(*model)

	if m.filterMode {
		t.Error("Expected filterMode to be false after routing to filter handler")
	}
}

func TestHandleKeyPress_RouterExitConfirm(t *testing.T) {
	m := &model{
		view: exitConfirmView,
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
	_, cmd := m.handleKeyPress(msg)

	if cmd == nil || cmd() != tea.Quit() {
		t.Error("Expected quit command from exit confirm handler")
	}
}

func TestHandleKeyPress_RouterConfirm(t *testing.T) {
	m := &model{
		view:          confirmView,
		pendingAction: "start",
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
	newModel, _ := m.handleKeyPress(msg)
	m = newModel.(*model)

	if m.view != listView {
		t.Error("Expected routing to confirm handler")
	}
}

func TestHandleKeyPress_RouterLogs(t *testing.T) {
	m := &model{
		view:         logsView,
		filterActive: "test",
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	newModel, _ := m.handleKeyPress(msg)
	m = newModel.(*model)

	if m.filterActive != "" {
		t.Error("Expected routing to logs handler")
	}
}

func TestHandleKeyPress_RouterList(t *testing.T) {
	m := &model{
		view:   listView,
		cursor: 5,
	}

	msg := tea.KeyMsg{Type: tea.KeyHome}
	newModel, _ := m.handleKeyPress(msg)
	m = newModel.(*model)

	if m.cursor != 0 {
		t.Error("Expected routing to list handler")
	}
}

// TestGetContainerName already exists in model_test.go

// Test compileFilter
func TestCompileFilter_ValidRegex(t *testing.T) {
	m := &model{}

	m.compileFilter("^error.*")

	if m.filterRegex == nil {
		t.Error("Expected filterRegex to be compiled")
	}
	if !m.filterIsRegex {
		t.Error("Expected filterIsRegex to be true")
	}
}

func TestCompileFilter_InvalidRegex(t *testing.T) {
	m := &model{}

	m.compileFilter("[invalid")

	if m.filterRegex != nil {
		t.Error("Expected filterRegex to be nil for invalid regex")
	}
	if m.filterIsRegex {
		t.Error("Expected filterIsRegex to be false for invalid regex")
	}
}

func TestCompileFilter_EmptyString(t *testing.T) {
	m := &model{
		filterRegex:  nil,
		filterIsRegex: false,
	}

	m.compileFilter("")

	if m.filterRegex != nil {
		t.Error("Expected filterRegex to be nil for empty string")
	}
	if m.filterIsRegex {
		t.Error("Expected filterIsRegex to be false for empty string")
	}
}
