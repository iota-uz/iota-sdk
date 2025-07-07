package devhub

import (
	"time"

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

type TickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

type Model struct {
	Services       []ServiceInfo
	SelectedIndex  int
	Width          int
	Height         int
	ServiceManager *ServiceManager
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		m.ServiceManager.StartMonitoring(),
	)
}
