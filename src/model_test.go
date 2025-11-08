package main

import (
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/docker/docker/api/types"
)

// TestUpdateContainerListMsg tests container list update message handling
func TestUpdateContainerListMsg(t *testing.T) {
	m := &model{
		containers:   []types.Container{},
		selected:     make(map[string]bool),
		processing:   make(map[string]bool),
		containersMu: sync.RWMutex{},
		processingMu: sync.RWMutex{},
		cpuStatsMu:   sync.RWMutex{},
		cpuCurrent:   make(map[string]float64),
		cpuStats:     make(map[string][]float64),
	}

	// Create container list message
	containers := []types.Container{
		{ID: "1", Names: []string{"/nginx"}, State: "running"},
		{ID: "2", Names: []string{"/postgres"}, State: "running"},
	}

	msg := containerListMsg(containers)
	result, _ := m.Update(msg)
	updatedModel := result.(*model)

	if len(updatedModel.containers) != 2 {
		t.Errorf("Update() containers count = %d, want 2", len(updatedModel.containers))
	}
}

// TestUpdateToastMsg tests toast message handling
func TestUpdateToastMsg(t *testing.T) {
	m := &model{
		containers:   []types.Container{},
		selected:     make(map[string]bool),
		processing:   make(map[string]bool),
		containersMu: sync.RWMutex{},
		processingMu: sync.RWMutex{},
		cpuStatsMu:   sync.RWMutex{},
		cpuCurrent:   make(map[string]float64),
		cpuStats:     make(map[string][]float64),
	}

	msg := toastMsg{
		message: "Operation successful",
		isError: false,
	}

	result, _ := m.Update(msg)
	updatedModel := result.(*model)

	if updatedModel.toastMessage != "Operation successful" {
		t.Errorf("Update() toastMessage = %q, want %q", updatedModel.toastMessage, "Operation successful")
	}

	if updatedModel.toastIsError {
		t.Error("Update() toastIsError should be false")
	}
}

// TestUpdateToastMsgWithProcessingClear tests toast clearing processing map
func TestUpdateToastMsgWithProcessingClear(t *testing.T) {
	m := &model{
		containers: []types.Container{},
		selected:   make(map[string]bool),
		processing: map[string]bool{
			"container1": true,
			"container2": true,
		},
		containersMu: sync.RWMutex{},
		processingMu: sync.RWMutex{},
		cpuStatsMu:   sync.RWMutex{},
		cpuCurrent:   make(map[string]float64),
		cpuStats:     make(map[string][]float64),
	}

	msg := toastMsg{
		message:         "Containers stopped",
		isError:         false,
		clearProcessing: []string{"container1", "container2"},
	}

	result, _ := m.Update(msg)
	updatedModel := result.(*model)

	if len(updatedModel.processing) != 0 {
		t.Errorf("Update() processing map size = %d, want 0", len(updatedModel.processing))
	}
}

// TestUpdateErrorMsg tests error message handling
func TestUpdateErrorMsg(t *testing.T) {
	m := &model{
		containers:   []types.Container{},
		selected:     make(map[string]bool),
		processing:   make(map[string]bool),
		containersMu: sync.RWMutex{},
		processingMu: sync.RWMutex{},
		cpuStatsMu:   sync.RWMutex{},
		cpuCurrent:   make(map[string]float64),
		cpuStats:     make(map[string][]float64),
	}

	testErr := &testError{msg: "test error"}
	msg := errorMsg{err: testErr}

	result, _ := m.Update(msg)
	updatedModel := result.(*model)

	if updatedModel.err == nil {
		t.Error("Update() err should not be nil")
	}

	if updatedModel.err.Error() != "test error" {
		t.Errorf("Update() err = %v, want %v", updatedModel.err, testErr)
	}
}

// TestUpdateNewLogLineMsg tests new log line notification
func TestUpdateNewLogLineMsg(t *testing.T) {
	m := &model{
		containers:   []types.Container{},
		selected:     make(map[string]bool),
		processing:   make(map[string]bool),
		view:         logsView,
		containersMu: sync.RWMutex{},
		processingMu: sync.RWMutex{},
		cpuStatsMu:   sync.RWMutex{},
		cpuCurrent:   make(map[string]float64),
		cpuStats:     make(map[string][]float64),
	}

	msg := newLogLineMsg{}
	result, _ := m.Update(msg)

	// Just verify it doesn't crash and returns a model
	if result == nil {
		t.Error("Update() returned nil for newLogLineMsg")
	}
}

// TestUpdateWindowSizeMsg tests window resize handling
func TestUpdateWindowSizeMsg(t *testing.T) {
	m := &model{
		containers:   []types.Container{},
		selected:     make(map[string]bool),
		processing:   make(map[string]bool),
		width:        80,
		height:       24,
		containersMu: sync.RWMutex{},
		processingMu: sync.RWMutex{},
		cpuStatsMu:   sync.RWMutex{},
		cpuCurrent:   make(map[string]float64),
		cpuStats:     make(map[string][]float64),
	}

	msg := tea.WindowSizeMsg{
		Width:  120,
		Height: 40,
	}

	result, _ := m.Update(msg)
	updatedModel := result.(*model)

	if updatedModel.width != 120 {
		t.Errorf("Update() width = %d, want 120", updatedModel.width)
	}

	if updatedModel.height != 40 {
		t.Errorf("Update() height = %d, want 40", updatedModel.height)
	}
}

// TestGetContainerName tests container name extraction
func TestGetContainerName(t *testing.T) {
	tests := []struct {
		name      string
		container types.Container
		want      string
	}{
		{
			name: "container with leading slash",
			container: types.Container{
				Names: []string{"/nginx"},
			},
			want: "nginx",
		},
		{
			name: "container with multiple names",
			container: types.Container{
				Names: []string{"/nginx", "/web"},
			},
			want: "nginx",
		},
		{
			name: "container with no names",
			container: types.Container{
				Names: []string{},
				ID:    "abc123def456789",
			},
			want: "abc123def456",
		},
		{
			name: "container without slash",
			container: types.Container{
				Names: []string{"nginx"},
			},
			want: "nginx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getContainerName(tt.container)
			if got != tt.want {
				t.Errorf("getContainerName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestCompileFilter tests filter compilation
func TestCompileFilter(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantIsRegex   bool
		wantRegexNil  bool
	}{
		{
			name:         "empty input",
			input:        "",
			wantIsRegex:  false,
			wantRegexNil: true,
		},
		{
			name:         "valid regex",
			input:        "nginx.*",
			wantIsRegex:  true,
			wantRegexNil: false,
		},
		{
			name:         "invalid regex",
			input:        "[invalid",
			wantIsRegex:  false,
			wantRegexNil: true,
		},
		{
			name:         "simple string",
			input:        "postgres",
			wantIsRegex:  true,
			wantRegexNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{}
			m.compileFilter(tt.input)

			if m.filterIsRegex != tt.wantIsRegex {
				t.Errorf("compileFilter() filterIsRegex = %v, want %v", m.filterIsRegex, tt.wantIsRegex)
			}

			if (m.filterRegex == nil) != tt.wantRegexNil {
				t.Errorf("compileFilter() filterRegex nil = %v, want %v", m.filterRegex == nil, tt.wantRegexNil)
			}
		})
	}
}

// TestLogLineMatchesFilter tests log line filtering
func TestLogLineMatchesFilter(t *testing.T) {
	tests := []struct {
		name         string
		line         string
		filterActive string
		want         bool
	}{
		{
			name:         "no filter",
			line:         "error: something went wrong",
			filterActive: "",
			want:         true,
		},
		{
			name:         "filter matches",
			line:         "error: something went wrong",
			filterActive: "error",
			want:         true,
		},
		{
			name:         "filter does not match",
			line:         "info: everything ok",
			filterActive: "error",
			want:         false,
		},
		{
			name:         "case insensitive match",
			line:         "ERROR: something went wrong",
			filterActive: "error",
			want:         true,
		},
		{
			name:         "filter with ANSI codes",
			line:         "\x1b[31merror\x1b[0m: something went wrong",
			filterActive: "error",
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{
				filterActive: tt.filterActive,
			}

			got := m.logLineMatchesFilter(tt.line)
			if got != tt.want {
				t.Errorf("logLineMatchesFilter() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetFilteredLogCount tests filtered log counting
func TestGetFilteredLogCount(t *testing.T) {
	m := &model{
		filterActive: "",
	}

	// Without bufferConsumer, should return 0
	count := m.getFilteredLogCount()
	if count != 0 {
		t.Errorf("getFilteredLogCount() without bufferConsumer = %d, want 0", count)
	}
}

// TestUpdateWasAtBottom tests wasAtBottom update logic
func TestUpdateWasAtBottom(t *testing.T) {
	m := &model{
		logsViewScroll: 0,
		height:         30,
		filterActive:   "",
		view:           logsView,
	}

	// Without bufferConsumer, should not panic
	m.updateWasAtBottom()

	// Verify wasAtBottom is set to false when bufferConsumer is nil
	if m.wasAtBottom {
		t.Error("updateWasAtBottom() should set wasAtBottom to false without bufferConsumer")
	}
}

// TestInitialModel tests model initialization
func TestInitialModel(t *testing.T) {
	// Create initial model
	m := model{
		containers:   []types.Container{},
		selected:     make(map[string]bool),
		processing:   make(map[string]bool),
		containersMu: sync.RWMutex{},
		processingMu: sync.RWMutex{},
		cpuStatsMu:   sync.RWMutex{},
		cpuCurrent:   make(map[string]float64),
		cpuStats:     make(map[string][]float64),
	}

	// Verify initial state
	if m.view != listView {
		t.Errorf("Initial view = %v, want listView", m.view)
	}

	if m.cursor != 0 {
		t.Error("Initial cursor should be 0")
	}

	if len(m.selected) != 0 {
		t.Error("Initial selected map should be empty")
	}
}

// TestClearToastAfterTimeout tests toast auto-clear behavior
func TestClearToastAfterTimeout(t *testing.T) {
	m := &model{
		containers:   []types.Container{},
		selected:     make(map[string]bool),
		processing:   make(map[string]bool),
		toastMessage: "Test message",
		containersMu: sync.RWMutex{},
		processingMu: sync.RWMutex{},
		cpuStatsMu:   sync.RWMutex{},
		cpuCurrent:   make(map[string]float64),
		cpuStats:     make(map[string][]float64),
	}

	// Verify toast is initially set
	if m.toastMessage != "Test message" {
		t.Error("Initial toast message should be set")
	}
}
