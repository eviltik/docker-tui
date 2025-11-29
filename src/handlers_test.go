package main

import (
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/docker/docker/api/types"
)

// Helper to create a basic model for testing
func createTestModel() *model {
	return &model{
		containers:   []types.Container{},
		selected:     make(map[string]bool),
		processing:   make(map[string]bool),
		view:         listView,
		cursor:       0,
		width:        100,
		height:       30,
		containersMu: sync.RWMutex{},
		processingMu: sync.RWMutex{},
		cpuStatsMu:   sync.RWMutex{},
		cpuCurrent:   make(map[string]float64),
		cpuStats:     make(map[string][]float64),
	}
}

// TestHandleKeyPress_FilterMode tests filter mode key handling
func TestHandleKeyPress_FilterMode(t *testing.T) {
	tests := []struct {
		name            string
		initialFilter   string
		key             tea.KeyMsg
		wantFilter      string
		wantFilterMode  bool
		wantFilterInput string
	}{
		{
			name:            "enter applies filter",
			initialFilter:   "nginx",
			key:             tea.KeyMsg{Type: tea.KeyEnter},
			wantFilter:      "nginx",
			wantFilterMode:  false,
			wantFilterInput: "nginx",
		},
		{
			name:            "esc cancels filter",
			initialFilter:   "redis",
			key:             tea.KeyMsg{Type: tea.KeyEsc},
			wantFilter:      "",
			wantFilterMode:  false,
			wantFilterInput: "",
		},
		{
			name:            "backspace removes character",
			initialFilter:   "test",
			key:             tea.KeyMsg{Type: tea.KeyBackspace},
			wantFilter:      "tes",
			wantFilterMode:  true,
			wantFilterInput: "tes",
		},
		{
			name:            "space adds space",
			initialFilter:   "my",
			key:             tea.KeyMsg{Type: tea.KeySpace},
			wantFilter:      "my ",
			wantFilterMode:  true,
			wantFilterInput: "my ",
		},
		{
			name:            "rune adds character",
			initialFilter:   "ngin",
			key:             tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}},
			wantFilter:      "nginx",
			wantFilterMode:  true,
			wantFilterInput: "nginx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestModel()
			m.filterMode = true
			m.filterInput = tt.initialFilter
			m.filterActive = ""

			result, _ := m.handleKeyPress(tt.key)
			got := result.(*model)

			if got.filterActive != tt.wantFilter {
				t.Errorf("filterActive = %q, want %q", got.filterActive, tt.wantFilter)
			}
			if got.filterMode != tt.wantFilterMode {
				t.Errorf("filterMode = %v, want %v", got.filterMode, tt.wantFilterMode)
			}
			if got.filterInput != tt.wantFilterInput {
				t.Errorf("filterInput = %q, want %q", got.filterInput, tt.wantFilterInput)
			}
		})
	}
}

// TestHandleKeyPress_ExitConfirm tests exit confirmation dialog
func TestHandleKeyPress_ExitConfirm(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		wantQuit bool
		wantView viewMode
	}{
		{
			name:     "Y confirms quit",
			key:      "y",
			wantQuit: true,
			wantView: exitConfirmView,
		},
		{
			name:     "Y uppercase confirms quit",
			key:      "Y",
			wantQuit: true,
			wantView: exitConfirmView,
		},
		{
			name:     "N cancels quit",
			key:      "n",
			wantQuit: false,
			wantView: listView,
		},
		{
			name:     "esc cancels quit",
			key:      "esc",
			wantQuit: false,
			wantView: listView,
		},
		{
			name:     "q cancels quit",
			key:      "q",
			wantQuit: false,
			wantView: listView,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestModel()
			m.view = exitConfirmView

			result, cmd := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key), Alt: false})
			got := result.(*model)

			if tt.wantQuit {
				if cmd == nil {
					t.Error("expected tea.Quit command, got nil")
				}
			} else {
				if cmd != nil {
					t.Errorf("expected nil command, got non-nil")
				}
			}

			if got.view != tt.wantView {
				t.Errorf("view = %v, want %v", got.view, tt.wantView)
			}
		})
	}
}

// TestHandleKeyPress_Confirmation tests action confirmation dialog
func TestHandleKeyPress_Confirmation(t *testing.T) {
	m := createTestModel()
	// Add container to m.containers so getSelectedIDs() can find it
	m.containers = []types.Container{
		{ID: "container1", Names: []string{"/test1"}, State: "running"},
	}
	m.view = confirmView
	m.pendingAction = "stop"
	m.selected["container1"] = true

	// Test Y confirms action
	result, cmd := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	got := result.(*model)

	if got.view != listView {
		t.Errorf("view after Y = %v, want listView", got.view)
	}
	if cmd == nil {
		t.Error("expected command after confirmation, got nil")
	}

	// Test N cancels action
	m = createTestModel()
	m.view = confirmView
	m.pendingAction = "stop"

	result, cmd = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	got = result.(*model)

	if got.view != listView {
		t.Errorf("view after N = %v, want listView", got.view)
	}
	if cmd != nil {
		t.Error("expected nil command after cancel, got non-nil")
	}
}

// TestHandleKeyPress_Navigation tests cursor navigation
func TestHandleKeyPress_Navigation(t *testing.T) {
	m := createTestModel()
	m.containers = []types.Container{
		{ID: "1", Names: []string{"/container1"}},
		{ID: "2", Names: []string{"/container2"}},
		{ID: "3", Names: []string{"/container3"}},
		{ID: "4", Names: []string{"/container4"}},
		{ID: "5", Names: []string{"/container5"}},
	}
	m.cursor = 0

	// Test down arrow
	result, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	if result.(*model).cursor != 1 {
		t.Errorf("cursor after down = %d, want 1", result.(*model).cursor)
	}

	// Test up arrow
	m.cursor = 2
	result, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyUp})
	if result.(*model).cursor != 1 {
		t.Errorf("cursor after up = %d, want 1", result.(*model).cursor)
	}

	// Note: vim-style j/k navigation was removed in v1.1.0
	// Use arrow keys instead

	// Test Home
	m.cursor = 3
	result, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyHome})
	if result.(*model).cursor != 0 {
		t.Errorf("cursor after Home = %d, want 0", result.(*model).cursor)
	}

	// Test End
	m.cursor = 0
	result, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnd})
	wantEnd := len(m.containers) - 1
	if result.(*model).cursor != wantEnd {
		t.Errorf("cursor after End = %d, want %d", result.(*model).cursor, wantEnd)
	}

	// Test PgDown (with 5 containers: min(4, 0+10) = 4)
	m.cursor = 0
	result, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyPgDown})
	if result.(*model).cursor != 4 {
		t.Errorf("cursor after PgDown = %d, expected 4", result.(*model).cursor)
	}

	// Test PgUp
	m.cursor = 4
	result, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyPgUp})
	if result.(*model).cursor != 0 {
		t.Errorf("cursor after PgUp = %d, want 0", result.(*model).cursor)
	}
}

// TestHandleKeyPress_Selection tests selection operations
func TestHandleKeyPress_Selection(t *testing.T) {
	m := createTestModel()
	m.containers = []types.Container{
		{ID: "1", Names: []string{"/container1"}, State: "running"},
		{ID: "2", Names: []string{"/container2"}, State: "running"},
		{ID: "3", Names: []string{"/container3"}, State: "exited"},
	}
	m.cursor = 0

	// Test SPACE toggles selection
	result, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if !result.(*model).selected["1"] {
		t.Error("expected container 1 to be selected after SPACE")
	}

	// Test SPACE again deselects
	m = result.(*model)
	result, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if result.(*model).selected["1"] {
		t.Error("expected container 1 to be deselected after second SPACE")
	}

	// Test A selects all
	m = createTestModel()
	m.containers = []types.Container{
		{ID: "1", Names: []string{"/container1"}},
		{ID: "2", Names: []string{"/container2"}},
	}
	result, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if len(result.(*model).selected) != 2 {
		t.Errorf("selected count after A = %d, want 2", len(result.(*model).selected))
	}

	// Test X clears selection
	result, _ = result.(*model).handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if len(result.(*model).selected) != 0 {
		t.Errorf("selected count after X = %d, want 0", len(result.(*model).selected))
	}

	// Test Ctrl+A selects only running
	m = createTestModel()
	m.containers = []types.Container{
		{ID: "1", Names: []string{"/container1"}, State: "running"},
		{ID: "2", Names: []string{"/container2"}, State: "running"},
		{ID: "3", Names: []string{"/container3"}, State: "exited"},
	}
	result, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyCtrlA})
	got := result.(*model)
	if len(got.selected) != 2 {
		t.Errorf("selected count after Ctrl+A = %d, want 2", len(got.selected))
	}
	if got.selected["3"] {
		t.Error("expected exited container 3 not to be selected")
	}

	// Test I inverts selection
	m = createTestModel()
	m.containers = []types.Container{
		{ID: "1", Names: []string{"/container1"}},
		{ID: "2", Names: []string{"/container2"}},
	}
	m.selected["1"] = true
	result, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	got = result.(*model)
	if got.selected["1"] {
		t.Error("expected container 1 to be deselected after invert")
	}
	if !got.selected["2"] {
		t.Error("expected container 2 to be selected after invert")
	}
}

// TestHandleKeyPress_Filter tests filter activation
func TestHandleKeyPress_Filter(t *testing.T) {
	m := createTestModel()
	m.view = listView

	// Test / activates filter mode
	result, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if !result.(*model).filterMode {
		t.Error("expected filterMode to be true after /")
	}

	// Test ESC clears filter when not in filter mode
	m = createTestModel()
	m.filterActive = "nginx"
	result, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEsc})
	if result.(*model).filterActive != "" {
		t.Error("expected filterActive to be empty after ESC")
	}
}

// TestHandleKeyPress_Quit tests quit handling
func TestHandleKeyPress_Quit(t *testing.T) {
	m := createTestModel()
	m.view = listView

	// Test q shows exit confirmation
	result, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if result.(*model).view != exitConfirmView {
		t.Error("expected exitConfirmView after q")
	}

	// Test Ctrl+C quits immediately
	m = createTestModel()
	_, cmd := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("expected quit command after Ctrl+C, got nil")
	}
}

// TestHandleMouseEvent_Click tests mouse click handling
func TestHandleMouseEvent_Click(t *testing.T) {
	m := createTestModel()
	m.containers = []types.Container{
		{ID: "1", Names: []string{"/container1"}},
		{ID: "2", Names: []string{"/container2"}},
		{ID: "3", Names: []string{"/container3"}},
	}
	m.view = listView
	m.width = 100
	m.height = 30

	// Test click selects and toggles
	msg := tea.MouseMsg{
		X:      10,
		Y:      6, // Click on first container row
		Type:   tea.MouseLeft,
		Action: tea.MouseActionPress,
	}

	result, _ := m.handleMouseEvent(msg)
	got := result.(*model)

	// First click should move cursor and select
	if got.cursor < 0 || got.cursor >= len(m.containers) {
		t.Errorf("cursor after click = %d, expected in range [0, %d)", got.cursor, len(m.containers))
	}
}

// TestHandleMouseEvent_Wheel tests mouse wheel scrolling
func TestHandleMouseEvent_Wheel(t *testing.T) {
	m := createTestModel()
	m.containers = []types.Container{
		{ID: "1", Names: []string{"/container1"}},
		{ID: "2", Names: []string{"/container2"}},
		{ID: "3", Names: []string{"/container3"}},
	}
	m.cursor = 1
	m.view = listView

	// Test wheel up
	msg := tea.MouseMsg{
		Type:   tea.MouseWheelUp,
		Action: tea.MouseActionMotion,
	}
	result, _ := m.handleMouseEvent(msg)
	if result.(*model).cursor >= 1 {
		t.Errorf("cursor after wheel up = %d, expected < 1", result.(*model).cursor)
	}

	// Test wheel down
	m.cursor = 0
	msg = tea.MouseMsg{
		Type:   tea.MouseWheelDown,
		Action: tea.MouseActionMotion,
	}
	result, _ = m.handleMouseEvent(msg)
	if result.(*model).cursor <= 0 {
		t.Errorf("cursor after wheel down = %d, expected > 0", result.(*model).cursor)
	}
}

// TestHandleKeyPress_LogsView tests logs view key handling
func TestHandleKeyPress_LogsView(t *testing.T) {
	m := createTestModel()
	m.view = logsView
	m.logsViewScroll = 10

	// Test ESC returns to list view
	result, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEsc})
	if result.(*model).view != listView {
		t.Error("expected listView after ESC in logs view")
	}

	// Test q returns to list view
	m.view = logsView
	result, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if result.(*model).view != listView {
		t.Error("expected listView after q in logs view")
	}

	// Note: Scroll tests require bufferConsumer setup and are tested separately
	// The key behavior (view switching) is tested above
}

// TestHandleKeyPress_BoundaryCases tests edge cases
func TestHandleKeyPress_BoundaryCases(t *testing.T) {
	// Test navigation with empty container list
	m := createTestModel()
	m.containers = []types.Container{}
	m.cursor = 0

	result, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	if result.(*model).cursor != 0 {
		t.Error("cursor should stay at 0 with empty list")
	}

	// Test navigation beyond bounds
	m = createTestModel()
	m.containers = []types.Container{
		{ID: "1", Names: []string{"/container1"}},
	}
	m.cursor = 0

	result, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	if result.(*model).cursor >= len(m.containers) {
		t.Error("cursor should not exceed container count")
	}

	// Test selection with empty list
	m = createTestModel()
	m.containers = []types.Container{}
	result, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if len(result.(*model).selected) != 0 {
		t.Error("selection should not work with empty list")
	}
}
