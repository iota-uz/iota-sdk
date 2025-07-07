package devhub

import (
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// NewDevHub creates a new DevHub instance with all configured services
func NewDevHub(configPath string) (*Model, error) {
	serviceManager, err := NewServiceManager(configPath)
	if err != nil {
		return nil, err
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &Model{
		Services:       serviceManager.GetServices(),
		SelectedIndex:  0,
		ServiceManager: serviceManager,
		Spinner:        s,
	}, nil
}

// Run starts the DevHub TUI interface
func (m *Model) Run() error {
	p := tea.NewProgram(*m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}
