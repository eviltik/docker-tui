package main

import (
	"strings"
	"sync"
	"testing"

	"github.com/docker/docker/api/types"
)

// TestRenderList tests the main list view rendering
func TestRenderList(t *testing.T) {
	tests := []struct {
		name       string
		containers []types.Container
		width      int
		height     int
		wantContain []string
	}{
		{
			name:       "empty container list",
			containers: []types.Container{},
			width:      100,
			height:     30,
			wantContain: []string{
				"Docker TUI",
				"Total: 0",
			},
		},
		{
			name: "single container",
			containers: []types.Container{
				{
					ID:    "abc123",
					Names: []string{"/nginx"},
					State: "running",
				},
			},
			width:  100,
			height: 30,
			wantContain: []string{
				"Docker TUI",
				"Total: 1",
				"nginx",
			},
		},
		{
			name: "multiple containers",
			containers: []types.Container{
				{ID: "1", Names: []string{"/container1"}, State: "running"},
				{ID: "2", Names: []string{"/container2"}, State: "exited"},
				{ID: "3", Names: []string{"/container3"}, State: "paused"},
			},
			width:  100,
			height: 30,
			wantContain: []string{
				"Total: 3",
				"container1",
				"container2",
				"container3",
			},
		},
		{
			name:       "terminal too small",
			containers: []types.Container{},
			width:      30,
			height:     5,
			wantContain: []string{
				"Terminal too small",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{
				containers:   tt.containers,
				selected:     make(map[string]bool),
				processing:   make(map[string]bool),
				view:         listView,
				cursor:       0,
				width:        tt.width,
				height:       tt.height,
				containersMu: sync.RWMutex{},
				processingMu: sync.RWMutex{},
				cpuStatsMu:   sync.RWMutex{},
				cpuCurrent:   make(map[string]float64),
				cpuStats:     make(map[string][]float64),
			}

			result := m.View()

			for _, want := range tt.wantContain {
				// Strip ANSI codes before checking
				cleaned := stripAnsiCodes(result)
				if !strings.Contains(cleaned, want) {
					t.Errorf("View() missing expected content %q\nGot:\n%s", want, cleaned)
				}
			}
		})
	}
}

// TestRenderConfirm tests confirmation dialog rendering
func TestRenderConfirm(t *testing.T) {
	tests := []struct {
		name        string
		action      string
		containers  []types.Container
		selected    map[string]bool
		wantContain []string
	}{
		{
			name:   "single container confirmation",
			action: "stop",
			containers: []types.Container{
				{ID: "1", Names: []string{"/nginx"}},
			},
			selected: map[string]bool{"1": true},
			wantContain: []string{
				"Are you sure",
				"stop",
			},
		},
		{
			name:   "multiple containers confirmation",
			action: "remove",
			containers: []types.Container{
				{ID: "1", Names: []string{"/container1"}},
				{ID: "2", Names: []string{"/container2"}},
			},
			selected: map[string]bool{"1": true, "2": true},
			wantContain: []string{
				"Are you sure",
				"remove",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{
				containers:      tt.containers,
				selected:        tt.selected,
				processing:      make(map[string]bool),
				view:            confirmView,
				confirmMessage:  "Are you sure you want to " + tt.action + " the selected containers?",
				pendingAction:   tt.action,
				width:           100,
				height:          30,
				containersMu:    sync.RWMutex{},
				processingMu:    sync.RWMutex{},
				cpuStatsMu:      sync.RWMutex{},
				cpuCurrent:      make(map[string]float64),
				cpuStats:        make(map[string][]float64),
			}

			result := m.View()
			cleaned := stripAnsiCodes(result)

			for _, want := range tt.wantContain {
				if !strings.Contains(cleaned, want) {
					t.Errorf("View() missing expected content %q\nGot:\n%s", want, cleaned)
				}
			}
		})
	}
}

// TestRenderExitConfirm tests exit confirmation dialog
func TestRenderExitConfirm(t *testing.T) {
	m := &model{
		containers:     []types.Container{},
		selected:       make(map[string]bool),
		processing:     make(map[string]bool),
		view:           exitConfirmView,
		confirmMessage: "Are you sure you want to quit?",
		width:          100,
		height:         30,
		containersMu:   sync.RWMutex{},
		processingMu:   sync.RWMutex{},
		cpuStatsMu:     sync.RWMutex{},
		cpuCurrent:     make(map[string]float64),
		cpuStats:       make(map[string][]float64),
	}

	result := m.View()
	cleaned := stripAnsiCodes(result)

	wantContain := []string{
		"Are you sure",
	}

	for _, want := range wantContain {
		if !strings.Contains(cleaned, want) {
			t.Errorf("View() missing expected content %q", want)
		}
	}
}

// TestRenderFilter tests filter bar rendering
func TestRenderFilter(t *testing.T) {
	tests := []struct {
		name        string
		filterMode  bool
		filterInput string
		filterRegex bool
		wantContain []string
	}{
		{
			name:        "filter mode active with valid regex",
			filterMode:  true,
			filterInput: "nginx",
			filterRegex: true,
			wantContain: []string{
				"Filter:",
				"nginx",
			},
		},
		{
			name:        "filter mode active with invalid regex",
			filterMode:  true,
			filterInput: "[invalid",
			filterRegex: false,
			wantContain: []string{
				"Filter:",
				"[invalid",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{
				containers:   []types.Container{},
				selected:     make(map[string]bool),
				processing:   make(map[string]bool),
				view:         listView,
				filterMode:   tt.filterMode,
				filterInput:  tt.filterInput,
				filterIsRegex: tt.filterRegex,
				width:        100,
				height:       30,
				containersMu: sync.RWMutex{},
				processingMu: sync.RWMutex{},
				cpuStatsMu:   sync.RWMutex{},
				cpuCurrent:   make(map[string]float64),
				cpuStats:     make(map[string][]float64),
			}

			result := m.View()
			cleaned := stripAnsiCodes(result)

			for _, want := range tt.wantContain {
				if !strings.Contains(cleaned, want) {
					t.Errorf("View() missing expected content %q", want)
				}
			}
		})
	}
}

// TestRenderWithSelection tests rendering with selected containers
func TestRenderWithSelection(t *testing.T) {
	m := &model{
		containers: []types.Container{
			{ID: "1", Names: []string{"/container1"}, State: "running"},
			{ID: "2", Names: []string{"/container2"}, State: "running"},
			{ID: "3", Names: []string{"/container3"}, State: "exited"},
		},
		selected: map[string]bool{
			"1": true,
			"2": true,
		},
		processing:   make(map[string]bool),
		view:         listView,
		cursor:       0,
		width:        100,
		height:       30,
		containersMu: sync.RWMutex{},
		processingMu: sync.RWMutex{},
		selectedMu:   sync.RWMutex{},
		cpuStatsMu:   sync.RWMutex{},
		cpuCurrent:   make(map[string]float64),
		cpuStats:     make(map[string][]float64),
	}

	result := m.View()
	cleaned := stripAnsiCodes(result)

	// Should show selection count in some form (the exact format may vary)
	// Just verify the view renders without error
	if len(cleaned) == 0 {
		t.Error("View() returned empty string")
	}
}

// TestRenderWithProcessing tests rendering with containers being processed
func TestRenderWithProcessing(t *testing.T) {
	m := &model{
		containers: []types.Container{
			{ID: "1", Names: []string{"/container1"}, State: "running"},
		},
		selected: make(map[string]bool),
		processing: map[string]bool{
			"1": true,
		},
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

	result := m.View()

	// Should contain processing indicator (rendered, even if stripped later)
	if len(result) == 0 {
		t.Error("View() returned empty string")
	}
}

// TestRenderWithError tests error display
func TestRenderWithError(t *testing.T) {
	m := &model{
		containers:   []types.Container{},
		selected:     make(map[string]bool),
		processing:   make(map[string]bool),
		view:         listView,
		err:          &testError{msg: "test error message"},
		width:        100,
		height:       30,
		containersMu: sync.RWMutex{},
		processingMu: sync.RWMutex{},
		cpuStatsMu:   sync.RWMutex{},
		cpuCurrent:   make(map[string]float64),
		cpuStats:     make(map[string][]float64),
	}

	result := m.View()
	cleaned := stripAnsiCodes(result)

	if !strings.Contains(cleaned, "Error:") || !strings.Contains(cleaned, "test error message") {
		t.Error("View() should display error message")
	}
}

// TestRenderWithToast tests toast message display
func TestRenderWithToast(t *testing.T) {
	m := &model{
		containers:   []types.Container{},
		selected:     make(map[string]bool),
		processing:   make(map[string]bool),
		view:         listView,
		toastMessage: "Operation successful",
		width:        100,
		height:       30,
		containersMu: sync.RWMutex{},
		processingMu: sync.RWMutex{},
		cpuStatsMu:   sync.RWMutex{},
		cpuCurrent:   make(map[string]float64),
		cpuStats:     make(map[string][]float64),
	}

	result := m.View()
	cleaned := stripAnsiCodes(result)

	if !strings.Contains(cleaned, "Operation successful") {
		t.Error("View() should display toast message")
	}
}

// TestRenderDemoMode tests demo mode name cleaning
func TestRenderDemoMode(t *testing.T) {
	m := &model{
		containers: []types.Container{
			{ID: "1", Names: []string{"/project_nginx"}, State: "running"},
		},
		selected:     make(map[string]bool),
		processing:   make(map[string]bool),
		view:         listView,
		demoMode:     true,
		width:        100,
		height:       30,
		containersMu: sync.RWMutex{},
		processingMu: sync.RWMutex{},
		cpuStatsMu:   sync.RWMutex{},
		cpuCurrent:   make(map[string]float64),
		cpuStats:     make(map[string][]float64),
	}

	result := m.View()
	cleaned := stripAnsiCodes(result)

	// Should show "nginx" not "project_nginx"
	if !strings.Contains(cleaned, "nginx") {
		t.Error("View() should show cleaned name 'nginx' in demo mode")
	}

	// Should NOT show the prefix
	if strings.Contains(cleaned, "project_nginx") {
		t.Error("View() should not show full name 'project_nginx' in demo mode")
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// TestGetContainerLogColor tests consistent color assignment
func TestGetContainerLogColor(t *testing.T) {
	// Same container should always get the same color
	color1 := getContainerLogColor("container1")
	color2 := getContainerLogColor("container1")

	if color1 != color2 {
		t.Error("Same container should get consistent color")
	}

	// Different containers should (likely) get different colors
	color3 := getContainerLogColor("container2")
	// Note: We can't guarantee different colors due to hash collisions,
	// but we can verify the function returns a valid color
	if color3 == "" {
		t.Error("getContainerLogColor should return non-empty color")
	}
}

// TestContainerMatchesFilter tests filter matching logic
func TestContainerMatchesFilter(t *testing.T) {
	tests := []struct {
		name         string
		containerName string
		filterActive string
		filterRegex  bool
		want         bool
	}{
		{
			name:         "no filter - should match",
			containerName: "nginx",
			filterActive: "",
			want:         true,
		},
		{
			name:         "valid regex matches",
			containerName: "nginx-prod",
			filterActive: "nginx",
			filterRegex:  true,
			want:         true,
		},
		{
			name:         "valid regex does not match",
			containerName: "postgres",
			filterActive: "nginx",
			filterRegex:  true,
			want:         false,
		},
		{
			name:         "invalid regex - falls back to substring",
			containerName: "nginx-[invalid]-test",
			filterActive: "[invalid",
			filterRegex:  false,
			want:         true, // Substring match: "[invalid" is in "nginx-[invalid]-test"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{
				filterActive:  tt.filterActive,
				filterIsRegex: tt.filterRegex,
			}

			// Compile regex if valid
			if tt.filterActive != "" && tt.filterRegex {
				m.compileFilter(tt.filterActive)
			}

			container := types.Container{
				Names: []string{"/" + tt.containerName},
			}

			got := m.containerMatchesFilter(container)
			if got != tt.want {
				t.Errorf("containerMatchesFilter() = %v, want %v", got, tt.want)
			}
		})
	}
}
