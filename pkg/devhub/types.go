package devhub

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/iota-uz/iota-sdk/pkg/devhub/services"
)

type ServiceInfo struct {
	Name         string
	Description  string
	Status       services.ServiceStatus
	Port         string
	LastUpdate   time.Time
	ErrorMsg     string
	PID          int
	StartTime    *time.Time
	CPUPercent   float64
	MemoryMB     float64
	HealthStatus services.HealthStatus
	DependsOn    []string
}

type StatusUpdateMsg struct {
	ServiceName string
	Status      services.ServiceStatus
	ErrorMsg    string
}

type BatchStatusUpdateMsg struct {
	Updates []StatusUpdateMsg
}

type TickMsg time.Time

func tickCmd() tea.Cmd {
	// Faster frequency for spinner animation
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

type ViewMode int

const (
	ServiceView ViewMode = iota
	LogView
)

// UIState manages UI-specific state
type UIState struct {
	Width         int
	Height        int
	SelectedIndex int
	ViewMode      ViewMode
	Spinner       spinner.Model
	SpinnerFrame  int
}

// LogViewState manages log view specific state
type LogViewState struct {
	ServiceIndex int // Index of the service whose logs are being viewed
	ScrollPos    int
	AutoScroll   bool     // Whether to auto-scroll to bottom of logs
	Lines        []string // Cached parsed log lines for efficiency
}

// SearchState manages search functionality
type SearchState struct {
	Active       bool
	Query        string
	Matches      []int // Indices of lines that match the search
	CurrentMatch int   // Current match index for navigation
}

// SystemStats holds system-wide resource usage
type SystemStats struct {
	CPUPercent    float64
	MemoryMB      float64
	MemoryPercent float64
}

type Model struct {
	// Core business logic
	Services       []ServiceInfo
	ServiceManager *ServiceManager
	StateManager   *StateManager

	// UI state
	UI UIState

	// View-specific states
	LogView LogViewState
	Search  SearchState

	// Performance optimizations
	LogCache *LogCache

	// System-wide stats
	SystemStats SystemStats
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		m.UI.Spinner.Tick,
	)
}
