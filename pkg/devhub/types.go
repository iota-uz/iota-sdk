package devhub

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type ServiceStatus int

const (
	StatusStopped ServiceStatus = iota
	StatusStarting
	StatusRunning
	StatusStopping
	StatusError
)

func (s ServiceStatus) String() string {
	switch s {
	case StatusStopped:
		return "Stopped"
	case StatusStarting:
		return "Starting"
	case StatusRunning:
		return "Running"
	case StatusStopping:
		return "Stopping"
	case StatusError:
		return "Error"
	default:
		return "Unknown"
	}
}

type ServiceInfo struct {
	Name        string
	Description string
	Status      ServiceStatus
	Port        string
	LastUpdate  time.Time
	ErrorMsg    string
}

type StatusUpdateMsg struct {
	ServiceName string
	Status      ServiceStatus
	ErrorMsg    string
}

type BatchStatusUpdateMsg struct {
	Updates []StatusUpdateMsg
}

type TickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*80, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

type ViewMode int

const (
	ServiceView ViewMode = iota
	LogView
)

type Model struct {
	Services         []ServiceInfo
	SelectedIndex    int
	Width            int
	Height           int
	ServiceManager   *ServiceManager
	Spinner          spinner.Model
	SpinnerFrame     int
	ViewMode         ViewMode
	LogViewService   int // Index of the service whose logs are being viewed
	LogViewScrollPos int
	AutoScroll       bool     // Whether to auto-scroll to bottom of logs
	LogLines         []string // Cached parsed log lines for efficiency
	// Search-related fields
	SearchMode    bool
	SearchQuery   string
	SearchMatches []int // Indices of lines that match the search
	CurrentMatch  int   // Current match index for navigation
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		m.ServiceManager.StartMonitoring(),
		m.Spinner.Tick,
	)
}
