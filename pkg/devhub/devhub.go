package devhub

import (
	"context"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// NewDevHub creates a new DevHub instance with all configured services
func NewDevHub(configPath string) (*Model, error) {
	loader := NewFileConfigLoader(configPath)
	serviceManager, err := NewServiceManager(loader)
	if err != nil {
		return nil, err
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Create state manager for async updates
	stateManager := NewStateManager(serviceManager)
	stateManager.Start()

	return &Model{
		Services:       stateManager.GetServices(),
		ServiceManager: serviceManager,
		StateManager:   stateManager,
		UI: UIState{
			SelectedIndex: 0,
			Spinner:       s,
			ViewMode:      ServiceView,
		},
		LogView: LogViewState{
			AutoScroll: true,
		},
		Search: SearchState{
			Active: false,
		},
		LogCache: NewLogCache(),
	}, nil
}

// Run starts the DevHub TUI interface
func (m *Model) Run(ctx context.Context) error {
	// Create cancellable context for the program
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle context cancellation
	go func() {
		<-runCtx.Done()
		m.StateManager.Stop()
		m.ServiceManager.Shutdown()
	}()

	p := tea.NewProgram(*m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return err
	}

	m.StateManager.Stop()
	m.ServiceManager.Shutdown()
	return nil
}
