package devhub

import (
	tea "github.com/charmbracelet/bubbletea"
)

// NewDevHub creates a new DevHub instance with all configured services
func NewDevHub() *Model {
	serviceManager := NewServiceManager()
	
	return &Model{
		Services:       serviceManager.GetServices(),
		SelectedIndex:  0,
		ServiceManager: serviceManager,
	}
}

// Run starts the DevHub TUI interface
func (m *Model) Run() error {
	p := tea.NewProgram(*m, tea.WithAltScreen())
	
	if _, err := p.Run(); err != nil {
		return err
	}
	
	return nil
}